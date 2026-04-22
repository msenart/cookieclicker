package game_test

import (
	"sync"
	"testing"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/game"
)

func defaultState(t *testing.T) *game.GameState {
	t.Helper()
	return game.NewGameState(config.Default())
}

// TestNewGameState verifies the initial game state matches expectations.
func TestNewGameState(t *testing.T) {
	cfg := config.Default()
	gs := game.NewGameState(cfg)

	if gs.Cookies() != 0 {
		t.Errorf("expected 0 cookies, got %f", gs.Cookies())
	}
	if gs.CPS() != 0 {
		t.Errorf("expected 0 CPS, got %f", gs.CPS())
	}

	snap := gs.Snapshot(0)
	if len(snap.Upgrades) != len(cfg.Upgrades) {
		t.Errorf("expected %d upgrades, got %d", len(cfg.Upgrades), len(snap.Upgrades))
	}
	for _, u := range snap.Upgrades {
		if u.Owned {
			t.Errorf("upgrade %q should start unowned", u.ID)
		}
	}
}

// TestAddCookies verifies sequential additions sum correctly.
func TestAddCookies(t *testing.T) {
	gs := defaultState(t)
	gs.AddCookies(10)
	gs.AddCookies(5.5)
	gs.AddCookies(-3)
	want := 12.5
	if got := gs.Cookies(); got != want {
		t.Errorf("expected %f cookies, got %f", want, got)
	}
}

// TestAddCookies_Race verifies concurrent AddCookies calls do not race.
// Run with: go test -race ./internal/game/
func TestAddCookies_Race(t *testing.T) {
	gs := defaultState(t)
	const goroutines = 50
	const adds = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < adds; j++ {
				gs.AddCookies(1)
			}
		}()
	}
	wg.Wait()
	want := float64(goroutines * adds)
	if got := gs.Cookies(); got != want {
		t.Errorf("expected %f cookies after concurrent adds, got %f", want, got)
	}
}

// TestApplyTick verifies one tick adds exactly CPS * interval seconds of cookies.
func TestApplyTick(t *testing.T) {
	cfg := config.Default()
	gs := game.NewGameState(cfg)
	// Give the player enough cookies to buy the first upgrade to set CPS > 0.
	gs.AddCookies(cfg.Upgrades[0].Cost)
	if err := gs.Purchase(cfg.Upgrades[0].ID); err != nil {
		t.Fatalf("purchase failed: %v", err)
	}
	cps := gs.CPS()
	if cps <= 0 {
		t.Fatal("CPS should be positive after purchase")
	}

	interval := 500 * time.Millisecond
	gs.ApplyTick(interval)

	wantIncrease := cps * interval.Seconds()
	// cookies after tick = (initial - cost) + tick income
	got := gs.Cookies()
	// We don't know exact starting cookies after purchase, so test the tick delta directly.
	// Re-test: record before and after.
	gs2 := game.NewGameState(cfg)
	gs2.AddCookies(cfg.Upgrades[0].Cost * 10)
	if err := gs2.Purchase(cfg.Upgrades[0].ID); err != nil {
		t.Fatalf("purchase failed: %v", err)
	}
	before := gs2.Cookies()
	gs2.ApplyTick(interval)
	after := gs2.Cookies()
	delta := after - before
	if delta != wantIncrease {
		t.Errorf("tick delta: want %f, got %f", wantIncrease, delta)
	}
	_ = got
}

// TestSnapshot_IsCopy verifies that modifying the returned snapshot's Upgrades slice
// does not affect the live game state.
func TestSnapshot_IsCopy(t *testing.T) {
	gs := defaultState(t)
	snap := gs.Snapshot(0)
	// Mutate the snapshot.
	if len(snap.Upgrades) > 0 {
		snap.Upgrades[0].Owned = true
		snap.Cookies = 999999
	}
	// Live state must be unchanged.
	snap2 := gs.Snapshot(0)
	if snap2.Cookies != 0 {
		t.Error("snapshot mutation affected live cookies")
	}
	for _, u := range snap2.Upgrades {
		if u.Owned {
			t.Errorf("snapshot mutation marked upgrade %q as owned in live state", u.ID)
		}
	}
}
