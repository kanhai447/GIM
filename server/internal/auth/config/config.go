// Package config validates Auth configuration loaded from GIM's safe config system.
package config

import (
	"errors"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const minimumSecretBytes = 32

type Config struct {
	JWTSecret []byte
	JWTExpiry time.Duration
}

func FromValues(values platformconfig.Values) (Config, error) {
	secret, err := values.Required("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	if len([]byte(secret)) < minimumSecretBytes {
		return Config{}, errors.New("configuration key JWT_SECRET must contain at least 32 bytes")
	}
	expirySeconds, err := values.Int("JWT_EXPIRE_SECONDS")
	if err != nil || expirySeconds < 1 {
		return Config{}, errors.New("configuration key JWT_EXPIRE_SECONDS must be a positive integer")
	}
	return Config{JWTSecret: []byte(secret), JWTExpiry: time.Duration(expirySeconds) * time.Second}, nil
}
