package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	chatmysql "github.com/kanhai447/GIM/server/internal/chat/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	filemysql "github.com/kanhai447/GIM/server/internal/file/repository/mysql"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
)

func TestChatFileMessageWebSocketHistoryAndValidation(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Chat file integration tests")
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

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 40_000)
	senderID, receiverID, offlineID := seed, seed+1, seed+2
	seedChatUsersAndFriendships(t, rawDB, senderID, receiverID, offlineID)
	repository := chatmysql.New(database)
	service, err := message.NewService(repository, integrationRecipients{active: map[uint64]bool{
		senderID: true, receiverID: true, offlineID: true,
	}}, chatmysql.NewFriendshipRepository(database), message.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	fileID := seedIntegrationFile(t, rawDB, senderID, "application/pdf", 4096, "report.pdf")
	receiverFileID := seedIntegrationFile(t, rawDB, receiverID, "text/plain", 16, "receiver.txt")
	largeFileID := seedIntegrationFile(t, rawDB, senderID, "application/octet-stream", 26<<20, "big.bin")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	instance := newChatMessageInstance(t, ctx, service, redisClient.Raw())
	sender := dialChatMessageWS(t, instance.server.URL, senderID)
	defer sender.Close()
	receiver := dialChatMessageWS(t, instance.server.URL, receiverID)
	defer receiver.Close()

	sendEnvelope := integrationFileEnvelope(t, "ws-file-key", receiverID, fileID)
	if err := sender.WriteMessage(websocket.TextMessage, sendEnvelope); err != nil {
		t.Fatal(err)
	}
	ack := readEnvelope(t, sender, 2*time.Second)
	if ack.Event != protocol.EventChatAck || ack.ClientMsgID != "ws-file-key" {
		t.Fatalf("file ack = %#v", ack)
	}
	ackData, err := protocol.DecodeData[protocol.AckData](ack.Data)
	if err != nil || ackData.MessageID == 0 {
		t.Fatalf("file ack data = %#v, %v", ackData, err)
	}
	event := readEnvelope(t, receiver, 2*time.Second)
	delivered, err := protocol.DecodeData[protocol.PrivateMessage](event.Data)
	if err != nil || event.Event != protocol.EventChatMessage || delivered.MessageID != ackData.MessageID {
		t.Fatalf("file delivery event=%#v data=%#v err=%v", event, delivered, err)
	}
	file := delivered.Message.FileMsg
	if delivered.Message.Type != message.FileMessageType || file == nil || file.FileID != fileID ||
		file.URL != fmt.Sprintf("/api/file/%d", fileID) || file.FileName != "report.pdf" ||
		file.Size != 4096 || file.MIMEType != "application/pdf" {
		t.Fatalf("delivered file payload = %#v", delivered.Message)
	}

	stored, err := repository.GetByID(context.Background(), ackData.MessageID)
	if err != nil || stored.Type != message.FileMessageType || stored.Preview != "[文件]" || stored.Payload.FileMsg == nil {
		t.Fatalf("stored file = %#v, %v", stored, err)
	}
	page, err := service.History(context.Background(), receiverID, senderID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range page.Items {
		if item.ID == ackData.MessageID && item.Type == message.FileMessageType && item.Payload.FileMsg != nil && item.Payload.FileMsg.FileID == fileID {
			found = true
		}
	}
	if !found {
		t.Fatalf("file message missing from history: %#v", page.Items)
	}

	retry, err := service.SendPrivateMessage(context.Background(), senderID, message.SendRequest{
		ClientMsgID: "ws-file-key", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.FileMessageType, FileMsg: &protocol.FileMessage{FileID: fileID}},
	})
	if err != nil || retry.CreatedNew || retry.Message.ID != ackData.MessageID {
		t.Fatalf("file retry = %#v, %v", retry, err)
	}
	var count int64
	if err := database.Table("chat_messages").Where("send_user_id = ? AND client_msg_id = ?", senderID, "ws-file-key").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("file row count = %d, %v", count, err)
	}

	for index, test := range []struct {
		name     string
		fileID   uint64
		wantCode uint32
	}{
		{name: "file belongs to receiver", fileID: receiverFileID, wantCode: message.CodeFileInvalid},
		{name: "file too large", fileID: largeFileID, wantCode: message.CodeFileTooLarge},
	} {
		clientMsgID := fmt.Sprintf("invalid-file-%d", index)
		_, sendErr := service.SendPrivateMessage(context.Background(), senderID, message.SendRequest{
			ClientMsgID: clientMsgID, ReceiverID: receiverID,
			Message: protocol.MessagePayload{Type: message.FileMessageType, FileMsg: &protocol.FileMessage{FileID: test.fileID}},
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
	if _, err := fileRepository.FindAccessibleByID(context.Background(), receiverID, fileID); err != nil {
		t.Fatalf("receiver cannot resolve the file message attachment: %v", err)
	}
	if _, err := fileRepository.FindAccessibleByID(context.Background(), offlineID, fileID); err == nil {
		t.Fatal("non-participant resolved the file message attachment")
	}
}

func integrationFileEnvelope(t *testing.T, clientMsgID string, receiverID, fileID uint64) []byte {
	t.Helper()
	data, err := protocol.Data(protocol.ChatSendData{
		ReceiverID: receiverID,
		Message: protocol.MessagePayload{
			Type:    message.FileMessageType,
			FileMsg: &protocol.FileMessage{FileID: fileID},
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
