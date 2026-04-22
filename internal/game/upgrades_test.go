package game_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/game"
)

// TestPurchase_Success verifies that a valid purchase deducts cost, sets owned,
// and increases CPS.
func TestPurchase_Success(t *testing.T) {
	cfg := config.Default()
	gs := game.NewGameState(cfg)
	u := cfg.Upgrades[0]
	gs.AddCookies(u.Cost)

	if err := gs.Purchase(u.ID); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if gs.Cookies() != 0 {
		t.Errorf("expected 0 cookies after exact purchase, got %f", gs.Cookies())
	}
	if gs.CPS() != u.CPSBonus {
		t.Errorf("expected CPS %f, got %f", u.CPSBonus, gs.CPS())
	}
	snap := gs.Snapshot(0)
	for _, su := range snap.Upgrades {
		if su.ID == u.ID && !su.Owned {
			t.Errorf("upgrade %q should be owned after purchase", u.ID)
		}
	}
}

// TestPurchase_InsufficientCookies verifies purchase is rejected when the player
// lacks funds and that game state is unchanged.
func TestPurchase_InsufficientCookies(t *testing.T) {
	cfg := config.Default()
	gs := game.NewGameState(cfg)
	u := cfg.Upgrades[0]
	gs.AddCookies(u.Cost - 1)

	err := gs.Purchase(u.ID)
	if !errors.Is(err, game.ErrInsufficientCookies) {
		t.Fatalf("expected ErrInsufficientCookies, got: %v", err)
	}
	if gs.CPS() != 0 {
		t.Error("CPS should remain 0 after failed purchase")
	}
	if gs.Cookies() != u.Cost-1 {
		t.Errorf("cookies should be unchanged after failed purchase, got %f", gs.Cookies())
	}
}

// TestPurchase_AlreadyOwned verifies that purchasing an owned upgrade returns
// ErrAlreadyOwned.
func TestPurchase_AlreadyOwned(t *testing.T) {
	cfg := config.Default()
	gs := game.NewGameState(cfg)
	u := cfg.Upgrades[0]
	gs.AddCookies(u.Cost * 2)

	if err := gs.Purchase(u.ID); err != nil {
		t.Fatalf("first purchase failed: %v", err)
	}
	err := gs.Purchase(u.ID)
	if !errors.Is(err, game.ErrAlreadyOwned) {
		t.Fatalf("expected ErrAlreadyOwned on second purchase, got: %v", err)
	}
}

// TestPurchase_UnknownID verifies that purchasing a nonexistent upgrade ID returns
// ErrUpgradeNotFound.
func TestPurchase_UnknownID(t *testing.T) {
	gs := game.NewGameState(config.Default())
	gs.AddCookies(999999)
	err := gs.Purchase("does_not_exist")
	if !errors.Is(err, game.ErrUpgradeNotFound) {
		t.Fatalf("expected ErrUpgradeNotFound, got: %v", err)
	}
}

// TestPurchase_Race verifies that concurrent purchase attempts for the same upgrade
// result in exactly one success and consistent state.
func TestPurchase_Race(t *testing.T) {
	cfg := config.Default()
	gs := game.NewGameState(cfg)
	u := cfg.Upgrades[0]
	gs.AddCookies(u.Cost * 100)

	const goroutines = 20
	results := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i] = gs.Purchase(u.ID)
		}()
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful purchase, got %d", successes)
	}
	// CPS should reflect exactly one upgrade.
	if gs.CPS() != u.CPSBonus {
		t.Errorf("expected CPS %f after one purchase, got %f", u.CPSBonus, gs.CPS())
	}
}
