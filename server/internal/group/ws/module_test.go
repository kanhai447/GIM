package ws

import (
	"testing"
	"time"

	groupconfig "github.com/kanhai447/GIM/server/internal/group/config"
)

func TestModuleAssembly(t *testing.T) {
	module, err := New(groupconfig.WebSocketConfig{
		Path: "/api/group/ws/chat", SendBuffer: 16,
		AllowedOrigins: []string{testOrigin}, ReadLimit: 1 << 20,
		PongWait: time.Minute, PingPeriod: 50 * time.Second, WriteWait: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if module.Hub == nil || module.Handler == nil {
		t.Fatalf("incomplete module: %#v", module)
	}
}
