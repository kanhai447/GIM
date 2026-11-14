package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/file/repository/mysql"
	fileservice "github.com/kanhai447/GIM/server/internal/file/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

// A text message may mention a fileID in its content, but only imageMsg.fileID
// is an authorization reference. Text rendering also never calls the download API.
func TestTextMessageDoesNotGrantImageDownload(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run file download authorization tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	database, rawDB := newChatMessageDatabase(t, values)
	service, err := fileservice.New(mysql.New(database), unusedStorage{}, fileservice.Config{MaxUploadBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 30_000)
	owner, peer := seed, seed+1
	seedChatUsers(t, rawDB, owner, peer)
	fileID := seedIntegrationFile(t, rawDB, owner, "image/png", 64, "text-only.png")
	if _, err := rawDB.Exec(`INSERT INTO chat_messages (send_user_id, rev_user_id, client_msg_id, msg_type, msg_preview, msg)
		VALUES (?, ?, 'auth-text-only', 1, ?, ?)`, owner, peer, fmt.Sprintf("see file %d", fileID),
		fmt.Sprintf(`{"type":1,"textMsg":{"content":"see file %d"}}`, fileID)); err != nil {
		t.Fatalf("seed text message: %v", err)
	}

	_, err = service.GetAccessible(context.Background(), peer, fmt.Sprintf("%d", fileID))
	_, code, _ := apperror.PublicFields(err)
	if err == nil || code != 2503 {
		t.Fatalf("text-only peer access = %v (code %d), want 2503", err, code)
	}
	if _, err := service.GetAccessible(context.Background(), owner, fmt.Sprintf("%d", fileID)); err != nil {
		t.Fatalf("owner access to text-referenced file: %v", err)
	}
}
