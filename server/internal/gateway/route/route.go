// Package route maps public API paths to explicit internal service names.
package route

import (
	"errors"
	"strings"
)

var (
	ErrInvalidPath        = errors.New("invalid api path")
	ErrUnsupportedService = errors.New("unsupported service")
)

type Target struct {
	PublicService    string
	DiscoveryService string
}

var services = map[string]string{
	"auth":     "auth_api",
	"user":     "user_api",
	"chat":     "chat_api",
	"group":    "group_api",
	"file":     "file_api",
	"settings": "settings_api",
	"logs":     "logs_api",
}

func Parse(path string) (Target, error) {
	if !strings.HasPrefix(path, "/api/") {
		return Target{}, ErrInvalidPath
	}
	remainder := strings.TrimPrefix(path, "/api/")
	separator := strings.IndexByte(remainder, '/')
	if separator < 1 {
		return Target{}, ErrInvalidPath
	}
	publicName := remainder[:separator]
	discoveryName, ok := services[publicName]
	if !ok {
		return Target{}, ErrUnsupportedService
	}
	return Target{PublicService: publicName, DiscoveryService: discoveryName}, nil
}
