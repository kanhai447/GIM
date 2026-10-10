package chat

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
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

const testOrigin = "http://localhost:5173"

func TestChatEndpointMultiClientOutboundAndCleanup(t *testing.T) {
	hub, _ := runningHub(t)
	policy, err := NewOriginPolicy([]string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	inbound := InboundHandlerFunc(func(ctx context.Context, identity ClientIdentity, envelope protocol.Envelope) error {
		response, marshalErr := json.Marshal(protocol.Envelope{Event: "foundation.outbound", RequestID: envelope.RequestID, Data: json.RawMessage(`{"accepted":true}`)})
		if marshalErr != nil {
			return marshalErr
		}
		_, deliveryErr := hub.SendToClient(ctx, identity.UserID, identity.ClientID, response)
		return deliveryErr
	})
	handler, err := NewHandler(hub, inbound, 8, policy, testHeartbeat(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/chat/ws/chat", handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	first := dialChat(t, server.URL, testOrigin, "44", "2")
	second := dialChat(t, server.URL, testOrigin, "44", "2")
	if count := waitForCount(t, hub, 44, 2); count != 2 {
		t.Fatalf("connection count = %d", count)
	}

	if err := first.WriteMessage(websocket.TextMessage, []byte(`{"event":"foundation.inbound","requestId":"one","data":{}}`)); err != nil {
		t.Fatalf("write inbound frame: %v", err)
	}
	_, payload, err := first.ReadMessage()
	if err != nil {
		t.Fatalf("read outbound frame: %v", err)
	}
	var response protocol.Envelope
	if err := json.Unmarshal(payload, &response); err != nil || response.Event != "foundation.outbound" || response.RequestID != "one" {
		t.Fatalf("outbound envelope = %#v, %v", response, err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first connection: %v", err)
	}
	if count := waitForCount(t, hub, 44, 1); count != 1 {
		t.Fatalf("remaining connection count = %d", count)
	}
	broadcast := []byte(`{"event":"foundation.broadcast","data":{}}`)
	result, err := hub.SendToUser(context.Background(), 44, broadcast)
	if err != nil || result.Delivered != 1 || result.Dropped != 0 {
		t.Fatalf("SendToUser() = %#v, %v", result, err)
	}
	_, payload, err = second.ReadMessage()
	if err != nil || string(payload) != string(broadcast) {
		t.Fatalf("remaining client read = %q, %v", payload, err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second connection: %v", err)
	}
	if count := waitForCount(t, hub, 44, 0); count != 0 {
		t.Fatalf("final connection count = %d", count)
	}
}

func TestChatEndpointOriginAndIdentityPolicy(t *testing.T) {
	hub, _ := runningHub(t)
	policy, _ := NewOriginPolicy([]string{testOrigin})
	handler, err := NewHandler(hub, UnavailableInboundHandler{}, 4, policy, testHeartbeat(), nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	allowed := dialChat(t, server.URL, testOrigin, "55", "2")
	_ = allowed.Close()

	for _, test := range []struct {
		name   string
		origin string
		userID string
		role   string
		status int
	}{
		{name: "disallowed origin", origin: "http://evil.example", userID: "55", role: "2", status: http.StatusForbidden},
		{name: "missing origin", userID: "55", role: "2", status: http.StatusForbidden},
		{name: "missing identity", origin: testOrigin, role: "2", status: http.StatusUnauthorized},
		{name: "invalid role", origin: testOrigin, userID: "55", role: "9", status: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			if test.origin != "" {
				header.Set("Origin", test.origin)
			}
			if test.userID != "" {
				header.Set("User-ID", test.userID)
			}
			if test.role != "" {
				header.Set("Role", test.role)
			}
			connection, response, err := websocket.DefaultDialer.Dial(websocketURL(server.URL), header)
			if connection != nil {
				_ = connection.Close()
			}
			if err == nil || response == nil || response.StatusCode != test.status {
				t.Fatalf("Dial() status=%v err=%v", responseStatus(response), err)
			}
			_ = response.Body.Close()
		})
	}
}

func TestHeartbeatTimeoutKeepsHealthySiblingConnected(t *testing.T) {
	heartbeat := HeartbeatConfig{ReadLimit: 1 << 20, PongWait: 300 * time.Millisecond, PingPeriod: 60 * time.Millisecond, WriteWait: 40 * time.Millisecond}
	hub, server, observer := heartbeatServer(t, heartbeat)
	dead := dialChat(t, server.URL, testOrigin, "61", "2")
	healthy := dialChat(t, server.URL, testOrigin, "61", "2")
	pingSeen, readerDone := startPongingReader(healthy)
	if count := waitForCount(t, hub, 61, 2); count != 2 {
		t.Fatalf("initial user connection count = %d", count)
	}
	if total := totalConnectionCount(t, hub); total != 2 {
		t.Fatalf("initial total connection count = %d", total)
	}
	select {
	case <-pingSeen:
	case <-time.After(time.Second):
		t.Fatal("healthy client did not receive Ping")
	}
	if count := waitForCount(t, hub, 61, 1); count != 1 {
		t.Fatalf("connection count after sibling timeout = %d", count)
	}
	time.Sleep(heartbeat.PongWait + heartbeat.PingPeriod)
	if count := waitForCount(t, hub, 61, 1); count != 1 {
		t.Fatalf("healthy sibling did not remain connected: %d", count)
	}
	timeoutEvent := waitLifecycleEvent(t, observer, time.Second)
	if timeoutEvent.reason != DisconnectTimeout {
		t.Fatalf("dead client reason = %s", timeoutEvent.reason)
	}
	if err := healthy.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "test complete"), time.Now().Add(time.Second)); err != nil {
		t.Fatalf("send normal close: %v", err)
	}
	if count := waitForCount(t, hub, 61, 0); count != 0 {
		t.Fatalf("final user connection count = %d", count)
	}
	normalEvent := waitLifecycleEvent(t, observer, time.Second)
	if normalEvent.reason != DisconnectNormal {
		t.Fatalf("healthy client reason = %s", normalEvent.reason)
	}
	_ = dead.Close()
	_ = healthy.Close()
	waitChannel(t, readerDone, time.Second, "healthy client reader")
}

func TestAbnormalCloseRemovesClient(t *testing.T) {
	hub, server, observer := heartbeatServer(t, shortHeartbeat())
	connection := dialChat(t, server.URL, testOrigin, "62", "2")
	if count := waitForCount(t, hub, 62, 1); count != 1 {
		t.Fatalf("connection count = %d", count)
	}
	if err := connection.UnderlyingConn().Close(); err != nil {
		t.Fatalf("abrupt close: %v", err)
	}
	if count := waitForCount(t, hub, 62, 0); count != 0 {
		t.Fatalf("connection count after abrupt close = %d", count)
	}
	event := waitLifecycleEvent(t, observer, time.Second)
	if event.reason != DisconnectAbnormal {
		t.Fatalf("disconnect reason = %s", event.reason)
	}
}

func TestHubShutdownClosesHeartbeatConnection(t *testing.T) {
	hub, server, observer := heartbeatServer(t, shortHeartbeat())
	connection := dialChat(t, server.URL, testOrigin, "63", "2")
	_, readerDone := startPongingReader(connection)
	if count := waitForCount(t, hub, 63, 1); count != 1 {
		t.Fatalf("connection count = %d", count)
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := hub.Shutdown(shutdownContext); err != nil {
		t.Fatalf("Hub.Shutdown() error = %v", err)
	}
	waitChannel(t, readerDone, time.Second, "client reader")
	event := waitLifecycleEvent(t, observer, time.Second)
	if event.reason != DisconnectShutdown {
		t.Fatalf("disconnect reason = %s", event.reason)
	}
	_ = connection.Close()
}

func TestServerAndHubGracefulShutdownLifecycle(t *testing.T) {
	hub := NewHub()
	hubContext, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go hub.Run(hubContext)
	policy, err := NewOriginPolicy([]string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	observer := newRecordingLifecycleObserver()
	handler, err := NewHandler(hub, UnavailableInboundHandler{}, 8, policy, shortHeartbeat(), observer)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()
	serverURL := "http://" + listener.Addr().String()
	connection := dialChat(t, serverURL, testOrigin, "64", "2")
	_, readerDone := startPongingReader(connection)
	if count := waitForCount(t, hub, 64, 1); count != 1 {
		t.Fatalf("connection count = %d", count)
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		t.Fatalf("server shutdown: %v", err)
	}
	if err := hub.Shutdown(shutdownContext); err != nil {
		t.Fatalf("hub shutdown: %v", err)
	}
	select {
	case err := <-serverDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("server exit: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not exit")
	}
	waitChannel(t, readerDone, time.Second, "client reader")
	if event := waitLifecycleEvent(t, observer, time.Second); event.reason != DisconnectShutdown {
		t.Fatalf("disconnect reason = %s", event.reason)
	}
	header := http.Header{"Origin": []string{testOrigin}, "User-ID": []string{"64"}, "Role": []string{"2"}}
	retry, response, dialErr := websocket.DefaultDialer.Dial(websocketURL(serverURL), header)
	if retry != nil {
		_ = retry.Close()
	}
	if response != nil {
		_ = response.Body.Close()
	}
	if dialErr == nil {
		t.Fatal("server accepted a new connection after shutdown")
	}
	_ = connection.Close()
}

func heartbeatServer(t *testing.T, heartbeat HeartbeatConfig) (*Hub, *httptest.Server, *recordingLifecycleObserver) {
	t.Helper()
	hub, _ := runningHub(t)
	policy, err := NewOriginPolicy([]string{testOrigin})
	if err != nil {
		t.Fatal(err)
	}
	observer := newRecordingLifecycleObserver()
	handler, err := NewHandler(hub, UnavailableInboundHandler{}, 8, policy, heartbeat, observer)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return hub, server, observer
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

func testHeartbeat() HeartbeatConfig {
	return HeartbeatConfig{ReadLimit: 1 << 20, PongWait: time.Second, PingPeriod: 250 * time.Millisecond, WriteWait: 100 * time.Millisecond}
}

func dialChat(t *testing.T, serverURL, origin, userID, role string) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", origin)
	header.Set("User-ID", userID)
	header.Set("Role", role)
	connection, response, err := websocket.DefaultDialer.Dial(websocketURL(serverURL), header)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		t.Fatalf("Dial() error = %v", err)
	}
	return connection
}

func websocketURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/api/chat/ws/chat"
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

func totalConnectionCount(t *testing.T, hub *Hub) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	count, err := hub.TotalConnectionCount(ctx)
	if err != nil {
		t.Fatalf("TotalConnectionCount() error = %v", err)
	}
	return count
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
