package tests

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	chatmysql "github.com/kanhai447/GIM/server/internal/chat/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	filerepo "github.com/kanhai447/GIM/server/internal/file/repository"
	filemysql "github.com/kanhai447/GIM/server/internal/file/repository/mysql"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
)

func TestChatImageMessageWebSocketHistoryAndValidation(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Chat image integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	database, rawDB := newChatMessageDatabase(t, values)
	redisConfig, err := platformredis.FromValues(values)
	if err != nil {
		t.Fatalf("redis config: %v", err)
	}
	redisContext, redisCancel := context.WithTimeout(context.Background(), 5*time.Second)
	redisClient, err := platformredis.Open(redisContext, redisConfig)
	redisCancel()
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 10_000)
	senderID, receiverID, offlineID := seed, seed+1, seed+2
	seedChatUsersAndFriendships(t, rawDB, senderID, receiverID, offlineID)
	repository := chatmysql.New(database)
	service, err := message.NewService(repository, integrationRecipients{active: map[uint64]bool{
		senderID: true, receiverID: true, offlineID: true,
	}}, chatmysql.NewFriendshipRepository(database), message.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	imageFileID := seedIntegrationFile(t, rawDB, senderID, "image/png", 1024, "photo.png")
	receiverFileID := seedIntegrationFile(t, rawDB, receiverID, "image/png", 32, "receiver.png")
	textFileID := seedIntegrationFile(t, rawDB, senderID, "text/plain", 16, "note.txt")
	largeFileID := seedIntegrationFile(t, rawDB, senderID, "image/png", 11<<20, "large.png")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	instance := newChatMessageInstance(t, ctx, service, redisClient.Raw())
	sender := dialChatMessageWS(t, instance.server.URL, senderID)
	defer sender.Close()
	receiver := dialChatMessageWS(t, instance.server.URL, receiverID)
	defer receiver.Close()

	sendEnvelope := integrationImageEnvelope(t, "ws-image-key", receiverID, imageFileID, 800, 600)
	if err := sender.WriteMessage(websocket.TextMessage, sendEnvelope); err != nil {
		t.Fatal(err)
	}
	ack := readEnvelope(t, sender, 2*time.Second)
	if ack.Event != protocol.EventChatAck || ack.ClientMsgID != "ws-image-key" {
		t.Fatalf("image ack = %#v", ack)
	}
	ackData, err := protocol.DecodeData[protocol.AckData](ack.Data)
	if err != nil || ackData.MessageID == 0 {
		t.Fatalf("image ack data = %#v, %v", ackData, err)
	}
	event := readEnvelope(t, receiver, 2*time.Second)
	delivered, err := protocol.DecodeData[protocol.PrivateMessage](event.Data)
	if err != nil || event.Event != protocol.EventChatMessage || delivered.MessageID != ackData.MessageID {
		t.Fatalf("image delivery event=%#v data=%#v err=%v", event, delivered, err)
	}
	image := delivered.Message.ImageMsg
	if delivered.Message.Type != message.ImageMessageType || image == nil || image.FileID != imageFileID ||
		image.URL != fmt.Sprintf("/api/file/%d", imageFileID) || image.MIMEType != "image/png" ||
		image.Size != 1024 || image.Width != 800 || image.Height != 600 {
		t.Fatalf("delivered image payload = %#v", delivered.Message)
	}

	stored, err := repository.GetByID(context.Background(), ackData.MessageID)
	if err != nil || stored.Type != message.ImageMessageType || stored.Preview != "[图片]" || stored.Payload.ImageMsg == nil {
		t.Fatalf("stored image = %#v, %v", stored, err)
	}
	page, err := service.History(context.Background(), receiverID, senderID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range page.Items {
		if item.ID == ackData.MessageID && item.Type == message.ImageMessageType && item.Payload.ImageMsg != nil && item.Payload.ImageMsg.FileID == imageFileID {
			found = true
		}
	}
	if !found {
		t.Fatalf("image message missing from history: %#v", page.Items)
	}

	retry, err := service.SendPrivateMessage(context.Background(), senderID, message.SendRequest{
		ClientMsgID: "ws-image-key", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.ImageMessageType, ImageMsg: &protocol.ImageMessage{FileID: imageFileID, Width: 800, Height: 600}},
	})
	if err != nil || retry.CreatedNew || retry.Message.ID != ackData.MessageID {
		t.Fatalf("image retry = %#v, %v", retry, err)
	}
	var count int64
	if err := database.Table("chat_messages").Where("send_user_id = ? AND client_msg_id = ?", senderID, "ws-image-key").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("image row count = %d, %v", count, err)
	}

	for index, test := range []struct {
		name     string
		fileID   uint64
		wantCode uint32
	}{
		{name: "file belongs to receiver", fileID: receiverFileID, wantCode: message.CodeImageFileInvalid},
		{name: "not an image", fileID: textFileID, wantCode: message.CodeImageMIMEType},
		{name: "image too large", fileID: largeFileID, wantCode: message.CodeImageTooLarge},
	} {
		clientMsgID := fmt.Sprintf("invalid-image-%d", index)
		_, sendErr := service.SendPrivateMessage(context.Background(), senderID, message.SendRequest{
			ClientMsgID: clientMsgID, ReceiverID: receiverID,
			Message: protocol.MessagePayload{Type: message.ImageMessageType, ImageMsg: &protocol.ImageMessage{FileID: test.fileID, Width: 10, Height: 10}},
		})
		publicError, ok := sendErr.(*message.PublicError)
		if !ok || publicError.Code != test.wantCode {
			t.Fatalf("%s error = %v, want code %d", test.name, sendErr, test.wantCode)
		}
		if err := database.Table("chat_messages").Where("send_user_id = ? AND client_msg_id = ?", senderID, clientMsgID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s persisted rows = %d, %v", test.name, count, err)
		}
	}

	fileRepository := filemysql.New(database)
	if _, err := fileRepository.FindAccessibleByID(context.Background(), receiverID, imageFileID); err != nil {
		t.Fatalf("receiver cannot resolve the image file: %v", err)
	}
	if _, err := fileRepository.FindAccessibleByID(context.Background(), offlineID, imageFileID); !errors.Is(err, filerepo.ErrNotFound) {
		t.Fatalf("non-participant image access = %v, want ErrNotFound", err)
	}
}

func seedIntegrationFile(t *testing.T, db *sql.DB, userID uint64, mimeType string, size int64, name string) uint64 {
	t.Helper()
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d-%s-%d-%s", userID, mimeType, size, name)))
	sha := hex.EncodeToString(digest[:])
	uid := fmt.Sprintf("%s-%s", sha[:8], sha[8:16])
	result, err := db.Exec(`INSERT INTO file_objects (uid, sha256, size, storage_path, mime_type) VALUES (?, ?, ?, ?, ?)`,
		uid, sha, size, "objects/test/"+uid, mimeType)
	if err != nil {
		t.Fatalf("seed file object: %v", err)
	}
	objectID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	reference, err := db.Exec(`INSERT INTO user_files (user_id, file_object_id, original_name) VALUES (?, ?, ?)`, userID, objectID, name)
	if err != nil {
		t.Fatalf("seed user file: %v", err)
	}
	fileID, err := reference.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(fileID)
}

func integrationImageEnvelope(t *testing.T, clientMsgID string, receiverID, fileID uint64, width, height int) []byte {
	t.Helper()
	data, err := protocol.Data(protocol.ChatSendData{
		ReceiverID: receiverID,
		Message: protocol.MessagePayload{
			Type:     message.ImageMessageType,
			ImageMsg: &protocol.ImageMessage{FileID: fileID, Width: width, Height: height},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := protocol.Encode(protocol.Envelope{Event: protocol.EventChatSend, ClientMsgID: clientMsgID, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
