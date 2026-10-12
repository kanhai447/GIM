// Package config validates Group service-specific runtime values.
package config

import (
	"errors"
	"net/url"
	"strings"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type Config struct {
	DependencyTimeout time.Duration
	RedisTimeout      time.Duration
}

type WebSocketConfig struct {
	Path           string
	SendBuffer     int
	AllowedOrigins []string
	ReadLimit      int64
	PongWait       time.Duration
	PingPeriod     time.Duration
	WriteWait      time.Duration
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

func WebSocketFromValues(values platformconfig.Values) (WebSocketConfig, error) {
	path, err := values.Required("GROUP_WS_PATH")
	if err != nil || !validWebSocketPath(path) {
		return WebSocketConfig{}, errors.New("configuration key GROUP_WS_PATH must be an absolute /api/group path")
	}
	origins, err := values.CSV("GROUP_WS_ALLOWED_ORIGINS")
	if err != nil {
		return WebSocketConfig{}, err
	}
	sendBuffer, err := integer(values, "GROUP_WS_SEND_BUFFER", 64)
	if err != nil || sendBuffer < 1 || sendBuffer > 4096 {
		return WebSocketConfig{}, errors.New("configuration key GROUP_WS_SEND_BUFFER must be between 1 and 4096")
	}
	readLimit, err := integer(values, "GROUP_WS_READ_LIMIT_BYTES", 1<<20)
	if err != nil || readLimit < 1 || readLimit > 16<<20 {
		return WebSocketConfig{}, errors.New("configuration key GROUP_WS_READ_LIMIT_BYTES must be between 1 and 16777216")
	}
	pongWait, err := duration(values, "GROUP_WS_PONG_WAIT", 60*time.Second)
	if err != nil || pongWait > 10*time.Minute {
		return WebSocketConfig{}, errors.New("configuration key GROUP_WS_PONG_WAIT must be positive and no greater than 10m")
	}
	pingPeriod, err := duration(values, "GROUP_WS_PING_PERIOD", 50*time.Second)
	if err != nil || pingPeriod >= pongWait {
		return WebSocketConfig{}, errors.New("configuration key GROUP_WS_PING_PERIOD must be positive and less than GROUP_WS_PONG_WAIT")
	}
	writeWait, err := duration(values, "GROUP_WS_WRITE_WAIT", 10*time.Second)
	if err != nil || writeWait > 10*time.Minute {
		return WebSocketConfig{}, errors.New("configuration key GROUP_WS_WRITE_WAIT must be positive and no greater than 10m")
	}
	return WebSocketConfig{
		Path: path, SendBuffer: sendBuffer, AllowedOrigins: origins, ReadLimit: int64(readLimit),
		PongWait: pongWait, PingPeriod: pingPeriod, WriteWait: writeWait,
	}, nil
}

func integer(values platformconfig.Values, key string, fallback int) (int, error) {
	raw, exists := values.Lookup(key)
	if !exists || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return values.Int(key)
}

func validWebSocketPath(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Path == value && strings.HasPrefix(value, "/api/group/") && parsed.RawQuery == "" && parsed.Fragment == ""
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
