package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/game"
)

// inboundMsg is the structure of every message sent by the client.
type inboundMsg struct {
	Type      string `json:"type"`
	Seq       uint64 `json:"seq"`
	UpgradeID string `json:"id,omitempty"`
}

// outboundMsg is the envelope for all server → client messages.
type outboundMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	// State fields — populated when Type == "state".
	Cookies  float64               `json:"cookies,omitempty"`
	CPS      float64               `json:"cps,omitempty"`
	Upgrades []game.UpgradeSnapshot `json:"upgrades,omitempty"`
	Tick     int64                 `json:"tick,omitempty"`
}

// upgrader configures gorilla/websocket. CheckOrigin allows all origins for
// development; production deployments should restrict this.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Session holds all per-connection state for one player.
type Session struct {
	id              string
	conn            *websocket.Conn
	state           *game.GameState
	ticker          *game.Ticker
	rateLimiter     *RateLimiter
	cfg             *config.Config
	lastAcceptedSeq uint64
	send            chan []byte
	done            chan struct{}
}

// Hub tracks all active player sessions.
type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	cfg      *config.Config
}

// NewHub constructs an empty Hub.
func NewHub(cfg *config.Config) *Hub {
	return &Hub{
		sessions: make(map[string]*Session),
		cfg:      cfg,
	}
}

// register adds a session to the hub.
func (h *Hub) register(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[s.id] = s
}

// unregister removes a session from the hub and stops its ticker.
func (h *Hub) unregister(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, s.id)
}

// ServeSession handles GET /session. It generates a new session ID, issues a
// signed token, and returns it as JSON: {"token":"..."}.
func ServeSession(ts *TokenService, w http.ResponseWriter, r *http.Request) {
	sessionID := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	token := ts.IssueToken(sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// ServeWS handles GET /ws. It validates the token query parameter, upgrades the
// connection to WebSocket, and starts the read/write goroutines for the session.
func ServeWS(hub *Hub, ts *TokenService, w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	sessionID, err := ts.ValidateToken(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	gs := game.NewGameState(hub.cfg)
	rl := NewRateLimiter(hub.cfg)
	sess := &Session{
		id:          sessionID,
		conn:        conn,
		state:       gs,
		rateLimiter: rl,
		cfg:         hub.cfg,
		send:        make(chan []byte, 64),
		done:        make(chan struct{}),
	}
	sess.ticker = game.NewTicker(hub.cfg, gs)
	sess.ticker.OnTick = func() {
		snap := gs.Snapshot(time.Now().UnixMilli())
		sess.pushState(snap)
	}

	hub.register(sess)
	sess.ticker.Start()

	// Send initial state immediately.
	snap := gs.Snapshot(time.Now().UnixMilli())
	sess.pushState(snap)

	go sess.writePump()
	go func() {
		sess.readPump()
		// Clean up when read pump exits.
		close(sess.done)
		sess.ticker.Stop()
		hub.unregister(sess)
		conn.Close()
		close(sess.send)
	}()
}

// readPump reads inbound messages from the WebSocket connection and dispatches them.
func (s *Session) readPump() {
	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg inboundMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			s.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "malformed json"))
			return
		}
		if err := s.dispatch(msg); err != nil {
			// dispatch signals a fatal protocol violation via a non-nil error.
			return
		}
	}
}

// dispatch processes a single validated inbound message.
// Returns a non-nil error only when the connection should be closed.
func (s *Session) dispatch(msg inboundMsg) error {
	// Sequence number validation.
	if msg.Seq <= s.lastAcceptedSeq {
		// Duplicate or replay — drop silently.
		return nil
	}
	if msg.Seq > s.lastAcceptedSeq+s.cfg.SequenceWindowSize {
		s.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "seq out of window"))
		return fmt.Errorf("seq %d too far ahead of %d", msg.Seq, s.lastAcceptedSeq)
	}
	s.lastAcceptedSeq = msg.Seq

	switch msg.Type {
	case "click":
		return s.handleClick()
	case "buy_upgrade":
		return s.handleBuyUpgrade(msg.UpgradeID)
	default:
		s.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown message type"))
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleClick processes a manual click event.
func (s *Session) handleClick() error {
	if !s.rateLimiter.Allow(s.id) {
		excess := s.rateLimiter.ExcessCount(s.id)
		if excess >= s.cfg.AnomalyThreshold {
			log.Printf("[ANOMALY] session %s: %d excess clicks in window", s.id, excess)
		}
		// Silently drop — no error sent to client.
		return nil
	}
	s.state.AddCookies(s.cfg.BaseCookiesPerClick)
	snap := s.state.Snapshot(time.Now().UnixMilli())
	s.pushState(snap)
	return nil
}

// handleBuyUpgrade processes an upgrade purchase request.
func (s *Session) handleBuyUpgrade(upgradeID string) error {
	err := s.state.Purchase(upgradeID)
	if err != nil {
		s.sendError(errorCode(err), err.Error())
		return nil
	}
	snap := s.state.Snapshot(time.Now().UnixMilli())
	s.pushState(snap)
	return nil
}

// pushState enqueues a state snapshot message on the send channel.
func (s *Session) pushState(snap game.StateSnapshot) {
	msg := outboundMsg{
		Type:     "state",
		Cookies:  snap.Cookies,
		CPS:      snap.CPS,
		Upgrades: snap.Upgrades,
		Tick:     snap.Tick,
	}
	s.enqueue(msg)
}

// sendError enqueues an error message on the send channel.
func (s *Session) sendError(code, message string) {
	s.enqueue(outboundMsg{Type: "error", Code: code, Message: message})
}

// enqueue serialises msg and places it on the send channel. Drops the message
// if the channel is full (backpressure: slow client).
func (s *Session) enqueue(msg outboundMsg) {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	select {
	case s.send <- b:
	default:
		log.Printf("send buffer full for session %s, dropping message", s.id)
	}
}

// writePump drains the send channel and writes each message to the WebSocket.
func (s *Session) writePump() {
	for {
		select {
		case msg, ok := <-s.send:
			if !ok {
				return
			}
			if err := s.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-s.done:
			return
		}
	}
}

// errorCode maps game purchase errors to wire error codes.
func errorCode(err error) string {
	switch {
	case errors.Is(err, game.ErrInsufficientCookies):
		return "insufficient_cookies"
	case errors.Is(err, game.ErrAlreadyOwned):
		return "already_owned"
	case errors.Is(err, game.ErrUpgradeNotFound):
		return "unknown_upgrade"
	default:
		return "error"
	}
}
