package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
)

const controlQueueSize = 4

type InboundHandler interface {
	Handle(context.Context, ClientIdentity, protocol.Envelope) error
	HandleMalformed(context.Context, ClientIdentity) error
}

type socketConnection interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	WriteControl(int, []byte, time.Time) error
	SetReadLimit(int64)
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	SetPongHandler(func(string) error)
	SetPingHandler(func(string) error)
	SetCloseHandler(func(int, string) error)
	Close() error
}

type HeartbeatConfig struct {
	ReadLimit  int64
	PongWait   time.Duration
	PingPeriod time.Duration
	WriteWait  time.Duration
}

func (config HeartbeatConfig) valid() bool {
	return config.ReadLimit > 0 && config.PongWait > 0 && config.PingPeriod > 0 &&
		config.PingPeriod < config.PongWait && config.WriteWait > 0
}

type controlFrame struct {
	kind    int
	payload []byte
}

type Client struct {
	UserID   uint64
	ClientID string

	conn        socketConnection
	send        chan []byte
	control     chan controlFrame
	hub         *Hub
	inbound     InboundHandler
	heartbeat   HeartbeatConfig
	observer    LifecycleObserver
	stopOnce    sync.Once
	connectOnce sync.Once
	readDone    chan struct{}
	writeDone   chan struct{}
}

func newClient(userID uint64, clientID string, conn socketConnection, hub *Hub, inbound InboundHandler, sendBuffer int, heartbeat HeartbeatConfig, observer LifecycleObserver) *Client {
	if observer == nil {
		observer = noopLifecycleObserver{}
	}
	return &Client{
		UserID: userID, ClientID: clientID, conn: conn, hub: hub, inbound: inbound,
		send: make(chan []byte, sendBuffer), control: make(chan controlFrame, controlQueueSize),
		heartbeat: heartbeat, observer: observer, readDone: make(chan struct{}), writeDone: make(chan struct{}),
	}
}

func newClientID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (client *Client) readPump(ctx context.Context) {
	reason := DisconnectAbnormal
	defer close(client.readDone)
	defer func() { client.hub.requestUnregister(client, reason) }()
	client.conn.SetReadLimit(client.heartbeat.ReadLimit)
	if err := client.refreshReadDeadline(); err != nil {
		return
	}
	client.conn.SetPongHandler(func(string) error { return client.refreshReadDeadline() })
	client.conn.SetPingHandler(func(payload string) error {
		select {
		case client.control <- controlFrame{kind: websocket.PongMessage, payload: []byte(payload)}:
			return nil
		default:
			return errors.New("group websocket control queue is full")
		}
	})
	client.conn.SetCloseHandler(func(int, string) error { return nil })
	identity := ClientIdentity{UserID: client.UserID, ClientID: client.ClientID}
	for {
		messageType, payload, err := client.conn.ReadMessage()
		if err != nil {
			reason = classifyReadFailure(err)
			return
		}
		if messageType != websocket.TextMessage {
			return
		}
		envelope, err := protocol.Decode(payload)
		if err != nil {
			if client.inbound.HandleMalformed(ctx, identity) != nil {
				return
			}
			continue
		}
		if client.inbound.Handle(ctx, identity, envelope) != nil {
			return
		}
	}
}

// writePump is the sole owner of all application initiated writes to conn.
func (client *Client) writePump() {
	reason := DisconnectAbnormal
	ticker := time.NewTicker(client.heartbeat.PingPeriod)
	defer ticker.Stop()
	defer close(client.writeDone)
	defer func() { client.hub.requestUnregister(client, reason) }()
	for {
		select {
		case payload, ok := <-client.send:
			if !ok {
				return
			}
			if err := client.conn.SetWriteDeadline(time.Now().Add(client.heartbeat.WriteWait)); err != nil {
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case tick := <-ticker.C:
			deadline := tick.Add(client.heartbeat.WriteWait)
			if err := client.conn.SetWriteDeadline(deadline); err != nil {
				return
			}
			if err := client.conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				return
			}
		case frame := <-client.control:
			deadline := time.Now().Add(client.heartbeat.WriteWait)
			if err := client.conn.SetWriteDeadline(deadline); err != nil {
				return
			}
			if err := client.conn.WriteControl(frame.kind, frame.payload, deadline); err != nil {
				return
			}
		}
	}
}

func (client *Client) refreshReadDeadline() error {
	return client.conn.SetReadDeadline(time.Now().Add(client.heartbeat.PongWait))
}

func (client *Client) connected() {
	client.connectOnce.Do(func() {
		client.observer.Connected(ClientIdentity{UserID: client.UserID, ClientID: client.ClientID})
	})
}

func (client *Client) stop(reason DisconnectReason) {
	client.stopOnce.Do(func() {
		close(client.send)
		_ = client.conn.Close()
		client.observer.Disconnected(ClientIdentity{UserID: client.UserID, ClientID: client.ClientID}, reason)
	})
}

func classifyReadFailure(err error) DisconnectReason {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return DisconnectNormal
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return DisconnectTimeout
	}
	return DisconnectAbnormal
}
