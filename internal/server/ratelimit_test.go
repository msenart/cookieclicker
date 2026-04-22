package server_test

import (
	"sync"
	"testing"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/server"
)

// TestRateLimiter_AllowsUpToLimit verifies that exactly MaxClicksPerSecond
// consecutive calls within one window are all allowed.
func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	cfg := config.Default()
	cfg.MaxClicksPerSecond = 5
	cfg.RateLimitWindow = time.Second
	rl := server.NewRateLimiter(cfg)

	for i := 0; i < cfg.MaxClicksPerSecond; i++ {
		if !rl.Allow("s1") {
			t.Errorf("call %d should be allowed", i+1)
		}
	}
}

// TestRateLimiter_BlocksOverLimit verifies that the (MaxClicksPerSecond+1)-th
// call within one window is rejected.
func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	cfg := config.Default()
	cfg.MaxClicksPerSecond = 5
	cfg.RateLimitWindow = time.Second
	rl := server.NewRateLimiter(cfg)

	for i := 0; i < cfg.MaxClicksPerSecond; i++ {
		rl.Allow("s1")
	}
	if rl.Allow("s1") {
		t.Error("call beyond limit should be blocked")
	}
	if rl.ExcessCount("s1") != 1 {
		t.Errorf("expected excess count 1, got %d", rl.ExcessCount("s1"))
	}
}

// TestRateLimiter_WindowSlides verifies that events slide out of the window
// and new ones are allowed after the window duration has elapsed.
func TestRateLimiter_WindowSlides(t *testing.T) {
	cfg := config.Default()
	cfg.MaxClicksPerSecond = 3
	cfg.RateLimitWindow = time.Second

	base := time.Now()
	current := base
	rl := server.NewRateLimiterWithClock(cfg, func() time.Time { return current })

	// Fill the window.
	for i := 0; i < cfg.MaxClicksPerSecond; i++ {
		if !rl.Allow("s1") {
			t.Fatalf("pre-slide call %d should be allowed", i+1)
		}
	}
	// Advance past the window.
	current = base.Add(cfg.RateLimitWindow + time.Millisecond)
	// Now the window is empty again.
	if !rl.Allow("s1") {
		t.Error("call after window slide should be allowed")
	}
}

// TestRateLimiter_PerSession verifies that two sessions have independent windows.
func TestRateLimiter_PerSession(t *testing.T) {
	cfg := config.Default()
	cfg.MaxClicksPerSecond = 2
	cfg.RateLimitWindow = time.Second
	rl := server.NewRateLimiter(cfg)

	// Fill session A.
	rl.Allow("A")
	rl.Allow("A")
	// Session B should still be allowed.
	if !rl.Allow("B") {
		t.Error("session B should not be affected by session A's limit")
	}
}

// TestRateLimiter_Race verifies concurrent Allow calls are race-free.
func TestRateLimiter_Race(t *testing.T) {
	cfg := config.Default()
	rl := server.NewRateLimiter(cfg)
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			rl.Allow("concurrent")
		}()
	}
	wg.Wait()
}
