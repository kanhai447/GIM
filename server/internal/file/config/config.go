// Package config validates File service runtime configuration.
package config

import (
	"errors"
	"strings"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const defaultMaxUploadBytes int64 = 50 << 20

type Config struct {
	StorageRoot    string
	MaxUploadBytes int64
}

func FromValues(values platformconfig.Values) (Config, error) {
	root, err := values.Required("FILE_STORAGE_ROOT")
	if err != nil || strings.TrimSpace(root) == "" {
		return Config{}, errors.New("configuration key FILE_STORAGE_ROOT is required")
	}
	max := defaultMaxUploadBytes
	if value, ok := values.Lookup("FILE_MAX_UPLOAD_BYTES"); ok && value != "" {
		parsed, parseErr := values.Int("FILE_MAX_UPLOAD_BYTES")
		if parseErr != nil || parsed < 1 || int64(parsed) > 1<<30 {
			return Config{}, errors.New("configuration key FILE_MAX_UPLOAD_BYTES must be between 1 and 1073741824")
		}
		max = int64(parsed)
	}
	return Config{StorageRoot: root, MaxUploadBytes: max}, nil
}
