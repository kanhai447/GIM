package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestIssueAndParseClaims(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	manager := mustManager(t, strings.Repeat("a", 32), time.Hour, func() time.Time { return now })
	raw, issued, err := manager.Issue(42, 2)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	parsed, err := manager.Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.UserID != 42 || parsed.Role != 2 || parsed.ID == "" || parsed.IssuedAt == nil || parsed.ExpiresAt == nil {
		t.Fatalf("claims = %#v", parsed)
	}
	if !parsed.ExpiresAt.Time.Equal(now.Add(time.Hour)) || issued.ID != parsed.ID {
		t.Fatalf("claim times/id mismatch: issued=%#v parsed=%#v", issued, parsed)
	}
}

func TestParseRejectsMalformedAndWrongSignature(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	manager := mustManager(t, strings.Repeat("a", 32), time.Hour, func() time.Time { return now })
	if _, err := manager.Parse("not-a-jwt"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed Parse() error = %v", err)
	}
	other := mustManager(t, strings.Repeat("b", 32), time.Hour, func() time.Time { return now })
	raw, _, err := other.Issue(42, 2)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, err := manager.Parse(raw); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong-signature Parse() error = %v", err)
	}
}

func TestParseRejectsExpired(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	clock := now
	manager := mustManager(t, strings.Repeat("a", 32), time.Minute, func() time.Time { return clock })
	raw, _, err := manager.Issue(42, 2)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	clock = now.Add(time.Minute)
	if _, err := manager.Parse(raw); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired Parse() error = %v", err)
	}
}

func TestParseRejectsUnexpectedAlgorithm(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	claims := Claims{UserID: 42, Role: 2, RegisteredClaims: jwt.RegisteredClaims{
		ID: "test-id", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(strings.Repeat("a", 32)))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	manager := mustManager(t, strings.Repeat("a", 32), time.Hour, func() time.Time { return now })
	if _, err := manager.Parse(raw); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unexpected-algorithm Parse() error = %v", err)
	}
}

func mustManager(t *testing.T, secret string, ttl time.Duration, now func() time.Time) *Manager {
	t.Helper()
	manager, err := NewManagerWithClock([]byte(secret), ttl, now)
	if err != nil {
		t.Fatalf("NewManagerWithClock() error = %v", err)
	}
	return manager
}
