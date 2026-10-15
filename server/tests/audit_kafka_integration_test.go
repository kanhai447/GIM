package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kanhai447/GIM/server/internal/logs/consumer"
	"github.com/kanhai447/GIM/server/internal/logs/domain"
	logsmysql "github.com/kanhai447/GIM/server/internal/logs/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/kafkaaudit"
	"github.com/kanhai447/GIM/server/migrations"
	"github.com/segmentio/kafka-go"
)

func TestKafkaAuditProduceConsumePersistQueryRestartAndIdempotency(t *testing.T) {
	if os.Getenv("GIM_KAFKA_INTEGRATION") != "1" {
		t.Skip("set GIM_KAFKA_INTEGRATION=1 to run real Kafka audit integration")
	}
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Kafka audit integration")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	kafkaConfig, err := kafkaaudit.FromValues(values)
	if err != nil || !kafkaConfig.Enabled {
		t.Fatalf("Kafka configuration: enabled=%t err=%v", kafkaConfig.Enabled, err)
	}
	kafkaConfig.Topic = "gim.audit.events.integration." + strings.ReplaceAll(uuid.NewString(), "-", "")
	kafkaConfig.ConsumerGroup = "gim-logs-integration-" + uuid.NewString()
	kafkaConfig.PublishTimeout = 2 * time.Second
	kafkaConfig.RetryBackoff = 25 * time.Millisecond
	kafkaConfig.MaxAttempts = 3
	ensureAuditTopic(t, kafkaConfig)
	t.Cleanup(func() { deleteAuditTopic(t, kafkaConfig) })

	mysqlConfig, err := platformmysql.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	mysqlClient, err := platformmysql.Open(context.Background(), mysqlConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer mysqlClient.Close()
	sqlDB, err := mysqlClient.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	runner, err := migrations.New(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	repository := logsmysql.New(mysqlClient.DB())
	requestPrefix := "audit-it-" + uuid.NewString()
	t.Cleanup(func() { mysqlClient.DB().Exec("DELETE FROM audit_logs WHERE request_id LIKE ?", requestPrefix+"%") })

	producer, err := kafkaaudit.NewProducer(kafkaConfig, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = producer.Close(ctx)
	}()

	auditConsumer, err := consumer.New(kafkaConfig, repository, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	consumerContext, cancelConsumer := context.WithCancel(context.Background())
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- auditConsumer.Run(consumerContext) }()

	for round := 1; round <= 3; round++ {
		event := integrationAuditEvent(t, requestPrefix+fmt.Sprintf("-round-%d", round), uint64(round))
		if err := producer.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if err := producer.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		page := waitForAuditRows(t, repository, event.RequestID, 1)
		if len(page.Items) != 1 || page.Items[0].EventID != event.EventID {
			t.Fatalf("round %d rows = %#v", round, page.Items)
		}
	}
	for _, event := range integrationBusinessEvents(t, requestPrefix) {
		payload, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		for _, forbidden := range []string{"password", "jwt", "Authorization", "message-content", "storage_path"} {
			if strings.Contains(string(payload), forbidden) {
				t.Fatalf("event %s leaked %q: %s", event.EventType, forbidden, payload)
			}
		}
		if err := producer.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		page := waitForAuditRows(t, repository, event.RequestID, 1)
		if len(page.Items) != 1 || page.Items[0].EventType != event.EventType {
			t.Fatalf("business event rows = %#v", page.Items)
		}
	}

	cancelConsumer()
	_ = auditConsumer.Close()
	select {
	case err := <-consumerDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop")
	}

	recoveryEvent := integrationAuditEvent(t, requestPrefix+"-recovery", 44)
	if err := producer.Publish(context.Background(), recoveryEvent); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)
	restarted, err := consumer.New(kafkaConfig, repository, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	restartContext, cancelRestart := context.WithCancel(context.Background())
	restartDone := make(chan error, 1)
	go func() { restartDone <- restarted.Run(restartContext) }()
	waitForAuditRows(t, repository, recoveryEvent.RequestID, 1)
	cancelRestart()
	_ = restarted.Close()
	select {
	case err := <-restartDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("restarted consumer did not stop")
	}
}

func TestKafkaAuditUnavailableBrokerKeepsPublishBounded(t *testing.T) {
	config := kafkaaudit.DefaultConfig()
	config.Enabled = true
	config.Brokers = []string{"127.0.0.1:1"}
	config.QueueCapacity = 1
	config.PublishTimeout = 100 * time.Millisecond
	config.RetryBackoff = 10 * time.Millisecond
	config.MaxAttempts = 2
	producer, err := kafkaaudit.NewProducer(config, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	event := integrationAuditEvent(t, "audit-unavailable-"+uuid.NewString(), 99)
	started := time.Now()
	if err := producer.Publish(context.Background(), event); err != nil {
		t.Fatalf("first Publish() = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("business-side Publish blocked for %s", elapsed)
	}
	closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := producer.Close(closeContext); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if producer.Dropped() != 1 {
		t.Fatalf("Dropped() = %d, want 1", producer.Dropped())
	}
}

func integrationAuditEvent(t *testing.T, requestID string, messageID uint64) audit.Event {
	t.Helper()
	metadata, err := audit.Metadata(map[string]any{"message_id": messageID, "sender_id": uint64(7), "receiver_id": uint64(8), "message_type": 1})
	if err != nil {
		t.Fatal(err)
	}
	event := audit.NewEvent(audit.EventChatMessageAccepted, "chat", "message.accept", audit.StatusSuccess)
	event.UserID, event.ResourceType, event.ResourceID, event.RequestID, event.Metadata = audit.Uint64(7), "chat_message", fmt.Sprint(messageID), requestID, metadata
	return event
}

func integrationBusinessEvents(t *testing.T, requestPrefix string) []audit.Event {
	t.Helper()
	login := audit.NewEvent(audit.EventAuthLoginSuccess, "auth", "login", audit.StatusSuccess)
	login.UserID, login.Role, login.ResourceType, login.ResourceID, login.RequestID = audit.Uint64(7), audit.Int32(2), "user", "7", requestPrefix+"-login"
	groupMetadata, err := audit.Metadata(map[string]any{"message_id": uint64(801), "sender_id": uint64(7), "group_id": uint64(80), "message_type": 1})
	if err != nil {
		t.Fatal(err)
	}
	group := audit.NewEvent(audit.EventGroupMessageAccepted, "group", "message.accept", audit.StatusSuccess)
	group.UserID, group.ResourceType, group.ResourceID, group.RequestID, group.Metadata = audit.Uint64(7), "group_message", "801", requestPrefix+"-group", groupMetadata
	fileMetadata, err := audit.Metadata(map[string]any{"user_file_id": uint64(91), "file_object_uid": uuid.NewString(), "size_bytes": uint64(12), "mime_type": "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	file := audit.NewEvent(audit.EventFileUploaded, "file", "upload", audit.StatusSuccess)
	file.UserID, file.ResourceType, file.ResourceID, file.RequestID, file.Metadata = audit.Uint64(7), "user_file", "91", requestPrefix+"-file", fileMetadata
	settings := audit.NewEvent(audit.EventAdminSettingsUpdated, "settings", "update", audit.StatusSuccess)
	settings.UserID, settings.Role, settings.ResourceType, settings.ResourceID, settings.RequestID = audit.Uint64(1), audit.Int32(1), "settings", "public", requestPrefix+"-settings"
	userMetadata, err := audit.Metadata(map[string]any{"target_user_id": uint64(7)})
	if err != nil {
		t.Fatal(err)
	}
	disabled := audit.NewEvent(audit.EventAdminUserDisabled, "user", "disable", audit.StatusSuccess)
	disabled.UserID, disabled.Role, disabled.ResourceType, disabled.ResourceID, disabled.RequestID, disabled.Metadata = audit.Uint64(1), audit.Int32(1), "user", "7", requestPrefix+"-user-disabled", userMetadata
	enabled := audit.NewEvent(audit.EventAdminUserEnabled, "user", "enable", audit.StatusSuccess)
	enabled.UserID, enabled.Role, enabled.ResourceType, enabled.ResourceID, enabled.RequestID, enabled.Metadata = audit.Uint64(1), audit.Int32(1), "user", "7", requestPrefix+"-user-enabled", userMetadata
	return []audit.Event{login, group, file, settings, disabled, enabled}
}

func waitForAuditRows(t *testing.T, repository *logsmysql.Repository, requestID string, count int) domain.Page {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		page, err := repository.List(context.Background(), domain.Query{RequestID: requestID, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == count {
			return page
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("audit rows for %s did not reach %d", requestID, count)
	return domain.Page{}
}

func ensureAuditTopic(t *testing.T, config kafkaaudit.Config) {
	t.Helper()
	connection, err := kafka.Dial("tcp", config.Brokers[0])
	if err != nil {
		t.Fatalf("connect Kafka broker: %v", err)
	}
	defer connection.Close()
	controller, err := connection.Controller()
	if err != nil {
		t.Fatalf("resolve Kafka controller: %v", err)
	}
	controllerAddress := fmt.Sprintf("%s:%d", controller.Host, controller.Port)
	controllerConnection, err := kafka.Dial("tcp", controllerAddress)
	if err != nil {
		t.Fatalf("connect Kafka controller: %v", err)
	}
	defer controllerConnection.Close()
	err = controllerConnection.CreateTopics(kafka.TopicConfig{Topic: config.Topic, NumPartitions: 3, ReplicationFactor: 1})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		t.Fatalf("create audit topic: %v", err)
	}
}

func deleteAuditTopic(t *testing.T, config kafkaaudit.Config) {
	t.Helper()
	connection, err := kafka.Dial("tcp", config.Brokers[0])
	if err != nil {
		t.Errorf("connect Kafka broker for topic cleanup: %v", err)
		return
	}
	defer connection.Close()
	controller, err := connection.Controller()
	if err != nil {
		t.Errorf("resolve Kafka controller for topic cleanup: %v", err)
		return
	}
	controllerConnection, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		t.Errorf("connect Kafka controller for topic cleanup: %v", err)
		return
	}
	defer controllerConnection.Close()
	if err := controllerConnection.DeleteTopics(config.Topic); err != nil {
		t.Errorf("delete integration audit topic: %v", err)
	}
}
