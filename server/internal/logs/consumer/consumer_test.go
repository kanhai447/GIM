package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
	"github.com/kanhai447/GIM/server/internal/logs/repository"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/segmentio/kafka-go"
)

type fakeReader struct {
	messages  chan kafka.Message
	mu        sync.Mutex
	committed []int64
	fetchErr  error
	commitErr error
}

func (reader *fakeReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	reader.mu.Lock()
	fetchErr := reader.fetchErr
	reader.fetchErr = nil
	reader.mu.Unlock()
	if fetchErr != nil {
		return kafka.Message{}, fetchErr
	}
	select {
	case message := <-reader.messages:
		return message, nil
	case <-ctx.Done():
		return kafka.Message{}, ctx.Err()
	}
}
func (reader *fakeReader) CommitMessages(_ context.Context, messages ...kafka.Message) error {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if reader.commitErr != nil {
		err := reader.commitErr
		reader.commitErr = nil
		return err
	}
	for _, message := range messages {
		reader.committed = append(reader.committed, message.Offset)
	}
	return nil
}

func TestConsumerDiscardsMalformedPayloadAndRetriesKafkaOperations(t *testing.T) {
	reader := &fakeReader{
		messages:  make(chan kafka.Message, 1),
		fetchErr:  errors.New("broker unavailable"),
		commitErr: errors.New("coordinator unavailable"),
	}
	reader.messages <- kafka.Message{Value: []byte(`{"password":"must-not-be-logged"}`), Offset: 12}
	consumer := newConsumer(reader, &fakeRepository{}, log.New(io.Discard, "", 0), time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		reader.mu.Lock()
		committed := len(reader.committed)
		reader.mu.Unlock()
		if committed == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if len(reader.committed) != 1 || reader.committed[0] != 12 {
		t.Fatalf("commits = %v", reader.committed)
	}
}
func (*fakeReader) Close() error { return nil }

type fakeRepository struct {
	mu       sync.Mutex
	attempts int
	events   []audit.Event
	failOnce bool
}

func (repo *fakeRepository) Insert(_ context.Context, event audit.Event) (bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.attempts++
	if repo.failOnce {
		repo.failOnce = false
		return false, errors.New("db unavailable")
	}
	repo.events = append(repo.events, event)
	return true, nil
}
func (*fakeRepository) List(context.Context, domain.Query) (domain.Page, error) {
	return domain.Page{}, nil
}
func (*fakeRepository) Get(context.Context, uint64) (domain.Log, error) {
	return domain.Log{}, repository.ErrNotFound
}

func TestConsumerPersistsBeforeCommitAndRetriesDatabase(t *testing.T) {
	event := audit.NewEvent(audit.EventAuthLogout, "auth", "logout", audit.StatusSuccess)
	payload, _ := json.Marshal(event)
	reader := &fakeReader{messages: make(chan kafka.Message, 1)}
	reader.messages <- kafka.Message{Value: payload, Offset: 9}
	repo := &fakeRepository{failOnce: true}
	consumer := newConsumer(reader, repo, log.New(io.Discard, "", 0), time.Millisecond, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		reader.mu.Lock()
		committed := len(reader.committed)
		reader.mu.Unlock()
		if committed == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.attempts != 2 || len(repo.events) != 1 {
		t.Fatalf("attempts/events = %d/%d", repo.attempts, len(repo.events))
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if len(reader.committed) != 1 || reader.committed[0] != 9 {
		t.Fatalf("commits = %v", reader.committed)
	}
}
