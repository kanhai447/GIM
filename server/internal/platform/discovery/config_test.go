package discovery

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestRegistrationFromValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.test")
	content := "INTERNAL_API_HOST=127.0.0.1\nAUTH_API_PORT=8081\nAUTH_API_INSTANCE_ID=auth-one\nETCD_SERVICE_TTL=20s\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	cfg, err := RegistrationFromValues(values, "AUTH", "auth_api")
	if err != nil {
		t.Fatalf("RegistrationFromValues() error = %v", err)
	}
	if cfg.Endpoint.String() != "http://127.0.0.1:8081" || cfg.InstanceID != "auth-one" || cfg.LeaseTTL != 20*time.Second {
		t.Fatalf("RegistrationFromValues() = %#v", cfg)
	}
}
