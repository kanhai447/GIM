// Package presence owns the Redis-backed global Chat Presence boundary.
package presence

import (
	"errors"
	"strings"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const (
	defaultTTL              = 90 * time.Second
	defaultRefreshInterval  = 30 * time.Second
	defaultRetryInterval    = 5 * time.Second
	defaultOperationTimeout = time.Second
	maximumTTL              = 24 * time.Hour
	maximumOperationTimeout = 30 * time.Second
)

type Config struct {
	InstanceID       string
	TTL              time.Duration
	RefreshInterval  time.Duration
	RetryInterval    time.Duration
	OperationTimeout time.Duration
}

func FromValues(values platformconfig.Values, instanceID string) (Config, error) {
	if strings.TrimSpace(instanceID) == "" {
		return Config{}, errors.New("chat presence instance ID is required")
	}
	ttl, err := optionalDuration(values, "CHAT_PRESENCE_TTL", defaultTTL)
	if err != nil || ttl <= 0 || ttl > maximumTTL {
		return Config{}, errors.New("configuration key CHAT_PRESENCE_TTL must be a positive duration no greater than 24h")
	}
	refresh, err := optionalDuration(values, "CHAT_PRESENCE_REFRESH_INTERVAL", defaultRefreshInterval)
	if err != nil || refresh <= 0 || refresh >= ttl {
		return Config{}, errors.New("configuration key CHAT_PRESENCE_REFRESH_INTERVAL must be positive and less than CHAT_PRESENCE_TTL")
	}
	retry, err := optionalDuration(values, "CHAT_PRESENCE_RETRY_INTERVAL", defaultRetryInterval)
	if err != nil || retry <= 0 || retry >= ttl {
		return Config{}, errors.New("configuration key CHAT_PRESENCE_RETRY_INTERVAL must be positive and less than CHAT_PRESENCE_TTL")
	}
	operationTimeout, err := optionalDuration(values, "CHAT_PRESENCE_OPERATION_TIMEOUT", defaultOperationTimeout)
	if err != nil || operationTimeout <= 0 || operationTimeout > maximumOperationTimeout {
		return Config{}, errors.New("configuration key CHAT_PRESENCE_OPERATION_TIMEOUT must be a positive duration no greater than 30s")
	}
	return Config{
		InstanceID: instanceID, TTL: ttl, RefreshInterval: refresh,
		RetryInterval: retry, OperationTimeout: operationTimeout,
	}, nil
}

func optionalDuration(values platformconfig.Values, key string, fallback time.Duration) (time.Duration, error) {
	if raw, exists := values.Lookup(key); !exists || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return values.Duration(key)
}
