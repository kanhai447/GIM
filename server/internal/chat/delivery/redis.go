package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
)

type Event struct {
	MessageID  uint64          `json:"messageId"`
	ReceiverID uint64          `json:"receiverId"`
	Payload    json.RawMessage `json:"payload"`
}

type Handler func(context.Context, Event)

type Bus struct {
	client           *redisv9.Client
	operationTimeout time.Duration
	logger           *log.Logger
}

func New(client *redisv9.Client, operationTimeout time.Duration, logger *log.Logger) (*Bus, error) {
	if client == nil || operationTimeout <= 0 {
		return nil, errors.New("invalid chat delivery bus configuration")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Bus{client: client, operationTimeout: operationTimeout, logger: logger}, nil
}

func (bus *Bus) Publish(ctx context.Context, event Event) error {
	if event.MessageID == 0 || event.ReceiverID == 0 || len(event.Payload) == 0 {
		return errors.New("invalid chat delivery event")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode chat delivery event: %w", err)
	}
	operationContext, cancel := context.WithTimeout(ctx, bus.operationTimeout)
	defer cancel()
	if err := bus.client.Publish(operationContext, rediskeys.ChatDeliveryChannel(), payload).Err(); err != nil {
		return fmt.Errorf("publish chat delivery event: %w", err)
	}
	return nil
}

func (bus *Bus) Subscribe(ctx context.Context, handler Handler) (*Subscription, error) {
	if handler == nil {
		return nil, errors.New("chat delivery handler is required")
	}
	pubsub := bus.client.Subscribe(ctx, rediskeys.ChatDeliveryChannel())
	readyContext, cancel := context.WithTimeout(ctx, bus.operationTimeout)
	defer cancel()
	if _, err := pubsub.Receive(readyContext); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("subscribe chat delivery channel: %w", err)
	}
	return &Subscription{pubsub: pubsub, handler: handler, logger: bus.logger}, nil
}

type Subscription struct {
	pubsub  *redisv9.PubSub
	handler Handler
	logger  *log.Logger
}

func (subscription *Subscription) Run(ctx context.Context) {
	channel := subscription.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case incoming, ok := <-channel:
			if !ok {
				return
			}
			var event Event
			if err := json.Unmarshal([]byte(incoming.Payload), &event); err != nil || event.MessageID == 0 || event.ReceiverID == 0 || len(event.Payload) == 0 {
				subscription.logger.Printf("chat delivery ignored invalid event")
				continue
			}
			subscription.handler(ctx, event)
		}
	}
}

func (subscription *Subscription) Close() error {
	if subscription == nil || subscription.pubsub == nil {
		return nil
	}
	return subscription.pubsub.Close()
}
