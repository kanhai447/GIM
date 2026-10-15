package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type wsClient struct {
	ctx       context.Context
	endpoint  string
	origin    string
	onMessage func([]byte)
	onConnect func(time.Duration, bool)
	onDrop    func()

	connectionMu sync.RWMutex
	connection   *websocket.Conn
	writeMu      sync.Mutex
	closed       atomic.Bool
	done         chan struct{}
}

func newWSClient(ctx context.Context, baseURL, path, token, origin string, onMessage func([]byte), onConnect func(time.Duration, bool), onDrop func()) (*wsClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	scheme := "ws"
	if parsed.Scheme == "https" {
		scheme = "wss"
	}
	query := url.Values{"token": []string{token}}
	client := &wsClient{
		ctx: ctx, endpoint: (&url.URL{Scheme: scheme, Host: parsed.Host, Path: path, RawQuery: query.Encode()}).String(),
		origin: origin, onMessage: onMessage, onConnect: onConnect, onDrop: onDrop, done: make(chan struct{}),
	}
	if err := client.connect(false); err != nil {
		return nil, err
	}
	go client.readLoop()
	return client, nil
}

func (client *wsClient) connect(reconnect bool) error {
	started := time.Now()
	header := http.Header{"Origin": []string{client.origin}}
	connection, response, err := websocket.DefaultDialer.DialContext(client.ctx, client.endpoint, header)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		return err
	}
	client.connectionMu.Lock()
	client.connection = connection
	client.connectionMu.Unlock()
	if client.onConnect != nil {
		client.onConnect(time.Since(started), reconnect)
	}
	return nil
}

func (client *wsClient) readLoop() {
	defer close(client.done)
	for {
		client.connectionMu.RLock()
		connection := client.connection
		client.connectionMu.RUnlock()
		if connection == nil {
			if !client.reconnect() {
				return
			}
			continue
		}
		_, payload, err := connection.ReadMessage()
		if err == nil {
			if client.onMessage != nil {
				client.onMessage(payload)
			}
			continue
		}
		client.connectionMu.Lock()
		if client.connection == connection {
			client.connection = nil
		}
		client.connectionMu.Unlock()
		_ = connection.Close()
		if !client.closed.Load() && client.ctx.Err() == nil && client.onDrop != nil {
			client.onDrop()
		}
	}
}

func (client *wsClient) reconnect() bool {
	backoff := 200 * time.Millisecond
	for !client.closed.Load() && client.ctx.Err() == nil {
		timer := time.NewTimer(backoff)
		select {
		case <-client.ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		if err := client.connect(true); err == nil {
			return true
		}
		if backoff < 3*time.Second {
			backoff *= 2
		}
	}
	return false
}

func (client *wsClient) writeJSON(value any) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	client.connectionMu.RLock()
	connection := client.connection
	client.connectionMu.RUnlock()
	if connection == nil {
		return errors.New("websocket disconnected")
	}
	_ = connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return connection.WriteJSON(value)
}

func (client *wsClient) close() {
	if !client.closed.CompareAndSwap(false, true) {
		return
	}
	client.connectionMu.Lock()
	connection := client.connection
	client.connection = nil
	client.connectionMu.Unlock()
	if connection != nil {
		_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		_ = connection.Close()
	}
	select {
	case <-client.done:
	case <-time.After(3 * time.Second):
	}
}
