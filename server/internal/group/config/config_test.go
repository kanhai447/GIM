package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestGroupWebSocketConfig(t *testing.T) {
	values := loadValues(t, "GROUP_WS_PATH=/api/group/ws/chat\nGROUP_WS_ALLOWED_ORIGINS=http://localhost:5173\nGROUP_WS_SEND_BUFFER=32\nGROUP_WS_READ_LIMIT_BYTES=2097152\nGROUP_WS_PONG_WAIT=45s\nGROUP_WS_PING_PERIOD=30s\nGROUP_WS_WRITE_WAIT=5s\n")
	configuration, err := WebSocketFromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Path != "/api/group/ws/chat" || configuration.SendBuffer != 32 || configuration.ReadLimit != 2<<20 ||
		configuration.PongWait != 45*time.Second || configuration.PingPeriod != 30*time.Second || configuration.WriteWait != 5*time.Second {
		t.Fatalf("unexpected websocket config: %#v", configuration)
	}
}

func TestGroupWebSocketConfigRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{
		"GROUP_WS_PATH=/api/chat/ws/chat\nGROUP_WS_ALLOWED_ORIGINS=http://localhost:5173\n",
		"GROUP_WS_PATH=/api/group/ws/chat\nGROUP_WS_ALLOWED_ORIGINS=CHANGE_ME\n",
		"GROUP_WS_PATH=/api/group/ws/chat\nGROUP_WS_ALLOWED_ORIGINS=http://localhost:5173\nGROUP_WS_SEND_BUFFER=0\n",
		"GROUP_WS_PATH=/api/group/ws/chat\nGROUP_WS_ALLOWED_ORIGINS=http://localhost:5173\nGROUP_WS_PONG_WAIT=30s\nGROUP_WS_PING_PERIOD=30s\n",
	} {
		if _, err := WebSocketFromValues(loadValues(t, input)); err == nil {
			t.Fatalf("expected invalid config for %q", input)
		}
	}
}

func loadValues(t *testing.T, content string) platformconfig.Values {
	t.Helper()
	path := filepath.Join(t.TempDir(), "group.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return values
}
