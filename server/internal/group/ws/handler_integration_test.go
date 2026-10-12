package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
)

const testOrigin = "http://localhost:5173"

func TestGroupEndpointTypedErrorsAndMultiDevice(t *testing.T) {
	hub, _ := runningHub(t)
	server := groupWebSocketServer(t, hub, testHeartbeat(), nil)
	first := dialGroup(t, server.URL, testOrigin, "44", "2")
	second := dialGroup(t, server.URL, testOrigin, "44", "2")
	if count := waitForCount(t, hub, 44, 2); count != 2 {
		t.Fatalf("connection count = %d", count)
	}
	if err := first.WriteMessage(websocket.TextMessage, []byte(`{"event":"group.send","requestId":"r1","clientMsgId":"c1","data":{"groupID":9}}`)); err != nil {
		t.Fatal(err)
	}
	response := readEnvelope(t, first)
	if response.Event != protocol.EventError || response.RequestID != "r1" || response.ClientMsgID != "c1" || response.Error == nil || response.Error.Code != protocol.ErrorUnsupported {
		t.Fatalf("unsupported response = %#v", response)
	}
	if err := first.WriteMessage(websocket.TextMessage, []byte(`{"invalid":true}`)); err != nil {
		t.Fatal(err)
	}
	response = readEnvelope(t, first)
	if response.Error == nil || response.Error.Code != protocol.ErrorMalformed {
		t.Fatalf("malformed response = %#v", response)
	}
	if err := first.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if count := waitForCount(t, hub, 44, 1); count != 1 {
		t.Fatalf("sibling count = %d", count)
	}
	if err := second.WriteMessage(websocket.TextMessage, []byte(`{"event":"group.send","requestId":"r2","data":{}}`)); err != nil {
		t.Fatal(err)
	}
	if response = readEnvelope(t, second); response.RequestID != "r2" {
		t.Fatalf("sibling response = %#v", response)
	}
	_ = first.Close()
	_ = second.Close()
	if count := waitForCount(t, hub, 44, 0); count != 0 {
		t.Fatalf("final count = %d", count)
	}
}

func TestGroupEndpointOriginIdentityAndPeerPing(t *testing.T) {
	hub, _ := runningHub(t)
	server := groupWebSocketServer(t, hub, testHeartbeat(), nil)
	connection := dialGroup(t, server.URL, testOrigin, "55", "2")
	pong := make(chan string, 1)
	connection.SetPongHandler(func(payload string) error {
		pong <- payload
		return nil
	})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()
	if err := connection.WriteControl(websocket.PingMessage, []byte("peer"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case payload := <-pong:
		if payload != "peer" {
			t.Fatalf("pong payload = %q", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("peer Ping did not receive Pong")
	}
	_ = connection.Close()
	<-readerDone

	for _, test := range []struct {
		origin, userID, role string
		status               int
	}{
		{origin: "http://evil.example", userID: "55", role: "2", status: http.StatusForbidden},
		{userID: "55", role: "2", status: http.StatusForbidden},
		{origin: testOrigin, role: "2", status: http.StatusUnauthorized},
		{origin: testOrigin, userID: "55", role: "9", status: http.StatusUnauthorized},
	} {
		header := http.Header{}
		if test.origin != "" {
			header.Set("Origin", test.origin)
		}
		header.Set("User-ID", test.userID)
		header.Set("Role", test.role)
		candidate, response, err := websocket.DefaultDialer.Dial(groupURL(server.URL), header)
		if candidate != nil {
			_ = candidate.Close()
		}
		if err == nil || response == nil || response.StatusCode != test.status {
			t.Fatalf("rejection status=%v expected=%d err=%v", responseStatus(response), test.status, err)
		}
		_ = response.Body.Close()
	}
}

func TestGroupHeartbeatTimeoutPreservesHealthySibling(t *testing.T) {
	heartbeat := HeartbeatConfig{ReadLimit: 1 << 20, PongWait: 300 * time.Millisecond, PingPeriod: 60 * time.Millisecond, WriteWait: 40 * time.Millisecond}
	hub, _ := runningHub(t)
	server := groupWebSocketServer(t, hub, heartbeat, nil)
	dead := dialGroup(t, server.URL, testOrigin, "61", "2")
	healthy := dialGroup(t, server.URL, testOrigin, "61", "2")
	pingSeen, readerDone := startPongingReader(healthy)
	if count := waitForCount(t, hub, 61, 2); count != 2 {
		t.Fatalf("initial count = %d", count)
	}
	select {
	case <-pingSeen:
	case <-time.After(time.Second):
		t.Fatal("healthy client did not receive Ping")
	}
	if count := waitForCount(t, hub, 61, 1); count != 1 {
		t.Fatalf("timeout sibling count = %d", count)
	}
	time.Sleep(heartbeat.PongWait + heartbeat.PingPeriod)
	if count := waitForCount(t, hub, 61, 1); count != 1 {
		t.Fatalf("healthy sibling lost: %d", count)
	}
	_ = dead.Close()
	_ = healthy.Close()
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("reader did not stop")
	}
}

func TestGroupAbnormalCloseCleanup(t *testing.T) {
	hub, _ := runningHub(t)
	server := groupWebSocketServer(t, hub, testHeartbeat(), nil)
	connection := dialGroup(t, server.URL, testOrigin, "62", "2")
	if count := waitForCount(t, hub, 62, 1); count != 1 {
		t.Fatalf("count = %d", count)
	}
	if err := connection.UnderlyingConn().Close(); err != nil {
		t.Fatal(err)
	}
	if count := waitForCount(t, hub, 62, 0); count != 0 {
		t.Fatalf("abnormal close count = %d", count)
	}
}

func TestGroupHubShutdownClosesClients(t *testing.T) {
	hub, _ := runningHub(t)
	server := groupWebSocketServer(t, hub, testHeartbeat(), nil)
	connection := dialGroup(t, server.URL, testOrigin, "70", "2")
	_, readerDone := startPongingReader(connection)
	if count := waitForCount(t, hub, 70, 1); count != 1 {
		t.Fatalf("count = %d", count)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("client reader did not stop")
	}
	_ = connection.Close()
}

func TestGroupServerAndHubGracefulShutdown(t *testing.T) {
	hub, _ := runningHub(t)
	policy, err := NewOriginPolicy([]string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(hub, NewFoundationInbound(hub), 8, policy, testHeartbeat(), nil)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/group/ws/chat", handler)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()
	serverURL := "http://" + listener.Addr().String()
	connection := dialGroup(t, serverURL, testOrigin, "63", "2")
	_, readerDone := startPongingReader(connection)
	if count := waitForCount(t, hub, 63, 1); count != 1 {
		t.Fatalf("count = %d", count)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-serverDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("server exit: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server shutdown timed out")
	}
	select {
	case <-readerDone:
	case <-time.After(time.Second):
		t.Fatal("client reader did not stop")
	}
	header := http.Header{"Origin": []string{testOrigin}, "User-ID": []string{"63"}, "Role": []string{"2"}}
	retry, response, dialErr := websocket.DefaultDialer.Dial(groupURL(serverURL), header)
	if retry != nil {
		_ = retry.Close()
	}
	if response != nil {
		_ = response.Body.Close()
	}
	if dialErr == nil {
		t.Fatal("server accepted a connection after shutdown")
	}
	_ = connection.Close()
}

func groupWebSocketServer(t *testing.T, hub *Hub, heartbeat HeartbeatConfig, observer LifecycleObserver) *httptest.Server {
	t.Helper()
	policy, err := NewOriginPolicy([]string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(hub, NewFoundationInbound(hub), 8, policy, heartbeat, observer)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/group/ws/chat", handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func dialGroup(t *testing.T, serverURL, origin, userID, role string) *websocket.Conn {
	t.Helper()
	header := http.Header{"Origin": []string{origin}, "User-ID": []string{userID}, "Role": []string{role}}
	connection, response, err := websocket.DefaultDialer.Dial(groupURL(serverURL), header)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		t.Fatalf("Dial() error = %v", err)
	}
	return connection
}

func groupURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/api/group/ws/chat"
}

func readEnvelope(t *testing.T, connection *websocket.Conn) protocol.Envelope {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := connection.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var envelope protocol.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func startPongingReader(connection *websocket.Conn) (<-chan struct{}, <-chan struct{}) {
	pingSeen := make(chan struct{}, 16)
	done := make(chan struct{})
	connection.SetPingHandler(func(payload string) error {
		select {
		case pingSeen <- struct{}{}:
		default:
		}
		return connection.WriteControl(websocket.PongMessage, []byte(payload), time.Now().Add(time.Second))
	})
	go func() {
		defer close(done)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()
	return pingSeen, done
}

func waitForCount(t *testing.T, hub *Hub, userID uint64, expected int) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		count, err := hub.UserConnectionCount(context.Background(), userID)
		if err == nil && count == expected {
			return count
		}
		if time.Now().After(deadline) {
			return count
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func testHeartbeat() HeartbeatConfig {
	return HeartbeatConfig{ReadLimit: 1 << 20, PongWait: time.Second, PingPeriod: 250 * time.Millisecond, WriteWait: 100 * time.Millisecond}
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
