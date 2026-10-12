// Package config validates Group service-specific runtime values.
package config

import (
	"errors"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type Config struct {
	DependencyTimeout time.Duration
	RedisTimeout      time.Duration
}

func FromValues(values platformconfig.Values) (Config, error) {
	dependency, err := duration(values, "GROUP_DEPENDENCY_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}
	redisTimeout, err := duration(values, "GROUP_REDIS_OPERATION_TIMEOUT", time.Second)
	if err != nil {
		return Config{}, err
	}
	return Config{DependencyTimeout: dependency, RedisTimeout: redisTimeout}, nil
}

func duration(values platformconfig.Values, key string, fallback time.Duration) (time.Duration, error) {
	value, exists := values.Lookup(key)
	if !exists || value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("configuration key " + key + " must be a positive duration")
	}
	return parsed, nil
}
