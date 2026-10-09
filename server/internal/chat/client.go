package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

var ErrMessageHandlingUnavailable = errors.New("chat message handling is not available in this checkpoint")

type ClientIdentity struct {
	UserID   uint64
	ClientID string
}

type InboundHandler interface {
	Handle(context.Context, ClientIdentity, protocol.Envelope) error
}

type InboundHandlerFunc func(context.Context, ClientIdentity, protocol.Envelope) error

func (function InboundHandlerFunc) Handle(ctx context.Context, identity ClientIdentity, envelope protocol.Envelope) error {
	return function(ctx, identity, envelope)
}

type UnavailableInboundHandler struct{}

func (UnavailableInboundHandler) Handle(context.Context, ClientIdentity, protocol.Envelope) error {
	return ErrMessageHandlingUnavailable
}

type socketConnection interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	Close() error
}

type Client struct {
	UserID   uint64
	ClientID string

	conn     socketConnection
	send     chan []byte
	hub      *Hub
	inbound  InboundHandler
	stopOnce sync.Once
}

func newClient(userID uint64, clientID string, connection socketConnection, hub *Hub, inbound InboundHandler, sendBuffer int) *Client {
	return &Client{
		UserID: userID, ClientID: clientID, conn: connection, send: make(chan []byte, sendBuffer), hub: hub, inbound: inbound,
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
	defer client.hub.Unregister(context.Background(), client)
	for {
		messageType, payload, err := client.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			return
		}
		envelope, err := protocol.Decode(payload)
		if err != nil || client.inbound.Handle(ctx, ClientIdentity{UserID: client.UserID, ClientID: client.ClientID}, envelope) != nil {
			return
		}
	}
}

func (client *Client) writePump() {
	defer client.hub.Unregister(context.Background(), client)
	for payload := range client.send {
		if err := client.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func (client *Client) stop() {
	client.stopOnce.Do(func() {
		close(client.send)
		_ = client.conn.Close()
	})
}
