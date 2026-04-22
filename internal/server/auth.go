// Package server implements the WebSocket server, session authentication,
// and per-session rate limiting for the cookie clicker game.
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/msenart/cookieclicker/internal/config"
)

// ErrInvalidToken is returned by ValidateToken when the token is missing,
// malformed, or its HMAC signature does not match.
var ErrInvalidToken = errors.New("invalid or tampered token")

// TokenService issues and validates HMAC-signed session tokens.
// The signing secret is read from an environment variable at construction time.
type TokenService struct {
	secret []byte
}

// NewTokenService constructs a TokenService by reading the signing secret from
// the environment variable named cfg.HMACKeyEnvVar. It returns an error if the
// variable is unset or its value is shorter than cfg.HMACMinKeyLen bytes.
func NewTokenService(cfg *config.Config) (*TokenService, error) {
	secret := os.Getenv(cfg.HMACKeyEnvVar)
	if len(secret) < cfg.HMACMinKeyLen {
		return nil, fmt.Errorf(
			"env var %s must be at least %d bytes (got %d)",
			cfg.HMACKeyEnvVar, cfg.HMACMinKeyLen, len(secret),
		)
	}
	return &TokenService{secret: []byte(secret)}, nil
}

// IssueToken returns a signed token for the given sessionID.
// Format: base64url( sessionID + "." + hex(HMAC-SHA256(sessionID)) )
func (ts *TokenService) IssueToken(sessionID string) string {
	sig := ts.sign(sessionID)
	raw := sessionID + "." + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ValidateToken parses and verifies a token produced by IssueToken.
// It returns the embedded sessionID on success, or ErrInvalidToken on any failure.
func (ts *TokenService) ValidateToken(token string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ErrInvalidToken
	}
	parts := strings.SplitN(string(decoded), ".", 2)
	if len(parts) != 2 {
		return "", ErrInvalidToken
	}
	sessionID, gotSig := parts[0], parts[1]
	if sessionID == "" {
		return "", ErrInvalidToken
	}
	wantSig := ts.sign(sessionID)
	if !hmac.Equal([]byte(gotSig), []byte(wantSig)) {
		return "", ErrInvalidToken
	}
	return sessionID, nil
}

// sign returns the hex-encoded HMAC-SHA256 of data using the service secret.
func (ts *TokenService) sign(data string) string {
	mac := hmac.New(sha256.New, ts.secret)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
