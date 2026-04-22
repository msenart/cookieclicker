package server

import (
	"sync"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
)

// RateLimiter enforces a sliding-window click rate limit per session.
// Excess events are counted so callers can detect anomalies.
type RateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time // sessionID → timestamps of allowed events
	excess  map[string]int         // sessionID → dropped-event count in current window
	cfg     *config.Config
	now     func() time.Time // injectable clock for tests
}

// NewRateLimiter constructs a RateLimiter using the limits defined in cfg.
func NewRateLimiter(cfg *config.Config) *RateLimiter {
	return NewRateLimiterWithClock(cfg, time.Now)
}

// NewRateLimiterWithClock constructs a RateLimiter with an injectable clock.
// Use this in tests to control time without sleeping.
func NewRateLimiterWithClock(cfg *config.Config, now func() time.Time) *RateLimiter {
	return &RateLimiter{
		windows: make(map[string][]time.Time),
		excess:  make(map[string]int),
		cfg:     cfg,
		now:     now,
	}
}

// SetNow replaces the clock function used by this RateLimiter.
// Intended for test use only.
func (rl *RateLimiter) SetNow(now func() time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.now = now
}

// Allow returns true if the event from sessionID is within the allowed rate,
// and records the event. Returns false and increments the excess counter when
// the window is full.
func (rl *RateLimiter) Allow(sessionID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	cutoff := now.Add(-rl.cfg.RateLimitWindow)

	// Prune events outside the current window.
	ts := rl.windows[sessionID]
	valid := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.cfg.MaxClicksPerSecond {
		rl.windows[sessionID] = valid
		rl.excess[sessionID]++
		return false
	}

	rl.windows[sessionID] = append(valid, now)
	rl.excess[sessionID] = 0
	return true
}

// ExcessCount returns the number of events dropped for sessionID since the last
// window reset. Used by the handler to detect anomalous clients.
func (rl *RateLimiter) ExcessCount(sessionID string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.excess[sessionID]
}
