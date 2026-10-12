package ws

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
	writes     atomic.Int64
	active     atomic.Int32
	concurrent atomic.Bool
	pong       func(string) error
	ping       func(string) error
	closeFn    func(int, string) error
}

func newFakeSocket() *fakeSocket { return &fakeSocket{closed: make(chan struct{})} }
func (socket *fakeSocket) ReadMessage() (int, []byte, error) {
	<-socket.closed
	return 0, nil, errors.New("closed")
}
func (socket *fakeSocket) WriteMessage(int, []byte) error            { return socket.recordWrite() }
func (socket *fakeSocket) WriteControl(int, []byte, time.Time) error { return socket.recordWrite() }
func (socket *fakeSocket) recordWrite() error {
	if socket.active.Add(1) != 1 {
		socket.concurrent.Store(true)
	}
	time.Sleep(100 * time.Microsecond)
	socket.writes.Add(1)
	socket.active.Add(-1)
	return nil
}
func (*fakeSocket) SetReadLimit(int64)                                {}
func (*fakeSocket) SetReadDeadline(time.Time) error                   { return nil }
func (*fakeSocket) SetWriteDeadline(time.Time) error                  { return nil }
func (socket *fakeSocket) SetPongHandler(fn func(string) error)       { socket.pong = fn }
func (socket *fakeSocket) SetPingHandler(fn func(string) error)       { socket.ping = fn }
func (socket *fakeSocket) SetCloseHandler(fn func(int, string) error) { socket.closeFn = fn }
func (socket *fakeSocket) Close() error {
	socket.closeOnce.Do(func() { close(socket.closed) })
	return nil
}

func TestHubMultiDeviceAndDuplicateCleanup(t *testing.T) {
	hub, cancel := runningHub(t)
	defer cancel()
	first := testClient(9, "a", newFakeSocket(), hub, 2)
	second := testClient(9, "b", newFakeSocket(), hub, 2)
	if err := hub.Register(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := hub.Register(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if count, _ := hub.UserConnectionCount(context.Background(), 9); count != 2 {
		t.Fatalf("count = %d", count)
	}
	if !hub.Unregister(context.Background(), first, DisconnectNormal) {
		t.Fatal("first unregister failed")
	}
	if hub.Unregister(context.Background(), first, DisconnectNormal) {
		t.Fatal("duplicate unregister succeeded")
	}
	if count, _ := hub.UserConnectionCount(context.Background(), 9); count != 1 {
		t.Fatalf("sibling count = %d", count)
	}
	if !hub.Unregister(context.Background(), second, DisconnectNormal) {
		t.Fatal("second unregister failed")
	}
}

func TestHubSlowClientRemovalPreservesSibling(t *testing.T) {
	hub, cancel := runningHub(t)
	defer cancel()
	slow := testClient(10, "slow", newFakeSocket(), hub, 1)
	healthySocket := newFakeSocket()
	healthy := testClient(10, "healthy", healthySocket, hub, 8)
	if err := hub.Register(context.Background(), slow); err != nil {
		t.Fatal(err)
	}
	if err := hub.Register(context.Background(), healthy); err != nil {
		t.Fatal(err)
	}
	go healthy.writePump()
	if result, err := hub.SendToUser(context.Background(), 10, []byte("one")); err != nil || result.Delivered != 2 {
		t.Fatalf("first delivery = %#v, %v", result, err)
	}
	deadline := time.Now().Add(time.Second)
	for healthySocket.writes.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	result, err := hub.SendToUser(context.Background(), 10, []byte("two"))
	if err != nil || result.Delivered != 1 || result.Dropped != 1 {
		t.Fatalf("second delivery = %#v, %v", result, err)
	}
	if count, _ := hub.UserConnectionCount(context.Background(), 10); count != 1 {
		t.Fatalf("healthy sibling count = %d", count)
	}
	hub.Unregister(context.Background(), healthy, DisconnectNormal)
}

func TestConcurrentLogicalSendsUseSingleWriter(t *testing.T) {
	hub, cancel := runningHub(t)
	defer cancel()
	socket := newFakeSocket()
	client := testClient(11, "stress", socket, hub, 256)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.writePump()
	const sends = 100
	var group sync.WaitGroup
	for index := 0; index < sends; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := hub.SendToUser(context.Background(), 11, []byte("payload")); err != nil {
				t.Errorf("SendToUser: %v", err)
			}
		}()
	}
	group.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for socket.writes.Load() < sends && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if socket.writes.Load() != sends {
		t.Fatalf("writes = %d", socket.writes.Load())
	}
	if socket.concurrent.Load() {
		t.Fatal("concurrent socket write detected")
	}
	hub.Unregister(context.Background(), client, DisconnectNormal)
}

func runningHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(func() {
		shutdown, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = hub.Shutdown(shutdown)
		cancel()
	})
	return hub, cancel
}

func testClient(userID uint64, clientID string, socket socketConnection, hub *Hub, buffer int) *Client {
	return newClient(userID, clientID, socket, hub, NewFoundationInbound(hub), buffer, HeartbeatConfig{
		ReadLimit: 1 << 20, PongWait: time.Hour, PingPeriod: 30 * time.Minute, WriteWait: time.Second,
	}, nil)
}
