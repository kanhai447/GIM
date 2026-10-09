// Package config owns Chat WebSocket configuration validation.
package config

import (
	"errors"
	"net/url"
	"strings"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const defaultSendBuffer = 64

type Config struct {
	Path           string
	SendBuffer     int
	AllowedOrigins []string
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
	return Config{Path: path, SendBuffer: buffer, AllowedOrigins: origins}, nil
}

func validPath(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Path == value && strings.HasPrefix(value, "/api/chat/") && parsed.RawQuery == "" && parsed.Fragment == ""
}
