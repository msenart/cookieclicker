// Package game implements the core cookie clicker game logic: state management,
// passive income ticks, and upgrade purchases. All exported types are thread-safe.
package game

import (
	"sync"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
)

// OwnedUpgrade represents a single upgrade slot in a player's game state,
// combining the static catalog definition with runtime ownership status.
type OwnedUpgrade struct {
	// ID is the unique identifier matching config.UpgradeConfig.ID.
	ID string
	// Name is the human-readable display name.
	Name string
	// Owned is true when the player has purchased this upgrade.
	Owned bool
	// Cost is the cookie price to purchase this upgrade.
	Cost float64
	// CPSBonus is the cookies-per-second contribution when owned.
	CPSBonus float64
}

// UpgradeSnapshot is a wire-ready copy of an OwnedUpgrade, safe to serialise
// without holding the GameState lock.
type UpgradeSnapshot struct {
	// ID is the upgrade identifier.
	ID string `json:"id"`
	// Name is the display name.
	Name string `json:"name"`
	// Owned indicates whether the player has purchased this upgrade.
	Owned bool `json:"owned"`
	// Cost is the cookie price.
	Cost float64 `json:"cost"`
	// CPSBonus is the passive income contribution when owned.
	CPSBonus float64 `json:"cps_bonus"`
}

// StateSnapshot is a point-in-time copy of a GameState, safe to serialise
// without holding the GameState lock.
type StateSnapshot struct {
	// Cookies is the current cookie count.
	Cookies float64 `json:"cookies"`
	// CPS is the current cookies-per-second rate from all owned upgrades.
	CPS float64 `json:"cps"`
	// Upgrades is the full catalog with ownership flags.
	Upgrades []UpgradeSnapshot `json:"upgrades"`
	// Tick is the Unix millisecond timestamp when this snapshot was taken.
	Tick int64 `json:"tick"`
}

// GameState is the authoritative, thread-safe game state for one player session.
// All methods that read or modify cookies, CPS, or upgrade ownership acquire the
// internal mutex. Callers must never hold an external lock while calling these methods.
type GameState struct {
	mu       sync.Mutex
	cookies  float64
	cps      float64
	upgrades map[string]*OwnedUpgrade
}

// NewGameState constructs a GameState initialised from the upgrade catalog in cfg.
// All upgrades start unowned; cookies and CPS start at zero.
func NewGameState(cfg *config.Config) *GameState {
	upgrades := make(map[string]*OwnedUpgrade, len(cfg.Upgrades))
	for _, u := range cfg.Upgrades {
		upgrades[u.ID] = &OwnedUpgrade{
			ID:       u.ID,
			Name:     u.Name,
			Owned:    false,
			Cost:     u.Cost,
			CPSBonus: u.CPSBonus,
		}
	}
	return &GameState{upgrades: upgrades}
}

// AddCookies increments the cookie count by n. n may be negative (e.g. for purchases).
func (gs *GameState) AddCookies(n float64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.cookies += n
}

// Cookies returns the current cookie count.
func (gs *GameState) Cookies() float64 {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.cookies
}

// CPS returns the current cookies-per-second rate.
func (gs *GameState) CPS() float64 {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.cps
}

// ApplyTick adds passive income for one tick interval. It should be called by
// the Ticker goroutine at cfg.TickRate intervals.
func (gs *GameState) ApplyTick(interval time.Duration) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.cookies += gs.cps * interval.Seconds()
}

// Snapshot returns a point-in-time copy of the full game state. The returned
// value is independent of the GameState and safe to serialise without locking.
func (gs *GameState) Snapshot(tickMillis int64) StateSnapshot {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	upgrades := make([]UpgradeSnapshot, 0, len(gs.upgrades))
	for _, u := range gs.upgrades {
		upgrades = append(upgrades, UpgradeSnapshot{
			ID:       u.ID,
			Name:     u.Name,
			Owned:    u.Owned,
			Cost:     u.Cost,
			CPSBonus: u.CPSBonus,
		})
	}
	return StateSnapshot{
		Cookies:  gs.cookies,
		CPS:      gs.cps,
		Upgrades: upgrades,
		Tick:     tickMillis,
	}
}
