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
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
)

func TestRecallMessageWebSocketHistoryAndAuthorization(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run recall integration tests")
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

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 50_000)
	senderID, receiverID, offlineID := seed, seed+1, seed+2
	seedChatUsersAndFriendships(t, rawDB, senderID, receiverID, offlineID)
	repository := chatmysql.New(database)
	service, err := message.NewService(repository, integrationRecipients{active: map[uint64]bool{
		senderID: true, receiverID: true, offlineID: true,
	}}, chatmysql.NewFriendshipRepository(database), message.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	instance := newChatMessageInstance(t, ctx, service, redisClient.Raw())
	sender := dialChatMessageWS(t, instance.server.URL, senderID)
	defer sender.Close()
	receiver := dialChatMessageWS(t, instance.server.URL, receiverID)
	defer receiver.Close()

	target, err := service.SendPrivateMessage(ctx, senderID, message.SendRequest{
		ClientMsgID: "recall-target-text", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.TextMessageType, TextMsg: &protocol.TextMessage{Content: "secret text"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.WriteMessage(websocket.TextMessage, integrationWithdrawEnvelope(t, "ws-recall", receiverID, target.Message.ID)); err != nil {
		t.Fatal(err)
	}
	ack := readEnvelope(t, sender, 2*time.Second)
	if ack.Event != protocol.EventChatAck || ack.ClientMsgID != "ws-recall" {
		t.Fatalf("recall ack = %#v", ack)
	}
	ackData, err := protocol.DecodeData[protocol.AckData](ack.Data)
	if err != nil || ackData.MessageID == 0 {
		t.Fatalf("recall ack data = %#v, %v", ackData, err)
	}
	event := readEnvelope(t, receiver, 2*time.Second)
	delivered, err := protocol.DecodeData[protocol.PrivateMessage](event.Data)
	if err != nil || event.Event != protocol.EventChatMessage || delivered.Message.Type != message.WithdrawMessageType ||
		delivered.Message.WithdrawMsg == nil || delivered.Message.WithdrawMsg.MsgID != target.Message.ID {
		t.Fatalf("recall delivery event=%#v data=%#v err=%v", event, delivered, err)
	}

	page, err := service.History(ctx, receiverID, senderID, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	targetPlaceholder, withdrawVisible := false, false
	for _, item := range page.Items {
		if item.ID == target.Message.ID {
			targetPlaceholder = true
			if item.Type != message.WithdrawMessageType || item.Preview != "[撤回]" || item.Payload.TextMsg != nil ||
				item.Payload.WithdrawMsg == nil || item.Payload.WithdrawMsg.MsgID != target.Message.ID {
				t.Fatalf("history target not replaced: %#v", item)
			}
		}
		if item.ID == ackData.MessageID {
			withdrawVisible = true
		}
	}
	if !targetPlaceholder || !withdrawVisible {
		t.Fatalf("history target=%v withdraw=%v", targetPlaceholder, withdrawVisible)
	}

	duplicate, err := service.SendPrivateMessage(ctx, senderID, message.SendRequest{
		ClientMsgID: "ws-recall-2", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: target.Message.ID}},
	})
	if err != nil || duplicate.CreatedNew || duplicate.Message.ID != ackData.MessageID {
		t.Fatalf("duplicate recall = %#v, %v", duplicate, err)
	}
	var withdrawCount int64
	if err := database.Table("chat_messages").Where("send_user_id = ? AND msg_type = 4 AND JSON_UNQUOTE(JSON_EXTRACT(msg, '$.withdrawMsg.msgID')) = ?", senderID, fmt.Sprint(target.Message.ID)).Count(&withdrawCount).Error; err != nil || withdrawCount != 1 {
		t.Fatalf("withdraw rows = %d, %v", withdrawCount, err)
	}

	nonOwner, err := service.SendPrivateMessage(ctx, receiverID, message.SendRequest{
		ClientMsgID: "non-owner-recall", ReceiverID: senderID,
		Message: protocol.MessagePayload{Type: message.WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: target.Message.ID}},
	})
	_ = nonOwner
	requirePublicCode(t, err, message.CodeWithdrawNotOwner)

	tombstoneTarget, err := service.SendPrivateMessage(ctx, senderID, message.SendRequest{
		ClientMsgID: "tombstone-target", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.TextMessageType, TextMsg: &protocol.TextMessage{Content: "tombstone"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawDB.Exec("UPDATE chat_messages SET admin_deleted_at = CURRENT_TIMESTAMP(3) WHERE id = ?", tombstoneTarget.Message.ID); err != nil {
		t.Fatal(err)
	}
	_, err = service.SendPrivateMessage(ctx, senderID, message.SendRequest{
		ClientMsgID: "tombstone-recall", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: tombstoneTarget.Message.ID}},
	})
	requirePublicCode(t, err, message.CodeWithdrawRecalled)

	oldTarget, err := service.SendPrivateMessage(ctx, senderID, message.SendRequest{
		ClientMsgID: "old-target", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.TextMessageType, TextMsg: &protocol.TextMessage{Content: "old"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawDB.Exec("UPDATE chat_messages SET created_at = DATE_SUB(NOW(3), INTERVAL 1 HOUR) WHERE id = ?", oldTarget.Message.ID); err != nil {
		t.Fatal(err)
	}
	shortWindowConfig := message.DefaultConfig()
	shortWindowConfig.RecallWindow = time.Second
	shortWindowService, err := message.NewService(repository, integrationRecipients{active: map[uint64]bool{senderID: true, receiverID: true}},
		chatmysql.NewFriendshipRepository(database), shortWindowConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = shortWindowService.SendPrivateMessage(ctx, senderID, message.SendRequest{
		ClientMsgID: "old-recall", ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: message.WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: oldTarget.Message.ID}},
	})
	requirePublicCode(t, err, message.CodeWithdrawWindow)
}

func requirePublicCode(t *testing.T, err error, want uint32) {
	t.Helper()
	publicError, ok := err.(*message.PublicError)
	if !ok || publicError.Code != want {
		t.Fatalf("error = %v, want code %d", err, want)
	}
}

func integrationWithdrawEnvelope(t *testing.T, clientMsgID string, receiverID, targetID uint64) []byte {
	t.Helper()
	data, err := protocol.Data(protocol.ChatSendData{
		ReceiverID: receiverID,
		Message: protocol.MessagePayload{
			Type:        message.WithdrawMessageType,
			WithdrawMsg: &protocol.WithdrawMessage{MsgID: targetID},
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
