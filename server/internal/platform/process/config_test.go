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
INTERNAL_BIND_HOST=0.0.0.0
USER_API_PORT=8082
USER_API_HOST=user-api
USER_API_INSTANCE_ID=user-api-test
USER_RPC_PORT=9091
USER_RPC_HOST=user-rpc
ETCD_SERVICE_TTL=5s
SERVICE_STARTUP_TIMEOUT=3s
SERVICE_SHUTDOWN_TIMEOUT=4s
`)

	cfg, err := FromValues(values)
	if err != nil || cfg.StartupTimeout != 3*time.Second || cfg.ShutdownTimeout != 4*time.Second {
		t.Fatalf("FromValues() = %#v, %v", cfg, err)
	}
	address, err := RPCAddress(values, "USER")
	if err != nil || address != "user-rpc:9091" {
		t.Fatalf("RPCAddress() = %q, %v", address, err)
	}
	listenAddress, err := RPCListenAddress(values, "USER")
	if err != nil || listenAddress != "0.0.0.0:9091" {
		t.Fatalf("RPCListenAddress() = %q, %v", listenAddress, err)
	}
	registration, err := discovery.RegistrationFromValues(values, "USER", "user_api")
	if err != nil {
		t.Fatalf("RegistrationFromValues() error = %v", err)
	}
	restConfig, err := RESTConfig(values, registration, "gim-user-api")
	if err != nil || restConfig.Host != "0.0.0.0" || restConfig.Port != 8082 {
		t.Fatalf("RESTConfig() = %#v, %v", restConfig, err)
	}
}

func TestLoadValuesUsesProcessEnvironmentWithoutFile(t *testing.T) {
	t.Setenv("GIM_ENV_FILE", "")
	t.Setenv("SERVICE_STARTUP_TIMEOUT", "7s")
	values, err := LoadValues(nil)
	if err != nil {
		t.Fatalf("LoadValues() error = %v", err)
	}
	if value, ok := values.Lookup("SERVICE_STARTUP_TIMEOUT"); !ok || value != "7s" {
		t.Fatalf("SERVICE_STARTUP_TIMEOUT = %q, %v", value, ok)
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
