// Package token signs and verifies GIM Auth JWTs.
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var (
	ErrInvalid = errors.New("invalid token")
	ErrExpired = errors.New("expired token")
)

type Claims struct {
	UserID uint64 `json:"userID"`
	Role   int32  `json:"role"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewManager(secret []byte, ttl time.Duration) (*Manager, error) {
	return newManager(secret, ttl, time.Now)
}

func NewManagerWithClock(secret []byte, ttl time.Duration, now func() time.Time) (*Manager, error) {
	return newManager(secret, ttl, now)
}

func newManager(secret []byte, ttl time.Duration, now func() time.Time) (*Manager, error) {
	if len(secret) < 32 || ttl <= 0 || now == nil {
		return nil, errors.New("invalid jwt configuration")
	}
	return &Manager{secret: append([]byte(nil), secret...), ttl: ttl, now: now}, nil
}

func (manager *Manager) Issue(userID uint64, role int32) (string, Claims, error) {
	if userID == 0 || !validRole(role) {
		return "", Claims{}, ErrInvalid
	}
	now := manager.now().UTC()
	tokenID, err := randomID()
	if err != nil {
		return "", Claims{}, fmt.Errorf("generate token id: %w", err)
	}
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(manager.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
	if err != nil {
		return "", Claims{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, claims, nil
}

func (manager *Manager) Parse(raw string) (Claims, error) {
	if raw == "" {
		return Claims{}, ErrInvalid
	}
	claims := Claims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithoutClaimsValidation(),
	)
	parsed, err := parser.ParseWithClaims(raw, &claims, func(parsed *jwt.Token) (any, error) {
		if parsed.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalid
		}
		return manager.secret, nil
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return Claims{}, ErrInvalid
	}
	now := manager.now().UTC()
	if claims.ExpiresAt == nil || !now.Before(claims.ExpiresAt.Time) {
		return Claims{}, ErrExpired
	}
	if claims.IssuedAt == nil || claims.IssuedAt.Time.After(now) || claims.ID == "" || claims.UserID == 0 || !validRole(claims.Role) {
		return Claims{}, ErrInvalid
	}
	return claims, nil
}

func validRole(role int32) bool { return role == 1 || role == 2 }

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
