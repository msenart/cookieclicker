package game_test

import (
	"testing"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/game"
)

// TestTicker_TicksApplied verifies that running the ticker increases cookies
// after a number of ticks, using OnTick to observe ticks without sleeping.
func TestTicker_TicksApplied(t *testing.T) {
	cfg := config.Default()
	// Use a fast tick rate for tests.
	cfg.TickRate = 10 * time.Millisecond

	gs := game.NewGameState(cfg)
	// Buy first upgrade so CPS > 0.
	gs.AddCookies(cfg.Upgrades[0].Cost)
	if err := gs.Purchase(cfg.Upgrades[0].ID); err != nil {
		t.Fatalf("purchase: %v", err)
	}

	const wantTicks = 5
	ch := make(chan struct{}, wantTicks+1)
	tk := game.NewTicker(cfg, gs)
	tk.OnTick = func() { ch <- struct{}{} }
	tk.Start()
	defer tk.Stop()

	timeout := time.After(2 * time.Second)
	for i := 0; i < wantTicks; i++ {
		select {
		case <-ch:
		case <-timeout:
			t.Fatalf("timeout waiting for tick %d", i+1)
		}
	}
	if gs.Cookies() <= 0 {
		t.Error("expected cookies > 0 after ticks with positive CPS")
	}
}

// TestTicker_StopsCleanly verifies that no ticks arrive after Stop returns.
func TestTicker_StopsCleanly(t *testing.T) {
	cfg := config.Default()
	cfg.TickRate = 10 * time.Millisecond

	gs := game.NewGameState(cfg)
	ch := make(chan struct{}, 100)
	tk := game.NewTicker(cfg, gs)
	tk.OnTick = func() { ch <- struct{}{} }
	tk.Start()

	// Let at least one tick fire.
	timeout := time.After(500 * time.Millisecond)
	select {
	case <-ch:
	case <-timeout:
		t.Fatal("no tick received before stop")
	}

	tk.Stop()
	// Drain the channel.
	for len(ch) > 0 {
		<-ch
	}
	// No new ticks should arrive after Stop.
	select {
	case <-ch:
		t.Error("received a tick after Stop returned")
	case <-time.After(50 * time.Millisecond):
		// Expected: no ticks.
	}
}

// TestTicker_NoCookiesWithZeroCPS verifies passive income stays zero when CPS = 0.
func TestTicker_NoCookiesWithZeroCPS(t *testing.T) {
	cfg := config.Default()
	cfg.TickRate = 10 * time.Millisecond

	gs := game.NewGameState(cfg)
	// No upgrades purchased → CPS remains 0.

	const wantTicks = 5
	ch := make(chan struct{}, wantTicks+1)
	tk := game.NewTicker(cfg, gs)
	tk.OnTick = func() { ch <- struct{}{} }
	tk.Start()
	defer tk.Stop()

	timeout := time.After(time.Second)
	for i := 0; i < wantTicks; i++ {
		select {
		case <-ch:
		case <-timeout:
			t.Fatalf("timeout waiting for tick %d", i+1)
		}
	}
	if gs.Cookies() != 0 {
		t.Errorf("expected 0 cookies with zero CPS, got %f", gs.Cookies())
	}
}
