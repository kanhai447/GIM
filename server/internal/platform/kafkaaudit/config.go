// Package kafkaaudit provides the shared Kafka transport for audit events.
package kafkaaudit

import (
	"errors"
	"strconv"
	"strings"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const DefaultTopic = "gim.audit.events"

type Config struct {
	Enabled        bool
	Brokers        []string
	Topic          string
	ConsumerGroup  string
	QueueCapacity  int
	PublishTimeout time.Duration
	RetryBackoff   time.Duration
	MaxAttempts    int
}

func DefaultConfig() Config {
	return Config{
		Topic: DefaultTopic, ConsumerGroup: "gim-logs-v1", QueueCapacity: 1024,
		PublishTimeout: 750 * time.Millisecond, RetryBackoff: 100 * time.Millisecond, MaxAttempts: 3,
	}
}

// FromValues keeps Kafka optional for existing service/unit-test configs. A
// non-empty KAFKA_BROKERS value enables the producer/consumer and then all
// remaining settings are validated.
func FromValues(values platformconfig.Values) (Config, error) {
	config := DefaultConfig()
	rawBrokers, configured := values.Lookup("KAFKA_BROKERS")
	if !configured || strings.TrimSpace(rawBrokers) == "" {
		return config, nil
	}
	for _, broker := range strings.Split(rawBrokers, ",") {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			config.Brokers = append(config.Brokers, broker)
		}
	}
	if len(config.Brokers) == 0 {
		return Config{}, errors.New("KAFKA_BROKERS must contain at least one broker")
	}
	config.Enabled = true
	if value, ok := values.Lookup("KAFKA_AUDIT_TOPIC"); ok && strings.TrimSpace(value) != "" {
		config.Topic = strings.TrimSpace(value)
	} else if value, ok := values.Lookup("KAFKA_LOG_TOPIC"); ok && strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "CHANGE_ME" {
		// Temporary local-config compatibility; committed examples and all new
		// deployments use KAFKA_AUDIT_TOPIC.
		config.Topic = strings.TrimSpace(value)
	}
	if value, ok := values.Lookup("KAFKA_AUDIT_CONSUMER_GROUP"); ok && strings.TrimSpace(value) != "" {
		config.ConsumerGroup = strings.TrimSpace(value)
	}
	var err error
	if config.QueueCapacity, err = optionalPositiveInt(values, "KAFKA_AUDIT_QUEUE_CAPACITY", config.QueueCapacity); err != nil {
		return Config{}, err
	}
	if config.MaxAttempts, err = optionalPositiveInt(values, "KAFKA_AUDIT_MAX_ATTEMPTS", config.MaxAttempts); err != nil {
		return Config{}, err
	}
	if config.PublishTimeout, err = optionalPositiveDuration(values, "KAFKA_AUDIT_PUBLISH_TIMEOUT", config.PublishTimeout); err != nil {
		return Config{}, err
	}
	if config.RetryBackoff, err = optionalPositiveDuration(values, "KAFKA_AUDIT_RETRY_BACKOFF", config.RetryBackoff); err != nil {
		return Config{}, err
	}
	if config.Topic == "" || config.ConsumerGroup == "" {
		return Config{}, errors.New("Kafka audit topic and consumer group are required")
	}
	return config, nil
}

func optionalPositiveInt(values platformconfig.Values, key string, fallback int) (int, error) {
	value, ok := values.Lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("configuration key " + key + " must be a positive integer")
	}
	return parsed, nil
}

func optionalPositiveDuration(values platformconfig.Values, key string, fallback time.Duration) (time.Duration, error) {
	value, ok := values.Lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("configuration key " + key + " must be a positive duration")
	}
	return parsed, nil
}
