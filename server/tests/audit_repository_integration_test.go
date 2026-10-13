package tests

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
	logsmysql "github.com/kanhai447/GIM/server/internal/logs/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/migrations"
)

func TestAuditRepositoryIdempotencyFiltersAndPrivacy(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run audit repository integration")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	config, err := platformmysql.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	client, err := platformmysql.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	sqlDB, err := client.DB().DB()
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

	repository := logsmysql.New(client.DB())
	metadata, err := audit.Metadata(map[string]any{"message_id": uint64(701), "sender_id": uint64(7), "receiver_id": uint64(8), "message_type": 1})
	if err != nil {
		t.Fatal(err)
	}
	event := audit.NewEvent(audit.EventChatMessageAccepted, "chat", "message.accept", audit.StatusSuccess)
	event.UserID, event.Role, event.ResourceType, event.ResourceID = audit.Uint64(7), audit.Int32(2), "chat_message", "701"
	event.RequestID, event.Method, event.Path, event.Metadata = "audit-repository-"+event.EventID, "GET", "/safe/path", metadata
	t.Cleanup(func() { client.DB().Exec("DELETE FROM audit_logs WHERE event_id = ?", event.EventID) })
	created, err := repository.Insert(context.Background(), event)
	if err != nil || !created {
		t.Fatalf("first insert created=%t err=%v", created, err)
	}
	created, err = repository.Insert(context.Background(), event)
	if err != nil || created {
		t.Fatalf("duplicate insert created=%t err=%v", created, err)
	}

	from, to := event.OccurredAt.Add(-time.Second), event.OccurredAt.Add(time.Second)
	page, err := repository.List(context.Background(), domain.Query{Limit: 10, From: &from, To: &to, Service: "chat", EventType: event.EventType, UserID: event.UserID, Status: audit.StatusSuccess, RequestID: event.RequestID})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("filtered page=%#v err=%v", page, err)
	}
	detail, err := repository.Get(context.Background(), page.Items[0].ID)
	if err != nil || detail.EventID != event.EventID {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	payload, _ := json.Marshal(detail)
	for _, forbidden := range []string{"do-not-log-password", "do-not-log-jwt", "message content", "storage_path", "MYSQL_PASSWORD", "REDIS_PASSWORD"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("persisted log leaked %q: %s", forbidden, payload)
		}
	}
}
