// Package debugserver exposes opt-in, loopback-only runtime diagnostics for
// development performance verification. It is disabled when the address is
// empty and is never routed through the public Gateway.
package debugserver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Extra func() map[string]any

type Server struct {
	listener net.Listener
	server   *http.Server
	done     chan struct{}
}

func Start(ctx context.Context, address string, extra Extra) (*Server, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return nil, errors.New("debug server address must use an explicit loopback IP and port")
	}
	if value, parseErr := strconv.Atoi(port); parseErr != nil || value < 0 || value > 65535 {
		return nil, errors.New("debug server address must use a valid port")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, errors.New("start debug listener failed")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/metrics", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		payload := map[string]any{
			"goroutines": runtime.NumGoroutine(), "heapAllocBytes": memory.HeapAlloc,
			"heapInuseBytes": memory.HeapInuse, "sysBytes": memory.Sys, "numGC": memory.NumGC,
		}
		if extra != nil {
			for key, value := range extra() {
				payload[key] = value
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(payload)
	})
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	server := &Server{listener: listener, server: &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second}, done: make(chan struct{})}
	go func() {
		defer close(server.done)
		_ = server.server.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.server.Shutdown(shutdown)
	}()
	return server, nil
}

func (server *Server) Address() string {
	if server == nil || server.listener == nil {
		return ""
	}
	return server.listener.Addr().String()
}

func (server *Server) Close(ctx context.Context) error {
	if server == nil || server.server == nil {
		return nil
	}
	err := server.server.Shutdown(ctx)
	select {
	case <-server.done:
	case <-ctx.Done():
		if err == nil {
			err = ctx.Err()
		}
	}
	return err
}
