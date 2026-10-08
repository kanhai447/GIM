package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	redisv9 "github.com/redis/go-redis/v9"
)

func TestNewBuildsAuthModuleFromSafeConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.test")
	content := "JWT_SECRET=" + strings.Repeat("x", 32) + "\nJWT_EXPIRE_SECONDS=3600\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	redisClient := redisv9.NewClient(&redisv9.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = redisClient.Close() })
	module, err := New(values, nil, redisClient)
	if err != nil || module == nil || module.Service == nil {
		t.Fatalf("New() = %#v, %v", module, err)
	}
}
