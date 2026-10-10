package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFromValues(t *testing.T) {
	values := loadValues(t, "CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_SEND_BUFFER=32\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173,https://chat.example.test\nCHAT_WS_READ_LIMIT_BYTES=2097152\nCHAT_WS_PONG_WAIT=45s\nCHAT_WS_PING_PERIOD=30s\nCHAT_WS_WRITE_WAIT=5s\n")
	configuration, err := FromValues(values)
	if err != nil {
		t.Fatalf("FromValues() error = %v", err)
	}
	if configuration.Path != "/api/chat/ws/chat" || configuration.SendBuffer != 32 || len(configuration.AllowedOrigins) != 2 ||
		configuration.ReadLimit != 2<<20 || configuration.PongWait != 45*time.Second ||
		configuration.PingPeriod != 30*time.Second || configuration.WriteWait != 5*time.Second ||
		configuration.MaxTextBytes != 4096 || configuration.MaxPayloadBytes != 16*1024 ||
		configuration.DependencyTimeout != 2*time.Second || configuration.DeliveryTimeout != time.Second {
		t.Fatalf("FromValues() = %#v", configuration)
	}
}

func TestFromValuesUsesSafeHeartbeatDefaults(t *testing.T) {
	configuration, err := FromValues(loadValues(t, "CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\n"))
	if err != nil {
		t.Fatal(err)
	}
	if configuration.ReadLimit != 1<<20 || configuration.PongWait != 60*time.Second || configuration.PingPeriod != 50*time.Second || configuration.WriteWait != 10*time.Second {
		t.Fatalf("heartbeat defaults = %#v", configuration)
	}
}

func TestFromValuesRejectsUnsafeConfiguration(t *testing.T) {
	for _, content := range []string{
		"CHAT_WS_PATH=/other/ws\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=CHANGE_ME\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_SEND_BUFFER=0\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_WS_READ_LIMIT_BYTES=16777217\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_WS_PONG_WAIT=30s\nCHAT_WS_PING_PERIOD=30s\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_WS_WRITE_WAIT=0s\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_MESSAGE_MAX_TEXT_BYTES=0\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_MESSAGE_MAX_TEXT_BYTES=4096\nCHAT_MESSAGE_MAX_PAYLOAD_BYTES=100\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_MESSAGE_DEPENDENCY_TIMEOUT=0s\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\nCHAT_DELIVERY_OPERATION_TIMEOUT=31s\n",
	} {
		if _, err := FromValues(loadValues(t, content)); err == nil {
			t.Fatalf("FromValues() accepted unsafe configuration %q", content)
		}
	}
}

func loadValues(t *testing.T, content string) platformconfig.Values {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.test")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	return values
}
