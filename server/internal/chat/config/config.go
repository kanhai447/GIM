// Package config owns Chat WebSocket configuration validation.
package config

import (
	"errors"
	"net/url"
	"strings"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const (
	defaultSendBuffer    = 64
	defaultReadLimit     = int64(1 << 20)
	maximumReadLimit     = int64(16 << 20)
	defaultPongWait      = 60 * time.Second
	defaultPingPeriod    = 50 * time.Second
	defaultWriteWait     = 10 * time.Second
	maximumHeartbeatWait = 10 * time.Minute
)

type Config struct {
	Path           string
	SendBuffer     int
	AllowedOrigins []string
	ReadLimit      int64
	PongWait       time.Duration
	PingPeriod     time.Duration
	WriteWait      time.Duration
}

func FromValues(values platformconfig.Values) (Config, error) {
	path, err := values.Required("CHAT_WS_PATH")
	if err != nil || !validPath(path) {
		return Config{}, errors.New("configuration key CHAT_WS_PATH must be an absolute API path")
	}
	origins, err := values.CSV("CHAT_WS_ALLOWED_ORIGINS")
	if err != nil {
		return Config{}, err
	}
	buffer := defaultSendBuffer
	if raw, exists := values.Lookup("CHAT_WS_SEND_BUFFER"); exists && strings.TrimSpace(raw) != "" {
		buffer, err = values.Int("CHAT_WS_SEND_BUFFER")
		if err != nil || buffer < 1 || buffer > 4096 {
			return Config{}, errors.New("configuration key CHAT_WS_SEND_BUFFER must be between 1 and 4096")
		}
	}
	readLimit, err := optionalInt(values, "CHAT_WS_READ_LIMIT_BYTES", int(defaultReadLimit))
	if err != nil || readLimit < 1 || int64(readLimit) > maximumReadLimit {
		return Config{}, errors.New("configuration key CHAT_WS_READ_LIMIT_BYTES must be between 1 and 16777216")
	}
	pongWait, err := optionalDuration(values, "CHAT_WS_PONG_WAIT", defaultPongWait)
	if err != nil || pongWait <= 0 || pongWait > maximumHeartbeatWait {
		return Config{}, errors.New("configuration key CHAT_WS_PONG_WAIT must be a positive duration no greater than 10m")
	}
	pingPeriod, err := optionalDuration(values, "CHAT_WS_PING_PERIOD", defaultPingPeriod)
	if err != nil || pingPeriod <= 0 || pingPeriod >= pongWait {
		return Config{}, errors.New("configuration key CHAT_WS_PING_PERIOD must be positive and less than CHAT_WS_PONG_WAIT")
	}
	writeWait, err := optionalDuration(values, "CHAT_WS_WRITE_WAIT", defaultWriteWait)
	if err != nil || writeWait <= 0 || writeWait > maximumHeartbeatWait {
		return Config{}, errors.New("configuration key CHAT_WS_WRITE_WAIT must be a positive duration no greater than 10m")
	}
	return Config{
		Path: path, SendBuffer: buffer, AllowedOrigins: origins, ReadLimit: int64(readLimit),
		PongWait: pongWait, PingPeriod: pingPeriod, WriteWait: writeWait,
	}, nil
}

func optionalInt(values platformconfig.Values, key string, fallback int) (int, error) {
	if raw, exists := values.Lookup(key); !exists || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return values.Int(key)
}

func optionalDuration(values platformconfig.Values, key string, fallback time.Duration) (time.Duration, error) {
	if raw, exists := values.Lookup(key); !exists || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return values.Duration(key)
}

func validPath(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Path == value && strings.HasPrefix(value, "/api/chat/") && parsed.RawQuery == "" && parsed.Fragment == ""
}
