package config_test

import (
	"testing"
	"time"

	"github.com/msenart/cookieclicker/internal/config"
)

// TestDefault_AllFieldsPopulated verifies that Default() returns a Config with
// every field set to a non-zero value and at least 5 upgrade entries.
func TestDefault_AllFieldsPopulated(t *testing.T) {
	cfg := config.Default()

	if cfg.ListenAddr == "" {
		t.Error("ListenAddr is empty")
	}
	if cfg.TickRate <= 0 {
		t.Error("TickRate must be positive")
	}
	if cfg.BaseCookiesPerClick <= 0 {
		t.Error("BaseCookiesPerClick must be positive")
	}
	if cfg.MaxClicksPerSecond <= 0 {
		t.Error("MaxClicksPerSecond must be positive")
	}
	if cfg.RateLimitWindow <= 0 {
		t.Error("RateLimitWindow must be positive")
	}
	if cfg.AnomalyThreshold <= 0 {
		t.Error("AnomalyThreshold must be positive")
	}
	if cfg.SequenceWindowSize == 0 {
		t.Error("SequenceWindowSize must be non-zero")
	}
	if cfg.HMACKeyEnvVar == "" {
		t.Error("HMACKeyEnvVar is empty")
	}
	if cfg.HMACMinKeyLen <= 0 {
		t.Error("HMACMinKeyLen must be positive")
	}
	if len(cfg.Upgrades) < 5 {
		t.Errorf("expected at least 5 upgrades, got %d", len(cfg.Upgrades))
	}
}

// TestDefault_UpgradesHaveUniqueIDs verifies no two upgrades share the same ID.
func TestDefault_UpgradesHaveUniqueIDs(t *testing.T) {
	cfg := config.Default()
	seen := make(map[string]bool)
	for _, u := range cfg.Upgrades {
		if u.ID == "" {
			t.Errorf("upgrade %q has empty ID", u.Name)
		}
		if seen[u.ID] {
			t.Errorf("duplicate upgrade ID: %q", u.ID)
		}
		seen[u.ID] = true
	}
}

// TestDefault_UpgradeValuesPositive verifies all upgrade costs and CPS bonuses are positive.
func TestDefault_UpgradeValuesPositive(t *testing.T) {
	cfg := config.Default()
	for _, u := range cfg.Upgrades {
		if u.Cost <= 0 {
			t.Errorf("upgrade %q has non-positive cost %f", u.ID, u.Cost)
		}
		if u.CPSBonus <= 0 {
			t.Errorf("upgrade %q has non-positive CPSBonus %f", u.ID, u.CPSBonus)
		}
	}
}

// TestDefault_TickRateSanity verifies the tick rate is within a sensible range
// (between 10ms and 1s) so the game loop is neither too fast nor too slow.
func TestDefault_TickRateSanity(t *testing.T) {
	cfg := config.Default()
	if cfg.TickRate < 10*time.Millisecond || cfg.TickRate > time.Second {
		t.Errorf("TickRate %v is outside sane range [10ms, 1s]", cfg.TickRate)
	}
}
