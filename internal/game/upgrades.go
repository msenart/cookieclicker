package game

import "errors"

// ErrInsufficientCookies is returned when a purchase is attempted but the player
// does not have enough cookies to cover the upgrade cost.
var ErrInsufficientCookies = errors.New("insufficient cookies")

// ErrUpgradeNotFound is returned when the requested upgrade ID does not exist
// in the game state's catalog.
var ErrUpgradeNotFound = errors.New("upgrade not found")

// ErrAlreadyOwned is returned when a purchase is attempted for an upgrade the
// player already owns.
var ErrAlreadyOwned = errors.New("upgrade already owned")

// Purchase attempts to buy the upgrade identified by upgradeID.
// On success it deducts the cost from cookies, marks the upgrade as owned,
// and adds its CPSBonus to the session CPS.
// Returns ErrUpgradeNotFound, ErrAlreadyOwned, or ErrInsufficientCookies on failure;
// the game state is left unchanged on any error.
func (gs *GameState) Purchase(upgradeID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	u, ok := gs.upgrades[upgradeID]
	if !ok {
		return ErrUpgradeNotFound
	}
	if u.Owned {
		return ErrAlreadyOwned
	}
	if gs.cookies < u.Cost {
		return ErrInsufficientCookies
	}

	gs.cookies -= u.Cost
	gs.cps += u.CPSBonus
	u.Owned = true
	return nil
}
