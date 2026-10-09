package process

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
)

func TestRuntimeConfiguration(t *testing.T) {
	values := loadTestValues(t, `
INTERNAL_API_HOST=127.0.0.1
USER_API_PORT=8082
USER_API_INSTANCE_ID=user-api-test
USER_RPC_PORT=9091
ETCD_SERVICE_TTL=5s
SERVICE_STARTUP_TIMEOUT=3s
SERVICE_SHUTDOWN_TIMEOUT=4s
`)

	cfg, err := FromValues(values)
	if err != nil || cfg.StartupTimeout != 3*time.Second || cfg.ShutdownTimeout != 4*time.Second {
		t.Fatalf("FromValues() = %#v, %v", cfg, err)
	}
	address, err := RPCAddress(values, "USER")
	if err != nil || address != "127.0.0.1:9091" {
		t.Fatalf("RPCAddress() = %q, %v", address, err)
	}
	registration, err := discovery.RegistrationFromValues(values, "USER", "user_api")
	if err != nil {
		t.Fatalf("RegistrationFromValues() error = %v", err)
	}
	restConfig, err := RESTConfig(registration, "gim-user-api")
	if err != nil || restConfig.Host != "127.0.0.1" || restConfig.Port != 8082 {
		t.Fatalf("RESTConfig() = %#v, %v", restConfig, err)
	}
}

func TestLoadValuesRequiresEnvironmentFile(t *testing.T) {
	t.Setenv("GIM_ENV_FILE", "")
	if _, err := LoadValues(nil); err == nil {
		t.Fatal("LoadValues() unexpectedly accepted a missing environment file")
	}
}

func loadTestValues(t *testing.T, content string) platformconfig.Values {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test configuration: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("load test configuration: %v", err)
	}
	return values
}
