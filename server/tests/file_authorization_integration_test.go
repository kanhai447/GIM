package tests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/file/repository/mysql"
	fileservice "github.com/kanhai447/GIM/server/internal/file/service"
	filestorage "github.com/kanhai447/GIM/server/internal/file/storage"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type unusedStorage struct{}

func (unusedStorage) CreateTemp(context.Context) (filestorage.TempObject, error) {
	return nil, errors.New("storage is not used by download authorization")
}
func (unusedStorage) Commit(context.Context, filestorage.TempObject, string) (bool, error) {
	return false, errors.New("storage is not used by download authorization")
}
func (unusedStorage) Abort(filestorage.TempObject) error { return nil }
func (unusedStorage) Open(context.Context, string) (filestorage.Object, error) {
	return nil, errors.New("storage is not used by download authorization")
}
func (unusedStorage) Delete(context.Context, string) error         { return nil }
func (unusedStorage) Exists(context.Context, string) (bool, error) { return false, nil }

func TestImageDownloadAuthorizationSemantics(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run file download authorization tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	database, rawDB := newChatMessageDatabase(t, values)
	repository := mysql.New(database)
	service, err := fileservice.New(repository, unusedStorage{}, fileservice.Config{MaxUploadBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 20_000)
	owner, peer, stranger, member, outsider := seed, seed+1, seed+2, seed+3, seed+4
	seedChatUsers(t, rawDB, owner, peer, stranger, member, outsider)

	imageFileID := seedIntegrationFile(t, rawDB, owner, "image/png", 2048, "authorization.png")
	chatMessageID := seedChatImageMessage(t, rawDB, owner, peer, imageFileID, "auth-chat-image")
	groupID, ownerMemberID := seedAuthorizationGroup(t, rawDB, owner, member)
	groupMessageID := seedGroupImageMessage(t, rawDB, groupID, owner, ownerMemberID, imageFileID, "auth-group-image")
	fileAttachmentID := seedIntegrationFile(t, rawDB, owner, "application/pdf", 4096, "attachment.pdf")
	groupFileMessageID := seedGroupFileMessage(t, rawDB, groupID, owner, ownerMemberID, fileAttachmentID, "auth-group-file")

	requireAccess := func(label string, userID, fileID uint64) {
		t.Helper()
		if _, err := service.GetAccessible(context.Background(), userID, fmt.Sprintf("%d", fileID)); err != nil {
			t.Fatalf("%s should be allowed: %v", label, err)
		}
	}
	requireDenied := func(label string, userID, fileID uint64) {
		t.Helper()
		_, err := service.GetAccessible(context.Background(), userID, fmt.Sprintf("%d", fileID))
		_, code, _ := apperror.PublicFields(err)
		if err == nil || code != 2503 {
			t.Fatalf("%s = %v (code %d), want 2503", label, err, code)
		}
	}

	requireAccess("owner", owner, imageFileID)
	requireAccess("private receiver", peer, imageFileID)
	requireDenied("non-participant", stranger, imageFileID)
	requireAccess("group member", member, imageFileID)
	requireDenied("non-group member", outsider, imageFileID)
	requireAccess("group member file message", member, fileAttachmentID)
	requireDenied("non-group member file message", outsider, fileAttachmentID)

	if _, err := rawDB.Exec("UPDATE chat_messages SET admin_deleted_at = CURRENT_TIMESTAMP(3) WHERE id = ?", chatMessageID); err != nil {
		t.Fatalf("tombstone chat message: %v", err)
	}
	requireDenied("private receiver after tombstone", peer, imageFileID)
	requireAccess("owner after chat tombstone", owner, imageFileID)

	if _, err := rawDB.Exec("UPDATE group_messages SET admin_deleted_at = CURRENT_TIMESTAMP(3) WHERE id = ?", groupMessageID); err != nil {
		t.Fatalf("tombstone group message: %v", err)
	}
	requireDenied("group member after tombstone", member, imageFileID)
	requireAccess("owner after group tombstone", owner, imageFileID)

	if _, err := rawDB.Exec("UPDATE group_messages SET admin_deleted_at = CURRENT_TIMESTAMP(3) WHERE id = ?", groupFileMessageID); err != nil {
		t.Fatalf("tombstone group file message: %v", err)
	}
	requireDenied("group member after file tombstone", member, fileAttachmentID)
	requireAccess("owner after file tombstone", owner, fileAttachmentID)

	if _, err := rawDB.Exec("UPDATE group_messages SET admin_deleted_at = NULL WHERE id IN (?, ?)", groupMessageID, groupFileMessageID); err != nil {
		t.Fatalf("restore group messages: %v", err)
	}
	if _, err := rawDB.Exec("DELETE FROM group_members WHERE group_id = ? AND user_id = ?", groupID, member); err != nil {
		t.Fatalf("remove group member: %v", err)
	}
	requireDenied("member after leaving group", member, imageFileID)

	requireDenied("unknown file id", peer, imageFileID+999_999)
}

func seedChatUsers(t *testing.T, db *sql.DB, ids ...uint64) {
	t.Helper()
	for _, id := range ids {
		account := fmt.Sprintf("file.auth.%d", id)
		if _, err := db.Exec(`INSERT INTO users(id, account, pwd_hash, nickname, role, status) VALUES (?, ?, 'integration-hash', ?, 2, 1)`, id, account, account); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
}

func seedChatImageMessage(t *testing.T, db *sql.DB, sender, receiver, fileID uint64, clientID string) uint64 {
	t.Helper()
	msg := fmt.Sprintf(`{"type":2,"imageMsg":{"fileID":%d,"url":"/api/file/%d","mimeType":"image/png","size":2048}}`, fileID, fileID)
	result, err := db.Exec(`INSERT INTO chat_messages (send_user_id, rev_user_id, client_msg_id, msg_type, msg_preview, msg)
		VALUES (?, ?, ?, 2, '[图片]', ?)`, sender, receiver, clientID, msg)
	if err != nil {
		t.Fatalf("seed chat image message: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}

func seedAuthorizationGroup(t *testing.T, db *sql.DB, owner, member uint64) (uint64, uint64) {
	t.Helper()
	result, err := db.Exec("INSERT INTO `groups`(name, owner_id, status, join_policy, member_limit) VALUES ('File Auth Group', ?, 1, 1, 100)", owner)
	if err != nil {
		t.Fatalf("seed group: %v", err)
	}
	groupID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	ownerMember, err := db.Exec("INSERT INTO group_members (group_id, user_id, member_nickname, role) VALUES (?, ?, 'owner', 1)", groupID, owner)
	if err != nil {
		t.Fatalf("seed owner membership: %v", err)
	}
	if _, err := db.Exec("INSERT INTO group_members (group_id, user_id, member_nickname, role) VALUES (?, ?, 'member', 3)", groupID, member); err != nil {
		t.Fatalf("seed member membership: %v", err)
	}
	memberID, err := ownerMember.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(groupID), uint64(memberID)
}

func seedGroupImageMessage(t *testing.T, db *sql.DB, groupID, sender, senderMemberID, fileID uint64, clientID string) uint64 {
	t.Helper()
	msg := fmt.Sprintf(`{"type":2,"imageMsg":{"fileID":%d,"url":"/api/file/%d","mimeType":"image/png","size":2048}}`, fileID, fileID)
	return seedGroupMessage(t, db, groupID, sender, senderMemberID, clientID, 2, "[图片]", msg)
}

func seedGroupFileMessage(t *testing.T, db *sql.DB, groupID, sender, senderMemberID, fileID uint64, clientID string) uint64 {
	t.Helper()
	msg := fmt.Sprintf(`{"type":3,"fileMsg":{"fileID":%d,"url":"/api/file/%d","fileName":"attachment.pdf","mimeType":"application/pdf","size":4096}}`, fileID, fileID)
	return seedGroupMessage(t, db, groupID, sender, senderMemberID, clientID, 3, "[文件]", msg)
}

func seedGroupMessage(t *testing.T, db *sql.DB, groupID, sender, senderMemberID uint64, clientID string, msgType uint8, preview, msg string) uint64 {
	t.Helper()
	result, err := db.Exec(`INSERT INTO group_messages (group_id, send_user_id, group_member_id, client_msg_id, msg_type, msg_preview, msg)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, groupID, sender, senderMemberID, clientID, msgType, preview, msg)
	if err != nil {
		t.Fatalf("seed group message: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}
