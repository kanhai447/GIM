package kafkaaudit

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/segmentio/kafka-go"
)

type fakeWriter struct {
	mu       sync.Mutex
	messages []kafka.Message
	err      error
	blocked  chan struct{}
}

func (writer *fakeWriter) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	if writer.blocked != nil {
		select {
		case <-writer.blocked:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	writer.mu.Lock()
	writer.messages = append(writer.messages, messages...)
	writer.mu.Unlock()
	return writer.err
}
func (*fakeWriter) Close() error { return nil }

func TestProducerPublishesValidatedEvent(t *testing.T) {
	config := DefaultConfig()
	config.Enabled, config.QueueCapacity = true, 4
	writer := &fakeWriter{}
	producer := newProducer(config, writer, log.New(io.Discard, "", 0))
	event := audit.NewEvent(audit.EventAuthLogout, "auth", "logout", audit.StatusSuccess)
	if err := producer.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := producer.Close(ctx); err != nil {
		t.Fatal(err)
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.messages) != 1 || string(writer.messages[0].Key) != event.EventType {
		t.Fatalf("messages = %#v", writer.messages)
	}
}

func TestProducerQueueIsBounded(t *testing.T) {
	config := DefaultConfig()
	config.Enabled, config.QueueCapacity, config.PublishTimeout, config.MaxAttempts = true, 1, 10*time.Millisecond, 1
	writer := &fakeWriter{blocked: make(chan struct{})}
	producer := newProducer(config, writer, log.New(io.Discard, "", 0))
	first := audit.NewEvent(audit.EventAuthLogout, "auth", "logout", audit.StatusSuccess)
	second := audit.NewEvent(audit.EventAuthLogout, "auth", "logout", audit.StatusSuccess)
	third := audit.NewEvent(audit.EventAuthLogout, "auth", "logout", audit.StatusSuccess)
	if err := producer.Publish(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for len(producer.queue) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := producer.Publish(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := producer.Publish(context.Background(), third); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("third publish error = %v", err)
	}
	close(writer.blocked)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = producer.Close(ctx)
}

func TestUnavailableWriterDoesNotBlockBusinessPublish(t *testing.T) {
	config := DefaultConfig()
	config.Enabled, config.QueueCapacity, config.PublishTimeout, config.MaxAttempts = true, 2, 5*time.Millisecond, 1
	writer := &fakeWriter{err: errors.New("broker unavailable")}
	producer := newProducer(config, writer, log.New(io.Discard, "", 0))
	event := audit.NewEvent(audit.EventAuthLoginSuccess, "auth", "login", audit.StatusSuccess)
	started := time.Now()
	if err := producer.Publish(context.Background(), event); err != nil {
		t.Fatalf("business-side publish returned error: %v", err)
	}
	if time.Since(started) > 20*time.Millisecond {
		t.Fatal("business-side publish blocked on unavailable writer")
	}
	deadline := time.Now().Add(time.Second)
	for producer.Dropped() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if producer.Dropped() != 1 {
		t.Fatalf("dropped = %d", producer.Dropped())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := producer.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
