package ws

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

var (
	ErrHubClosed       = errors.New("group websocket hub is closed")
	ErrDuplicateClient = errors.New("group websocket client already registered")
)

type DeliveryResult struct {
	Delivered int
	Dropped   int
}

type Hub struct {
	commands chan any
	stop     chan struct{}
	done     chan struct{}
	runOnce  sync.Once
	stopOnce sync.Once
}

type registerCommand struct {
	client *Client
	result chan error
}

type unregisterCommand struct {
	client *Client
	reason DisconnectReason
	result chan bool
}

type userCountCommand struct {
	userID uint64
	result chan int
}

type totalCountCommand struct{ result chan int }

type deliverCommand struct {
	userID   uint64
	clientID string
	payload  []byte
	result   chan DeliveryResult
}

func NewHub() *Hub {
	return &Hub{commands: make(chan any, 128), stop: make(chan struct{}), done: make(chan struct{})}
}

func (hub *Hub) Run(ctx context.Context) {
	hub.runOnce.Do(func() { hub.eventLoop(ctx) })
}

func (hub *Hub) eventLoop(ctx context.Context) {
	clients := make(map[uint64]map[string]*Client)
	defer func() {
		for _, devices := range clients {
			for _, client := range devices {
				client.stop(DisconnectShutdown)
			}
		}
		close(hub.done)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-hub.stop:
			return
		case command := <-hub.commands:
			switch current := command.(type) {
			case registerCommand:
				current.result <- addClient(clients, current.client)
			case unregisterCommand:
				current.result <- dropClient(clients, current.client, current.reason)
			case userCountCommand:
				current.result <- len(clients[current.userID])
			case totalCountCommand:
				current.result <- countClients(clients)
			case deliverCommand:
				current.result <- deliver(clients, current)
			}
		}
	}
}

func (hub *Hub) Register(ctx context.Context, client *Client) error {
	if client == nil || client.hub != hub || client.UserID == 0 || client.ClientID == "" {
		return errors.New("invalid group websocket client")
	}
	result := make(chan error, 1)
	if err := hub.submit(ctx, registerCommand{client: client, result: result}); err != nil {
		return err
	}
	select {
	case err := <-result:
		return err
	case <-hub.done:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (hub *Hub) Unregister(ctx context.Context, client *Client, reason DisconnectReason) bool {
	if client == nil {
		return false
	}
	result := make(chan bool, 1)
	if err := hub.submit(ctx, unregisterCommand{client: client, reason: reason, result: result}); err != nil {
		return false
	}
	select {
	case removed := <-result:
		return removed
	case <-hub.done:
		return false
	case <-ctx.Done():
		return false
	}
}

// requestUnregister is used by pumps after the request context may already be
// cancelled. It never owns or closes Client resources; the Hub event loop does.
func (hub *Hub) requestUnregister(client *Client, reason DisconnectReason) {
	result := make(chan bool, 1)
	select {
	case hub.commands <- unregisterCommand{client: client, reason: reason, result: result}:
	case <-hub.done:
	case <-hub.stop:
	}
}

func (hub *Hub) UserConnectionCount(ctx context.Context, userID uint64) (int, error) {
	result := make(chan int, 1)
	if err := hub.submit(ctx, userCountCommand{userID: userID, result: result}); err != nil {
		return 0, err
	}
	select {
	case count := <-result:
		return count, nil
	case <-hub.done:
		return 0, ErrHubClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (hub *Hub) TotalConnectionCount(ctx context.Context) (int, error) {
	result := make(chan int, 1)
	if err := hub.submit(ctx, totalCountCommand{result: result}); err != nil {
		return 0, err
	}
	select {
	case count := <-result:
		return count, nil
	case <-hub.done:
		return 0, ErrHubClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (hub *Hub) SendToUser(ctx context.Context, userID uint64, payload []byte) (DeliveryResult, error) {
	return hub.route(ctx, userID, "", payload)
}

func (hub *Hub) SendToClient(ctx context.Context, identity ClientIdentity, payload []byte) (DeliveryResult, error) {
	return hub.route(ctx, identity.UserID, identity.ClientID, payload)
}

func (hub *Hub) route(ctx context.Context, userID uint64, clientID string, payload []byte) (DeliveryResult, error) {
	if userID == 0 || len(payload) == 0 {
		return DeliveryResult{}, errors.New("invalid group websocket delivery")
	}
	result := make(chan DeliveryResult, 1)
	command := deliverCommand{userID: userID, clientID: clientID, payload: bytes.Clone(payload), result: result}
	if err := hub.submit(ctx, command); err != nil {
		return DeliveryResult{}, err
	}
	select {
	case delivered := <-result:
		return delivered, nil
	case <-hub.done:
		return DeliveryResult{}, ErrHubClosed
	case <-ctx.Done():
		return DeliveryResult{}, ctx.Err()
	}
}

func (hub *Hub) Shutdown(ctx context.Context) error {
	hub.stopOnce.Do(func() { close(hub.stop) })
	select {
	case <-hub.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (hub *Hub) submit(ctx context.Context, command any) error {
	select {
	case <-hub.done:
		return ErrHubClosed
	case <-hub.stop:
		return ErrHubClosed
	default:
	}
	select {
	case hub.commands <- command:
		return nil
	case <-hub.done:
		return ErrHubClosed
	case <-hub.stop:
		return ErrHubClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func addClient(clients map[uint64]map[string]*Client, client *Client) error {
	devices := clients[client.UserID]
	if devices == nil {
		devices = make(map[string]*Client)
		clients[client.UserID] = devices
	}
	if _, exists := devices[client.ClientID]; exists {
		return ErrDuplicateClient
	}
	devices[client.ClientID] = client
	client.connected()
	return nil
}

func dropClient(clients map[uint64]map[string]*Client, client *Client, reason DisconnectReason) bool {
	devices := clients[client.UserID]
	if devices == nil || devices[client.ClientID] != client {
		return false
	}
	delete(devices, client.ClientID)
	if len(devices) == 0 {
		delete(clients, client.UserID)
	}
	client.stop(reason)
	return true
}

func deliver(clients map[uint64]map[string]*Client, command deliverCommand) DeliveryResult {
	var result DeliveryResult
	for id, client := range clients[command.userID] {
		if command.clientID != "" && id != command.clientID {
			continue
		}
		select {
		case client.send <- command.payload:
			result.Delivered++
		default:
			result.Dropped++
			dropClient(clients, client, DisconnectSlow)
		}
	}
	return result
}

func countClients(clients map[uint64]map[string]*Client) int {
	total := 0
	for _, devices := range clients {
		total += len(devices)
	}
	return total
}
