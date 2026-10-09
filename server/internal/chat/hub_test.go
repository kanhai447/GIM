package chat

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSocket struct {
	closed     chan struct{}
	closeOnce  sync.Once
	writes     chan []byte
	active     atomic.Int32
	concurrent atomic.Bool
}

func newFakeSocket(writeCapacity int) *fakeSocket {
	return &fakeSocket{closed: make(chan struct{}), writes: make(chan []byte, writeCapacity)}
}

func (connection *fakeSocket) ReadMessage() (int, []byte, error) {
	<-connection.closed
	return 0, nil, errors.New("closed")
}

func (connection *fakeSocket) WriteMessage(_ int, payload []byte) error {
	if connection.active.Add(1) != 1 {
		connection.concurrent.Store(true)
	}
	defer connection.active.Add(-1)
	select {
	case <-connection.closed:
		return errors.New("closed")
	case connection.writes <- append([]byte(nil), payload...):
		return nil
	}
}

func (connection *fakeSocket) Close() error {
	connection.closeOnce.Do(func() { close(connection.closed) })
	return nil
}

func TestHubRegisterUnregisterMultiClientAndShutdown(t *testing.T) {
	hub, cancel := runningHub(t)
	defer cancel()
	clientA := testClient(hub, 10, "client-a", 4)
	clientB := testClient(hub, 10, "client-b", 4)
	clientC := testClient(hub, 11, "client-c", 4)
	for _, client := range []*Client{clientA, clientB, clientC} {
		if err := hub.Register(context.Background(), client); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	if count := connectionCount(t, hub, 10); count != 2 {
		t.Fatalf("user 10 connection count = %d", count)
	}
	if count := connectionCount(t, hub, 11); count != 1 {
		t.Fatalf("user 11 connection count = %d", count)
	}
	duplicate := newClient(10, "client-b", newFakeSocket(1), hub, UnavailableInboundHandler{}, 1)
	if err := hub.Register(context.Background(), duplicate); !errors.Is(err, ErrDuplicateClient) {
		t.Fatalf("duplicate Register() error = %v", err)
	}
	if !hub.Unregister(context.Background(), clientA) {
		t.Fatal("first unregister did not remove client")
	}
	if hub.Unregister(context.Background(), clientA) {
		t.Fatal("duplicate unregister removed a client")
	}
	if count := connectionCount(t, hub, 10); count != 1 {
		t.Fatalf("remaining user 10 connection count = %d", count)
	}
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := hub.Shutdown(shutdownContext); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	for _, client := range []*Client{clientB, clientC} {
		select {
		case <-client.conn.(*fakeSocket).closed:
		default:
			t.Fatalf("client %s connection was not closed", client.ClientID)
		}
	}
}

func TestHubSlowClientDoesNotBlockOtherClients(t *testing.T) {
	hub, cancel := runningHub(t)
	defer cancel()
	slow := testClient(hub, 20, "slow", 1)
	healthy := testClient(hub, 21, "healthy", 1)
	if err := hub.Register(context.Background(), slow); err != nil {
		t.Fatal(err)
	}
	if err := hub.Register(context.Background(), healthy); err != nil {
		t.Fatal(err)
	}
	if result, err := hub.SendToClient(context.Background(), 20, "slow", []byte("first")); err != nil || result.Delivered != 1 {
		t.Fatalf("first delivery = %#v, %v", result, err)
	}
	deadline, deadlineCancel := context.WithTimeout(context.Background(), time.Second)
	defer deadlineCancel()
	result, err := hub.SendToClient(deadline, 20, "slow", []byte("second"))
	if err != nil || result.Dropped != 1 {
		t.Fatalf("slow delivery = %#v, %v", result, err)
	}
	if count := connectionCount(t, hub, 20); count != 0 {
		t.Fatalf("slow client remained registered: %d", count)
	}
	if count := connectionCount(t, hub, 21); count != 1 {
		t.Fatalf("healthy client was affected: %d", count)
	}
}

func TestConcurrentLogicalSendsUseSingleWriter(t *testing.T) {
	hub, cancel := runningHub(t)
	defer cancel()
	connection := newFakeSocket(128)
	client := newClient(30, "writer", connection, hub, UnavailableInboundHandler{}, 128)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.writePump()
	const messageCount = 64
	var wait sync.WaitGroup
	for index := 0; index < messageCount; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := hub.SendToClient(context.Background(), 30, "writer", []byte("message"))
			if err != nil || result.Delivered != 1 {
				t.Errorf("SendToClient() = %#v, %v", result, err)
			}
		}()
	}
	wait.Wait()
	for index := 0; index < messageCount; index++ {
		select {
		case <-connection.writes:
		case <-time.After(time.Second):
			t.Fatalf("received %d/%d writes", index, messageCount)
		}
	}
	if connection.concurrent.Load() {
		t.Fatal("connection observed concurrent WriteMessage calls")
	}
	hub.Unregister(context.Background(), client)
}

func TestClientIDIsRandomAndOpaque(t *testing.T) {
	first, err := newClientID()
	if err != nil {
		t.Fatalf("newClientID() error = %v", err)
	}
	second, err := newClientID()
	if err != nil {
		t.Fatalf("newClientID() error = %v", err)
	}
	if len(first) != 32 || len(second) != 32 || first == second {
		t.Fatalf("client IDs are not independent 128-bit values: lengths=%d/%d equal=%t", len(first), len(second), first == second)
	}
}

func runningHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(func() {
		cancel()
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = hub.Shutdown(shutdownContext)
	})
	return hub, cancel
}

func testClient(hub *Hub, userID uint64, clientID string, buffer int) *Client {
	return newClient(userID, clientID, newFakeSocket(1), hub, UnavailableInboundHandler{}, buffer)
}

func connectionCount(t *testing.T, hub *Hub, userID uint64) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	count, err := hub.UserConnectionCount(ctx, userID)
	if err != nil {
		t.Fatalf("UserConnectionCount() error = %v", err)
	}
	return count
}
