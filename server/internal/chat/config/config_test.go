package config

import (
	"os"
	"path/filepath"
	"testing"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFromValues(t *testing.T) {
	values := loadValues(t, "CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_SEND_BUFFER=32\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173,https://chat.example.test\n")
	configuration, err := FromValues(values)
	if err != nil {
		t.Fatalf("FromValues() error = %v", err)
	}
	if configuration.Path != "/api/chat/ws/chat" || configuration.SendBuffer != 32 || len(configuration.AllowedOrigins) != 2 {
		t.Fatalf("FromValues() = %#v", configuration)
	}
}

func TestFromValuesRejectsUnsafeConfiguration(t *testing.T) {
	for _, content := range []string{
		"CHAT_WS_PATH=/other/ws\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_ALLOWED_ORIGINS=CHANGE_ME\n",
		"CHAT_WS_PATH=/api/chat/ws/chat\nCHAT_WS_SEND_BUFFER=0\nCHAT_WS_ALLOWED_ORIGINS=http://localhost:5173\n",
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
