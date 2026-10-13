package kafkaaudit

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/segmentio/kafka-go"
)

var (
	ErrQueueFull = errors.New("audit producer queue is full")
	ErrClosed    = errors.New("audit producer is closed")
)

type writer interface {
	WriteMessages(context.Context, ...kafka.Message) error
	Close() error
}

type Producer struct {
	config    Config
	writer    writer
	logger    *log.Logger
	queue     chan audit.Event
	root      context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	queueMu   sync.RWMutex
	closed    atomic.Bool
	dropped   atomic.Uint64
}

func NewProducer(config Config, logger *log.Logger) (*Producer, error) {
	if logger == nil {
		logger = log.Default()
	}
	if !config.Enabled {
		return &Producer{config: config, logger: logger}, nil
	}
	if len(config.Brokers) == 0 || config.Topic == "" || config.QueueCapacity <= 0 || config.PublishTimeout <= 0 || config.MaxAttempts <= 0 {
		return nil, errors.New("invalid Kafka audit producer configuration")
	}
	w := &kafka.Writer{
		Addr:                   kafka.TCP(config.Brokers...),
		Topic:                  config.Topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
		BatchSize:              1,
		WriteTimeout:           config.PublishTimeout,
		MaxAttempts:            1,
		AllowAutoTopicCreation: false,
	}
	return newProducer(config, w, logger), nil
}

func newProducer(config Config, w writer, logger *log.Logger) *Producer {
	root, cancel := context.WithCancel(context.Background())
	producer := &Producer{
		config: config, writer: w, logger: logger, queue: make(chan audit.Event, config.QueueCapacity),
		root: root, cancel: cancel, done: make(chan struct{}),
	}
	go producer.run()
	return producer
}

// Publish validates and enqueues without blocking business execution. Queue
// exhaustion is an explicit V1 drop/degradation boundary.
func (producer *Producer) Publish(ctx context.Context, event audit.Event) error {
	if !producer.config.Enabled {
		return nil
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if producer.closed.Load() {
		return ErrClosed
	}
	producer.queueMu.RLock()
	defer producer.queueMu.RUnlock()
	if producer.closed.Load() {
		return ErrClosed
	}
	select {
	case producer.queue <- event:
		return nil
	default:
		producer.dropped.Add(1)
		return ErrQueueFull
	}
}

func (producer *Producer) Dropped() uint64 { return producer.dropped.Load() }

func (producer *Producer) run() {
	defer close(producer.done)
	for {
		select {
		case <-producer.root.Done():
			return
		case event, ok := <-producer.queue:
			if !ok {
				return
			}
			producer.write(event)
		}
	}
}

func (producer *Producer) write(event audit.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		producer.dropped.Add(1)
		producer.logger.Printf("audit event encode failed event_id=%s", event.EventID)
		return
	}
	message := kafka.Message{Key: []byte(audit.PartitionKey(event)), Value: payload, Time: event.OccurredAt.UTC()}
	for attempt := 1; attempt <= producer.config.MaxAttempts; attempt++ {
		writeContext, cancel := context.WithTimeout(producer.root, producer.config.PublishTimeout)
		err = producer.writer.WriteMessages(writeContext, message)
		cancel()
		if err == nil {
			return
		}
		if producer.root.Err() != nil {
			break
		}
		if attempt < producer.config.MaxAttempts {
			timer := time.NewTimer(producer.config.RetryBackoff)
			select {
			case <-producer.root.Done():
				timer.Stop()
				attempt = producer.config.MaxAttempts
			case <-timer.C:
			}
		}
	}
	producer.dropped.Add(1)
	producer.logger.Printf("audit delivery degraded event_id=%s event_type=%s", event.EventID, event.EventType)
}

func (producer *Producer) Close(ctx context.Context) error {
	if !producer.config.Enabled {
		return nil
	}
	producer.closeOnce.Do(func() {
		producer.queueMu.Lock()
		defer producer.queueMu.Unlock()
		producer.closed.Store(true)
		close(producer.queue)
	})
	select {
	case <-producer.done:
	case <-ctx.Done():
		producer.cancel()
		<-producer.done
		_ = producer.writer.Close()
		return ctx.Err()
	}
	producer.cancel()
	return producer.writer.Close()
}
