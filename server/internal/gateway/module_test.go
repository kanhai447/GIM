package gateway

import (
	"os"
	"path/filepath"
	"testing"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestNewRejectsMissingEtcdClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.test")
	if err := os.WriteFile(path, []byte("GATEWAY_HOST=127.0.0.1\nGATEWAY_PORT=8080\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if _, err := New(values, nil); err == nil {
		t.Fatal("New() error = nil for missing etcd client")
	}
}
