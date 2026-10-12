package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
	"log"
	"time"
)

type Event struct {
	MessageID     uint64          `json:"messageId"`
	TargetUserIDs []uint64        `json:"targetUserIds"`
	Payload       json.RawMessage `json:"payload"`
}
type Handler func(context.Context, Event)
type Bus struct {
	client  *redisv9.Client
	timeout time.Duration
	logger  *log.Logger
}

func New(client *redisv9.Client, timeout time.Duration, logger *log.Logger) (*Bus, error) {
	if client == nil || timeout <= 0 {
		return nil, errors.New("invalid group delivery bus configuration")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Bus{client: client, timeout: timeout, logger: logger}, nil
}
func (bus *Bus) Publish(ctx context.Context, event Event) error {
	if event.MessageID == 0 || len(event.TargetUserIDs) == 0 || len(event.Payload) == 0 {
		return errors.New("invalid group delivery event")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode group delivery event: %w", err)
	}
	op, cancel := context.WithTimeout(ctx, bus.timeout)
	defer cancel()
	if err := bus.client.Publish(op, rediskeys.GroupDeliveryChannel(), payload).Err(); err != nil {
		return fmt.Errorf("publish group delivery event: %w", err)
	}
	return nil
}
func (bus *Bus) Subscribe(ctx context.Context, handler Handler) (*Subscription, error) {
	if handler == nil {
		return nil, errors.New("group delivery handler is required")
	}
	pubsub := bus.client.Subscribe(ctx, rediskeys.GroupDeliveryChannel())
	ready, cancel := context.WithTimeout(ctx, bus.timeout)
	defer cancel()
	if _, err := pubsub.Receive(ready); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("subscribe group delivery channel: %w", err)
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
			if err := json.Unmarshal([]byte(incoming.Payload), &event); err != nil || event.MessageID == 0 || len(event.TargetUserIDs) == 0 || len(event.Payload) == 0 {
				subscription.logger.Printf("group delivery ignored invalid event")
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
