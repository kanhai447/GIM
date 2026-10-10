package chat

import (
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type lifecycleEvent struct {
	identity ClientIdentity
	reason   DisconnectReason
}

type recordingLifecycleObserver struct {
	connected    chan ClientIdentity
	disconnected chan lifecycleEvent
}

func newRecordingLifecycleObserver() *recordingLifecycleObserver {
	return &recordingLifecycleObserver{
		connected: make(chan ClientIdentity, 32), disconnected: make(chan lifecycleEvent, 32),
	}
}

func (observer *recordingLifecycleObserver) Connected(identity ClientIdentity) {
	observer.connected <- identity
}

func (observer *recordingLifecycleObserver) Disconnected(identity ClientIdentity, reason DisconnectReason) {
	observer.disconnected <- lifecycleEvent{identity: identity, reason: reason}
}

func TestPongRefreshesReadDeadline(t *testing.T) {
	hub, _ := runningHub(t)
	connection := newFakeSocket(1)
	heartbeat := shortHeartbeat()
	client := newClient(40, "pong", connection, hub, UnavailableInboundHandler{}, 1, heartbeat, nil)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.readPump(context.Background())
	waitForCondition(t, time.Second, func() bool {
		connection.pongMu.Lock()
		defer connection.pongMu.Unlock()
		return connection.pong != nil
	})
	if got := connection.readLimit.Load(); got != heartbeat.ReadLimit {
		t.Fatalf("read limit = %d", got)
	}
	initial := connection.readDeadline.Load()
	time.Sleep(20 * time.Millisecond)
	connection.pongMu.Lock()
	handler := connection.pong
	connection.pongMu.Unlock()
	if err := handler("heartbeat"); err != nil {
		t.Fatalf("pong handler: %v", err)
	}
	if refreshed := connection.readDeadline.Load(); refreshed <= initial {
		t.Fatalf("pong did not refresh deadline: initial=%d refreshed=%d", initial, refreshed)
	}
	hub.Unregister(context.Background(), client, DisconnectNormal)
	waitChannel(t, client.readDone, time.Second, "readPump")
}

func TestWritePumpOwnsPingAndStopsTicker(t *testing.T) {
	hub, _ := runningHub(t)
	connection := newFakeSocket(1)
	heartbeat := shortHeartbeat()
	client := newClient(41, "ticker", connection, hub, UnavailableInboundHandler{}, 2, heartbeat, nil)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.writePump()
	select {
	case messageType := <-connection.controls:
		if messageType != websocket.PingMessage {
			t.Fatalf("control frame type = %d", messageType)
		}
	case <-time.After(time.Second):
		t.Fatal("writePump did not emit Ping")
	}
	if connection.writeDeadline.Load() == 0 {
		t.Fatal("write deadline was not set")
	}
	hub.Unregister(context.Background(), client, DisconnectNormal)
	waitChannel(t, client.writeDone, time.Second, "writePump")
	drainControls(connection.controls)
	time.Sleep(3 * heartbeat.PingPeriod)
	if len(connection.controls) != 0 {
		t.Fatal("heartbeat ticker continued after writePump exit")
	}
	if connection.concurrent.Load() {
		t.Fatal("Ping and data writes were concurrent")
	}
}

func TestPeerPingResponseUsesWritePump(t *testing.T) {
	hub, _ := runningHub(t)
	connection := newFakeSocket(8)
	client := newClient(44, "peer-ping", connection, hub, UnavailableInboundHandler{}, 2, shortHeartbeat(), nil)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.readPump(context.Background())
	go client.writePump()
	waitForCondition(t, time.Second, func() bool {
		connection.pongMu.Lock()
		defer connection.pongMu.Unlock()
		return connection.ping != nil && connection.closeHandler != nil
	})
	connection.pongMu.Lock()
	pingHandler := connection.ping
	closeHandler := connection.closeHandler
	connection.pongMu.Unlock()
	if err := pingHandler("peer-heartbeat"); err != nil {
		t.Fatalf("ping handler: %v", err)
	}
	if err := closeHandler(websocket.CloseNormalClosure, "done"); err != nil {
		t.Fatalf("close handler: %v", err)
	}
	deadline := time.After(time.Second)
	for {
		select {
		case messageType := <-connection.controls:
			if messageType == websocket.PongMessage {
				if connection.concurrent.Load() {
					t.Fatal("peer Ping response bypassed single writer")
				}
				hub.Unregister(context.Background(), client, DisconnectNormal)
				waitChannel(t, client.readDone, time.Second, "readPump")
				waitChannel(t, client.writeDone, time.Second, "writePump")
				return
			}
		case <-deadline:
			t.Fatal("writePump did not send Pong response")
		}
	}
}

func TestShutdownStopsBothClientPumps(t *testing.T) {
	hub := NewHub()
	hubContext, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go hub.Run(hubContext)
	observer := newRecordingLifecycleObserver()
	client := newClient(42, "shutdown", newFakeSocket(8), hub, UnavailableInboundHandler{}, 2, shortHeartbeat(), observer)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.readPump(context.Background())
	go client.writePump()
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := hub.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
	waitChannel(t, client.readDone, time.Second, "readPump")
	waitChannel(t, client.writeDone, time.Second, "writePump")
	event := waitLifecycleEvent(t, observer, time.Second)
	if event.reason != DisconnectShutdown {
		t.Fatalf("disconnect reason = %s", event.reason)
	}
	if hub.Unregister(context.Background(), client, DisconnectAbnormal) {
		t.Fatal("duplicate cleanup removed a client after shutdown")
	}
}

func TestSlowClientRemovalStopsWriterAndHeartbeat(t *testing.T) {
	hub, _ := runningHub(t)
	observer := newRecordingLifecycleObserver()
	connection := newFakeSocket(0)
	heartbeat := shortHeartbeat()
	client := newClient(43, "slow-heartbeat", connection, hub, UnavailableInboundHandler{}, 1, heartbeat, observer)
	if err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	go client.writePump()
	if result, err := hub.SendToClient(context.Background(), 43, client.ClientID, []byte("one")); err != nil || result.Delivered != 1 {
		t.Fatalf("first delivery = %#v, %v", result, err)
	}
	waitForCondition(t, time.Second, func() bool { return connection.active.Load() == 1 })
	if result, err := hub.SendToClient(context.Background(), 43, client.ClientID, []byte("two")); err != nil || result.Delivered != 1 {
		t.Fatalf("second delivery = %#v, %v", result, err)
	}
	if result, err := hub.SendToClient(context.Background(), 43, client.ClientID, []byte("three")); err != nil || result.Dropped != 1 {
		t.Fatalf("slow delivery = %#v, %v", result, err)
	}
	waitChannel(t, client.writeDone, time.Second, "slow client writePump")
	event := waitLifecycleEvent(t, observer, time.Second)
	if event.reason != DisconnectSlow {
		t.Fatalf("disconnect reason = %s", event.reason)
	}
	if count := connectionCount(t, hub, 43); count != 0 {
		t.Fatalf("slow client count = %d", count)
	}
}

func shortHeartbeat() HeartbeatConfig {
	return HeartbeatConfig{ReadLimit: 1 << 20, PongWait: 180 * time.Millisecond, PingPeriod: 40 * time.Millisecond, WriteWait: 30 * time.Millisecond}
}

func waitLifecycleEvent(t *testing.T, observer *recordingLifecycleObserver, timeout time.Duration) lifecycleEvent {
	t.Helper()
	select {
	case event := <-observer.disconnected:
		return event
	case <-time.After(timeout):
		t.Fatal("disconnect event timed out")
		return lifecycleEvent{}
	}
}

func waitChannel(t *testing.T, channel <-chan struct{}, timeout time.Duration, name string) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(timeout):
		t.Fatalf("%s did not exit", name)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func drainControls(channel chan int) {
	for {
		select {
		case <-channel:
		default:
			return
		}
	}
}

var _ LifecycleObserver = (*recordingLifecycleObserver)(nil)
