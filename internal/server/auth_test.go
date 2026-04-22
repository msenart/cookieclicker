package server_test

import (
	"errors"
	"os"
	"testing"

	"github.com/msenart/cookieclicker/internal/config"
	"github.com/msenart/cookieclicker/internal/server"
)

const testSecret = "this-is-a-32-byte-test-secret!!"

func tokenServiceWithSecret(t *testing.T, secret string) *server.TokenService {
	t.Helper()
	t.Setenv("SESSION_SECRET", secret)
	cfg := config.Default()
	ts, err := server.NewTokenService(cfg)
	if err != nil {
		t.Fatalf("NewTokenService: %v", err)
	}
	return ts
}

// TestIssueAndValidate_RoundTrip verifies a token issued for a session ID
// validates back to the same session ID.
func TestIssueAndValidate_RoundTrip(t *testing.T) {
	ts := tokenServiceWithSecret(t, testSecret)
	sessionID := "session-abc-123"
	token := ts.IssueToken(sessionID)
	got, err := ts.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if got != sessionID {
		t.Errorf("expected session ID %q, got %q", sessionID, got)
	}
}

// TestValidate_TamperedSignature verifies that flipping bytes in the signature
// portion of the token causes validation to fail.
func TestValidate_TamperedSignature(t *testing.T) {
	ts := tokenServiceWithSecret(t, testSecret)
	token := ts.IssueToken("session-xyz")
	// Flip the last byte of the token string.
	b := []byte(token)
	b[len(b)-1] ^= 0xFF
	_, err := ts.ValidateToken(string(b))
	if !errors.Is(err, server.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got: %v", err)
	}
}

// TestValidate_TamperedSessionID verifies that mixing tokens from two different
// sessions produces an invalid token (HMAC covers the session ID).
func TestValidate_TamperedSessionID(t *testing.T) {
	ts := tokenServiceWithSecret(t, testSecret)
	tokenReal := ts.IssueToken("real-session")
	tokenEvil := ts.IssueToken("evil-session")
	if tokenReal == tokenEvil {
		t.Skip("tokens are identical (unexpected)")
	}
	// Combine halves from different tokens — signature will not match any session ID.
	mixed := tokenEvil[:len(tokenEvil)/2] + tokenReal[len(tokenReal)/2:]
	_, err := ts.ValidateToken(mixed)
	if !errors.Is(err, server.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for mixed token, got: %v", err)
	}
}

// TestValidate_EmptyToken verifies an empty token string returns ErrInvalidToken.
func TestValidate_EmptyToken(t *testing.T) {
	ts := tokenServiceWithSecret(t, testSecret)
	_, err := ts.ValidateToken("")
	if !errors.Is(err, server.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for empty token, got: %v", err)
	}
}

// TestNewTokenService_MissingEnv verifies that NewTokenService returns an error
// when the environment variable is not set.
func TestNewTokenService_MissingEnv(t *testing.T) {
	os.Unsetenv("SESSION_SECRET")
	cfg := config.Default()
	_, err := server.NewTokenService(cfg)
	if err == nil {
		t.Error("expected error when SESSION_SECRET is unset")
	}
}

// TestNewTokenService_ShortSecret verifies that NewTokenService returns an error
// when the secret is shorter than HMACMinKeyLen.
func TestNewTokenService_ShortSecret(t *testing.T) {
	t.Setenv("SESSION_SECRET", "short")
	cfg := config.Default()
	_, err := server.NewTokenService(cfg)
	if err == nil {
		t.Error("expected error when SESSION_SECRET is too short")
	}
}
