package game

import (
	"sync"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
)

// Ticker drives the server-side passive income loop for a single game session.
// It calls GameState.ApplyTick at cfg.TickRate intervals until Stop is called.
type Ticker struct {
	cfg   *config.Config
	state *GameState
	stop  chan struct{}
	wg    sync.WaitGroup
	// OnTick is an optional callback invoked after each successful tick.
	// It is nil in production; tests set it to observe ticks without sleeping.
	OnTick func()
}

// NewTicker constructs a Ticker that drives passive income for state.
// Call Start to begin ticking; call Stop to halt it cleanly.
func NewTicker(cfg *config.Config, state *GameState) *Ticker {
	return &Ticker{
		cfg:   cfg,
		state: state,
		stop:  make(chan struct{}),
	}
}

// Start launches the tick goroutine. Must be called at most once per Ticker.
func (t *Ticker) Start() {
	t.wg.Add(1)
	go t.run()
}

// Stop signals the tick goroutine to exit and blocks until it has done so.
func (t *Ticker) Stop() {
	close(t.stop)
	t.wg.Wait()
}

// run is the internal tick loop.
func (t *Ticker) run() {
	defer t.wg.Done()
	ticker := time.NewTicker(t.cfg.TickRate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.state.ApplyTick(t.cfg.TickRate)
			if t.OnTick != nil {
				t.OnTick()
			}
		case <-t.stop:
			return
		}
	}
}
