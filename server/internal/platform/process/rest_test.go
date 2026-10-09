package process

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
)

func TestServeRESTStopsOnContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release local port: %v", err)
	}
	server, err := rest.NewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "process-rest-test-" + strconv.Itoa(port)},
		Host:        "127.0.0.1",
		Port:        port,
		Timeout:     1000,
	})
	if err != nil {
		t.Fatalf("create REST server: %v", err)
	}
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/health", Handler: func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeREST(ctx, server, time.Second) }()
	client := &http.Client{Timeout: 250 * time.Millisecond}

	deadline := time.Now().Add(3 * time.Second)
	for {
		response, requestErr := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusNoContent {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("REST server did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeREST() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ServeREST() did not stop after context cancellation")
	}
}
