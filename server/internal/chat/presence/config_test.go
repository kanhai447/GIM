package presence

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFromValues(t *testing.T) {
	values := loadValues(t, "CHAT_PRESENCE_TTL=2m\nCHAT_PRESENCE_REFRESH_INTERVAL=40s\nCHAT_PRESENCE_RETRY_INTERVAL=3s\nCHAT_PRESENCE_OPERATION_TIMEOUT=500ms\n")
	configuration, err := FromValues(values, "chat-api-a")
	if err != nil {
		t.Fatal(err)
	}
	if configuration.InstanceID != "chat-api-a" || configuration.TTL != 2*time.Minute ||
		configuration.RefreshInterval != 40*time.Second || configuration.RetryInterval != 3*time.Second ||
		configuration.OperationTimeout != 500*time.Millisecond {
		t.Fatalf("FromValues() = %#v", configuration)
	}
}

func TestFromValuesDefaultsAndValidation(t *testing.T) {
	configuration, err := FromValues(loadValues(t, ""), "chat-api-local")
	if err != nil {
		t.Fatal(err)
	}
	if configuration.TTL != 90*time.Second || configuration.RefreshInterval != 30*time.Second ||
		configuration.RetryInterval != 5*time.Second || configuration.OperationTimeout != time.Second {
		t.Fatalf("defaults = %#v", configuration)
	}
	for _, content := range []string{
		"CHAT_PRESENCE_TTL=0s\n",
		"CHAT_PRESENCE_TTL=30s\nCHAT_PRESENCE_REFRESH_INTERVAL=30s\n",
		"CHAT_PRESENCE_TTL=30s\nCHAT_PRESENCE_RETRY_INTERVAL=30s\n",
		"CHAT_PRESENCE_OPERATION_TIMEOUT=0s\n",
	} {
		if _, err := FromValues(loadValues(t, content), "chat-api-local"); err == nil {
			t.Fatalf("accepted invalid configuration %q", content)
		}
	}
	if _, err := FromValues(loadValues(t, ""), ""); err == nil {
		t.Fatal("accepted empty instance ID")
	}
}

func loadValues(t *testing.T, content string) platformconfig.Values {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.test")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return values
}
