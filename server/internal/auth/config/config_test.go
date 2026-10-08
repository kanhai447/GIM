package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFromValues(t *testing.T) {
	values := loadValues(t, "JWT_SECRET="+strings.Repeat("x", 32)+"\nJWT_EXPIRE_SECONDS=3600\n")
	cfg, err := FromValues(values)
	if err != nil || cfg.JWTExpiry != time.Hour || len(cfg.JWTSecret) != 32 {
		t.Fatalf("FromValues() = %#v, %v", cfg, err)
	}
}

func TestFromValuesRejectsUnsafeSecretWithoutLeakingIt(t *testing.T) {
	secret := "short-private-value"
	values := loadValues(t, "JWT_SECRET="+secret+"\nJWT_EXPIRE_SECONDS=3600\n")
	_, err := FromValues(values)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("FromValues() error = %v", err)
	}
}

func loadValues(t *testing.T, content string) platformconfig.Values {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.test")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	return values
}
