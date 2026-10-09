package chat

import (
	"context"
	"encoding/json"
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
	handler := NewHandler(hub, inbound, 8, policy)
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
	handler := NewHandler(hub, UnavailableInboundHandler{}, 4, policy)
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

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
