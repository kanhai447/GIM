package chat

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

var (
	ErrHubClosed       = errors.New("chat hub is closed")
	ErrDuplicateClient = errors.New("chat client already registered")
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
	result chan bool
}

type countCommand struct {
	userID uint64
	result chan int
}

type routeCommand struct {
	userID   uint64
	clientID string
	payload  []byte
	result   chan DeliveryResult
}

func NewHub() *Hub {
	return &Hub{commands: make(chan any, 64), stop: make(chan struct{}), done: make(chan struct{})}
}

func (hub *Hub) Run(ctx context.Context) {
	hub.runOnce.Do(func() { hub.run(ctx) })
}

func (hub *Hub) run(ctx context.Context) {
	clients := make(map[uint64]map[string]*Client)
	defer func() {
		for _, userClients := range clients {
			for _, client := range userClients {
				client.stop()
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
				current.result <- registerClient(clients, current.client)
			case unregisterCommand:
				current.result <- removeClient(clients, current.client)
			case countCommand:
				current.result <- len(clients[current.userID])
			case routeCommand:
				current.result <- routeMessage(clients, current)
			}
		}
	}
}

func (hub *Hub) Register(ctx context.Context, client *Client) error {
	if client == nil || client.hub != hub || client.UserID == 0 || client.ClientID == "" {
		return errors.New("invalid chat client")
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

func (hub *Hub) Unregister(ctx context.Context, client *Client) bool {
	if client == nil {
		return false
	}
	result := make(chan bool, 1)
	if err := hub.submit(ctx, unregisterCommand{client: client, result: result}); err != nil {
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

func (hub *Hub) UserConnectionCount(ctx context.Context, userID uint64) (int, error) {
	result := make(chan int, 1)
	if err := hub.submit(ctx, countCommand{userID: userID, result: result}); err != nil {
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

func (hub *Hub) SendToClient(ctx context.Context, userID uint64, clientID string, payload []byte) (DeliveryResult, error) {
	return hub.route(ctx, userID, clientID, payload)
}

func (hub *Hub) SendToUser(ctx context.Context, userID uint64, payload []byte) (DeliveryResult, error) {
	return hub.route(ctx, userID, "", payload)
}

func (hub *Hub) route(ctx context.Context, userID uint64, clientID string, payload []byte) (DeliveryResult, error) {
	if userID == 0 || len(payload) == 0 {
		return DeliveryResult{}, errors.New("invalid chat delivery")
	}
	result := make(chan DeliveryResult, 1)
	command := routeCommand{userID: userID, clientID: clientID, payload: bytes.Clone(payload), result: result}
	if err := hub.submit(ctx, command); err != nil {
		return DeliveryResult{}, err
	}
	select {
	case delivery := <-result:
		return delivery, nil
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

func registerClient(clients map[uint64]map[string]*Client, client *Client) error {
	userClients := clients[client.UserID]
	if userClients == nil {
		userClients = make(map[string]*Client)
		clients[client.UserID] = userClients
	}
	if _, exists := userClients[client.ClientID]; exists {
		return ErrDuplicateClient
	}
	userClients[client.ClientID] = client
	return nil
}

func removeClient(clients map[uint64]map[string]*Client, client *Client) bool {
	userClients := clients[client.UserID]
	if userClients == nil || userClients[client.ClientID] != client {
		return false
	}
	delete(userClients, client.ClientID)
	if len(userClients) == 0 {
		delete(clients, client.UserID)
	}
	client.stop()
	return true
}

func routeMessage(clients map[uint64]map[string]*Client, command routeCommand) DeliveryResult {
	var result DeliveryResult
	userClients := clients[command.userID]
	for id, client := range userClients {
		if command.clientID != "" && id != command.clientID {
			continue
		}
		select {
		case client.send <- command.payload:
			result.Delivered++
		default:
			result.Dropped++
			removeClient(clients, client)
		}
	}
	return result
}
