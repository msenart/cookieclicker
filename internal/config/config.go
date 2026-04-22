// Package config defines all tunable parameters for the cookie clicker game server.
// All numeric, duration, and string constants that affect game behaviour or server
// policy live here. Logic packages receive a *Config at construction and must not
// declare their own magic numbers.
package config

import "time"

// UpgradeConfig holds the static definition of a single purchasable upgrade.
type UpgradeConfig struct {
	// ID is the unique machine-readable identifier used in wire messages.
	ID string
	// Name is the human-readable display name shown in the UI.
	Name string
	// Cost is the number of cookies required to purchase this upgrade.
	Cost float64
	// CPSBonus is the cookies-per-second added when this upgrade is owned.
	CPSBonus float64
}

// Config holds all tunable parameters for the game server.
// Modify Default() to change game balance or server policy without touching logic files.
type Config struct {
	// ListenAddr is the TCP address the HTTP server binds to (e.g. ":8080").
	ListenAddr string

	// TickRate is the duration between server-side passive-income ticks.
	TickRate time.Duration

	// BaseCookiesPerClick is the number of cookies awarded for one manual click.
	BaseCookiesPerClick float64

	// MaxClicksPerSecond is the sliding-window ceiling for click rate limiting.
	// Clicks beyond this threshold within RateLimitWindow are silently dropped.
	MaxClicksPerSecond int

	// RateLimitWindow is the duration of the sliding window used by the rate limiter.
	RateLimitWindow time.Duration

	// AnomalyThreshold is the number of excess (rate-limited) clicks within one
	// RateLimitWindow before the session is flagged in server logs.
	AnomalyThreshold int

	// SequenceWindowSize is the maximum gap between the last accepted sequence
	// number and an incoming one before the connection is closed with code 1008.
	SequenceWindowSize uint64

	// HMACKeyEnvVar is the name of the environment variable that holds the HMAC
	// signing secret for session tokens.
	HMACKeyEnvVar string

	// HMACMinKeyLen is the minimum acceptable byte length of the HMAC secret.
	// The server panics at startup if the secret is shorter than this value.
	HMACMinKeyLen int

	// Upgrades is the ordered catalog of purchasable upgrades presented to players.
	Upgrades []UpgradeConfig
}

// Default returns a Config populated with the standard game balance values.
// All fields are set; callers may override individual fields after calling Default().
func Default() *Config {
	return &Config{
		ListenAddr:          ":8080",
		TickRate:            100 * time.Millisecond,
		BaseCookiesPerClick: 1.0,
		MaxClicksPerSecond:  20,
		RateLimitWindow:     1 * time.Second,
		AnomalyThreshold:    5,
		SequenceWindowSize:  10,
		HMACKeyEnvVar:       "SESSION_SECRET",
		HMACMinKeyLen:       32,
		Upgrades: []UpgradeConfig{
			{ID: "cursor", Name: "Cursor", Cost: 15.0, CPSBonus: 0.1},
			{ID: "grandma", Name: "Grandma", Cost: 100.0, CPSBonus: 0.5},
			{ID: "farm", Name: "Farm", Cost: 500.0, CPSBonus: 2.0},
			{ID: "mine", Name: "Mine", Cost: 2000.0, CPSBonus: 8.0},
			{ID: "factory", Name: "Factory", Cost: 8000.0, CPSBonus: 20.0},
			{ID: "bank", Name: "Bank", Cost: 30000.0, CPSBonus: 50.0},
			{ID: "temple", Name: "Temple", Cost: 100000.0, CPSBonus: 130.0},
		},
	}
}
