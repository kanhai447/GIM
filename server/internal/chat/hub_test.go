package chat

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/presence"
)

type fakeSocket struct {
	closed        chan struct{}
	closeOnce     sync.Once
	writes        chan []byte
	controls      chan int
	active        atomic.Int32
	concurrent    atomic.Bool
	pongMu        sync.Mutex
	pong          func(string) error
	ping          func(string) error
	closeHandler  func(int, string) error
	readLimit     atomic.Int64
	readDeadline  atomic.Int64
	writeDeadline atomic.Int64
}

type recordingTransitionObserver struct {
	mu      sync.Mutex
	online  []uint64
	offline []uint64
}

type timeoutPresenceStore struct {
	started chan struct{}
	once    sync.Once
}

func (store *timeoutPresenceStore) MarkOnline(ctx context.Context, _ uint64, _ string, _ time.Time, _ time.Duration) error {
	store.once.Do(func() { close(store.started) })
	<-ctx.Done()
	return ctx.Err()
}

func (store *timeoutPresenceStore) MarkOffline(ctx context.Context, _ uint64, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (store *timeoutPresenceStore) IsOnline(ctx context.Context, _ uint64, _ time.Time) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}

func (observer *recordingTransitionObserver) LocalUserOnline(userID uint64) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.online = append(observer.online, userID)
}

func (observer *recordingTransitionObserver) LocalUserOffline(userID uint64) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.offline = append(observer.offline, userID)
}

func (observer *recordingTransitionObserver) counts() (int, int) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return len(observer.online), len(observer.offline)
}

func newFakeSocket(writeCapacity int) *fakeSocket {
	return &fakeSocket{closed: make(chan struct{}), writes: make(chan []byte, writeCapacity), controls: make(chan int, 128)}
}

func (connection *fakeSocket) WriteControl(messageType int, _ []byte, _ time.Time) error {
	if connection.active.Add(1) != 1 {
		connection.concurrent.Store(true)
	}
	defer connection.active.Add(-1)
	select {
	case <-connection.closed:
		return errors.New("closed")
	case connection.controls <- messageType:
		return nil
	}
}

func (connection *fakeSocket) SetReadLimit(limit int64) { connection.readLimit.Store(limit) }

func (connection *fakeSocket) SetReadDeadline(deadline time.Time) error {
	connection.readDeadline.Store(deadline.UnixNano())
	return nil
}

func (connection *fakeSocket) SetPongHandler(handler func(string) error) {
	connection.pongMu.Lock()
	defer connection.pongMu.Unlock()
	connection.pong = handler
}

func (connection *fakeSocket) SetPingHandler(handler func(string) error) {
	connection.pongMu.Lock()
	defer connection.pongMu.Unlock()
	connection.ping = handler
}

func (connection *fakeSocket) SetCloseHandler(handler func(int, string) error) {
	connection.pongMu.Lock()
	defer connection.pongMu.Unlock()
	connection.closeHandler = handler
}

func (connection *fakeSocket) SetWriteDeadline(deadline time.Time) error {
	connection.writeDeadline.Store(deadline.UnixNano())
	return nil
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
	duplicate := newClient(10, "client-b", newFakeSocket(1), hub, UnavailableInboundHandler{}, 1, testHeartbeat(), nil)
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

func TestHubPresenceTransitionsFollowUserConnectionCount(t *testing.T) {
	observer := &recordingTransitionObserver{}
	hub := NewHub(observer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	clientA := testClient(hub, 12, "device-a", 2)
	clientB := testClient(hub, 12, "device-b", 2)

	if err := hub.Register(context.Background(), clientA); err != nil {
		t.Fatal(err)
	}
	if online, offline := observer.counts(); online != 1 || offline != 0 {
		t.Fatalf("0->1 transitions online=%d offline=%d", online, offline)
	}
	if err := hub.Register(context.Background(), clientB); err != nil {
		t.Fatal(err)
	}
	if online, offline := observer.counts(); online != 1 || offline != 0 {
		t.Fatalf("1->2 duplicated transition online=%d offline=%d", online, offline)
	}
	if !hub.Unregister(context.Background(), clientA, DisconnectTimeout) {
		t.Fatal("first device was not removed")
	}
	if online, offline := observer.counts(); online != 1 || offline != 0 {
		t.Fatalf("2->1 emitted offline online=%d offline=%d", online, offline)
	}
	if !hub.Unregister(context.Background(), clientB, DisconnectNormal) {
		t.Fatal("final device was not removed")
	}
	if online, offline := observer.counts(); online != 1 || offline != 1 {
		t.Fatalf("1->0 transitions online=%d offline=%d", online, offline)
	}
	if hub.Unregister(context.Background(), clientB, DisconnectAbnormal) {
		t.Fatal("duplicate unregister removed client")
	}
	if online, offline := observer.counts(); online != 1 || offline != 1 {
		t.Fatalf("duplicate unregister changed transitions online=%d offline=%d", online, offline)
	}
}

func TestSlowClientAndShutdownUseSamePresenceTransitions(t *testing.T) {
	observer := &recordingTransitionObserver{}
	hub := NewHub(observer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	slow := testClient(hub, 13, "slow", 1)
	healthy := testClient(hub, 13, "healthy", 2)
	other := testClient(hub, 14, "other", 2)
	for _, client := range []*Client{slow, healthy, other} {
		if err := hub.Register(context.Background(), client); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := hub.SendToClient(context.Background(), 13, "slow", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if result, err := hub.SendToClient(context.Background(), 13, "slow", []byte("second")); err != nil || result.Dropped != 1 {
		t.Fatalf("slow removal = %#v, %v", result, err)
	}
	if online, offline := observer.counts(); online != 2 || offline != 0 {
		t.Fatalf("slow sibling removal changed user Presence online=%d offline=%d", online, offline)
	}
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := hub.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
	if online, offline := observer.counts(); online != 2 || offline != 2 {
		t.Fatalf("shutdown transitions online=%d offline=%d", online, offline)
	}
}

func TestRedisTimeoutDoesNotBlockHubRegisterOrUnregister(t *testing.T) {
	store := &timeoutPresenceStore{started: make(chan struct{})}
	presenceService, err := presence.NewService(store, presence.Config{
		InstanceID: "chat-timeout", TTL: time.Second, RefreshInterval: 300 * time.Millisecond,
		RetryInterval: 100 * time.Millisecond, OperationTimeout: 300 * time.Millisecond,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	presenceContext, presenceCancel := context.WithCancel(context.Background())
	go presenceService.Run(presenceContext)
	defer presenceCancel()
	hub := NewHub(presenceService)
	hubContext, hubCancel := context.WithCancel(context.Background())
	go hub.Run(hubContext)
	defer hubCancel()

	client := testClient(hub, 15, "redis-timeout", 2)
	started := time.Now()
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 150*time.Millisecond {
		t.Fatalf("Hub Register blocked on Redis for %v", elapsed)
	}
	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("Presence worker did not attempt Redis operation")
	}
	started = time.Now()
	if !hub.Unregister(context.Background(), client, DisconnectTimeout) {
		t.Fatal("Hub Unregister did not remove client")
	}
	if elapsed := time.Since(started); elapsed > 150*time.Millisecond {
		t.Fatalf("Hub Unregister blocked on Redis for %v", elapsed)
	}
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := hub.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	if err := presenceService.Shutdown(shutdownContext); err == nil {
		t.Fatal("Presence shutdown unexpectedly succeeded against timed-out Redis")
	}
	if elapsed := time.Since(started); elapsed > 750*time.Millisecond {
		t.Fatalf("Presence shutdown blocked too long: %v", elapsed)
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
	client := newClient(30, "writer", connection, hub, UnavailableInboundHandler{}, 128, testHeartbeat(), nil)
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
	return newClient(userID, clientID, newFakeSocket(1), hub, UnavailableInboundHandler{}, buffer, testHeartbeat(), nil)
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
