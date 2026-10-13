// Package consumer validates Kafka audit events and persists them before
// committing offsets.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/kanhai447/GIM/server/internal/logs/repository"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/kafkaaudit"
	"github.com/segmentio/kafka-go"
)

type reader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

type Consumer struct {
	reader        reader
	repository    repository.Repository
	logger        *log.Logger
	retryBackoff  time.Duration
	commitTimeout time.Duration
}

func New(config kafkaaudit.Config, repo repository.Repository, logger *log.Logger) (*Consumer, error) {
	if !config.Enabled {
		return nil, nil
	}
	if repo == nil || len(config.Brokers) == 0 || config.Topic == "" || config.ConsumerGroup == "" {
		return nil, errors.New("invalid audit consumer configuration")
	}
	if logger == nil {
		logger = log.Default()
	}
	value := kafka.NewReader(kafka.ReaderConfig{
		Brokers: config.Brokers, Topic: config.Topic, GroupID: config.ConsumerGroup,
		MinBytes: 1, MaxBytes: 10 << 20, MaxWait: 500 * time.Millisecond,
		CommitInterval: 0, StartOffset: kafka.FirstOffset,
	})
	return newConsumer(value, repo, logger, config.RetryBackoff, 5*time.Second), nil
}

func newConsumer(value reader, repo repository.Repository, logger *log.Logger, retryBackoff, commitTimeout time.Duration) *Consumer {
	return &Consumer{reader: value, repository: repo, logger: logger, retryBackoff: retryBackoff, commitTimeout: commitTimeout}
}

func (consumer *Consumer) Run(ctx context.Context) error {
	for {
		message, err := consumer.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			consumer.logger.Printf("audit consumer fetch retry")
			if !consumer.waitForRetry(ctx) {
				return nil
			}
			continue
		}
		var event audit.Event
		if err := json.Unmarshal(message.Value, &event); err != nil || event.Validate() != nil {
			consumer.logger.Printf("audit consumer discarded malformed payload partition=%d offset=%d", message.Partition, message.Offset)
			if !consumer.commitWithRetry(ctx, message) {
				return nil
			}
			continue
		}

		for {
			_, err = consumer.repository.Insert(ctx, event)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return nil
			}
			consumer.logger.Printf("audit persistence retry event_id=%s", event.EventID)
			if !consumer.waitForRetry(ctx) {
				return nil
			}
		}
		if !consumer.commitWithRetry(ctx, message) {
			return nil
		}
	}
}

func (consumer *Consumer) commitWithRetry(ctx context.Context, message kafka.Message) bool {
	for {
		if err := consumer.commit(ctx, message); err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		consumer.logger.Printf("audit consumer commit retry partition=%d offset=%d", message.Partition, message.Offset)
		if !consumer.waitForRetry(ctx) {
			return false
		}
	}
}

func (consumer *Consumer) waitForRetry(ctx context.Context) bool {
	timer := time.NewTimer(consumer.retryBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (consumer *Consumer) commit(ctx context.Context, message kafka.Message) error {
	commitContext, cancel := context.WithTimeout(ctx, consumer.commitTimeout)
	defer cancel()
	return consumer.reader.CommitMessages(commitContext, message)
}

func (consumer *Consumer) Close() error {
	if consumer == nil || consumer.reader == nil {
		return nil
	}
	return consumer.reader.Close()
}
