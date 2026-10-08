package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFromValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.test")
	content := "GATEWAY_HOST=127.0.0.1\nGATEWAY_PORT=8080\nGATEWAY_AUTH_TIMEOUT=4s\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	cfg, err := FromValues(values)
	if err != nil {
		t.Fatalf("FromValues() error = %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:8080" || cfg.AuthTimeout != 4*time.Second || cfg.DiscoveryTimeout != 2*time.Second || cfg.ProxyTimeout != 15*time.Second {
		t.Fatalf("FromValues() = %#v", cfg)
	}
}
