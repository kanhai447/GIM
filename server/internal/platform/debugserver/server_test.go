package debugserver

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestDebugServerDisabledAndLoopbackOnly(t *testing.T) {
	if server, err := Start(context.Background(), "", nil); err != nil || server != nil {
		t.Fatalf("disabled server=%v err=%v", server, err)
	}
	if _, err := Start(context.Background(), "0.0.0.0:6060", nil); err == nil {
		t.Fatal("expected non-loopback rejection")
	}
}

func TestDebugServerMetricsAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server, err := Start(ctx, "127.0.0.1:0", func() map[string]any { return map[string]any{"auditDropped": 3} })
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://" + server.Address() + "/debug/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil || payload["goroutines"] == nil || payload["auditDropped"].(float64) != 3 {
		t.Fatalf("payload=%v err=%v", payload, err)
	}
	cancel()
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := server.Close(shutdown); err != nil {
		t.Fatal(err)
	}
}
