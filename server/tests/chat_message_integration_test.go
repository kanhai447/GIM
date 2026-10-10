package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/delivery"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	chatmysql "github.com/kanhai447/GIM/server/internal/chat/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/migrations"
	redisv9 "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var chatMessageDatabasePattern = regexp.MustCompile(`^gim_chat_message_test_[0-9]+_[0-9]+$`)

type integrationRecipients struct{ active map[uint64]bool }

func (recipients integrationRecipients) GetByID(_ context.Context, userID uint64) (message.Recipient, error) {
	active, exists := recipients.active[userID]
	if !exists {
		return message.Recipient{}, message.ErrRecipientNotFound
	}
	return message.Recipient{ID: userID, Active: active}, nil
}

func TestChatMessageMySQLAndWebSocketReliability(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Chat message MySQL/WebSocket integration tests")
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
	friendships := chatmysql.NewFriendshipRepository(database)
	service, err := message.NewService(repository, integrationRecipients{active: map[uint64]bool{
		senderID: true, receiverID: true, offlineID: true,
	}}, friendships, message.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("repository create query duplicate and id", func(t *testing.T) {
		created, err := repository.Create(context.Background(), integrationMessage(senderID, receiverID, "repository-key", "repository"))
		if err != nil || created.ID == 0 {
			t.Fatalf("create = %#v, %v", created, err)
		}
		if _, err := repository.Create(context.Background(), integrationMessage(senderID, receiverID, "repository-key", "repository")); !errors.Is(err, message.ErrDuplicate) {
			t.Fatalf("duplicate error = %v", err)
		}
		byKey, err := repository.GetBySenderAndClientMsgID(context.Background(), senderID, "repository-key")
		if err != nil || byKey.ID != created.ID {
			t.Fatalf("by key = %#v, %v", byKey, err)
		}
		byID, err := repository.GetByID(context.Background(), created.ID)
		if err != nil || byID.ClientMsgID != "repository-key" {
			t.Fatalf("by id = %#v, %v", byID, err)
		}
	})

	t.Run("concurrent unique constraint idempotency", func(t *testing.T) {
		request := integrationRequest("concurrent-key", receiverID, "concurrent")
		const attempts = 50
		ids := make(chan uint64, attempts)
		errorsFound := make(chan error, attempts)
		var wait sync.WaitGroup
		for range attempts {
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, sendErr := service.SendPrivateMessage(context.Background(), senderID, request)
				if sendErr != nil {
					errorsFound <- sendErr
					return
				}
				ids <- result.Message.ID
			}()
		}
		wait.Wait()
		close(ids)
		close(errorsFound)
		for sendErr := range errorsFound {
			t.Fatalf("concurrent send: %v", sendErr)
		}
		var first uint64
		for id := range ids {
			if first == 0 {
				first = id
			}
			if id != first {
				t.Fatalf("concurrent message IDs differ: %d and %d", first, id)
			}
		}
		var count int64
		if err := database.Table("chat_messages").Where("send_user_id = ? AND client_msg_id = ?", senderID, "concurrent-key").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("concurrent row count = %d, %v", count, err)
		}
	})

	t.Run("real websocket ack delivery retry offline and multi instance", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		instanceA := newChatMessageInstance(t, ctx, service, redisClient.Raw())
		instanceB := newChatMessageInstance(t, ctx, service, redisClient.Raw())

		sender := dialChatMessageWS(t, instanceA.server.URL, senderID)
		defer sender.Close()
		senderSibling := dialChatMessageWS(t, instanceB.server.URL, senderID)
		defer senderSibling.Close()
		receiverA := dialChatMessageWS(t, instanceA.server.URL, receiverID)
		defer receiverA.Close()
		receiverB1 := dialChatMessageWS(t, instanceB.server.URL, receiverID)
		defer receiverB1.Close()
		receiverB2 := dialChatMessageWS(t, instanceB.server.URL, receiverID)
		defer receiverB2.Close()
		if err := sender.WriteMessage(websocket.TextMessage, []byte(`{"event":`)); err != nil {
			t.Fatal(err)
		}
		malformed := readEnvelope(t, sender, time.Second)
		if malformed.Event != protocol.EventError || malformed.Error == nil || malformed.Error.Code != message.CodeInvalidMessage {
			t.Fatalf("malformed response = %#v", malformed)
		}

		sendEnvelope := integrationEnvelope(t, "ws-key", receiverID, "hello across instances")
		if err := sender.WriteMessage(websocket.TextMessage, sendEnvelope); err != nil {
			t.Fatal(err)
		}
		ack := readEnvelope(t, sender, time.Second)
		if ack.Event != protocol.EventChatAck || ack.ClientMsgID != "ws-key" {
			t.Fatalf("ack = %#v", ack)
		}
		ackData, err := protocol.DecodeData[protocol.AckData](ack.Data)
		if err != nil || ackData.MessageID == 0 {
			t.Fatalf("ack data = %#v, %v", ackData, err)
		}
		for index, receiver := range []*websocket.Conn{receiverA, receiverB1, receiverB2} {
			event := readEnvelope(t, receiver, time.Second)
			data, decodeErr := protocol.DecodeData[protocol.PrivateMessage](event.Data)
			if decodeErr != nil || event.Event != protocol.EventChatMessage || data.MessageID != ackData.MessageID || data.ReceiverID != receiverID {
				t.Fatalf("receiver %d event=%#v data=%#v err=%v", index, event, data, decodeErr)
			}
		}
		if err := senderSibling.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := senderSibling.ReadMessage(); err == nil {
			t.Fatal("sender sibling received the originating client's ACK")
		}

		if err := sender.WriteMessage(websocket.TextMessage, sendEnvelope); err != nil {
			t.Fatal(err)
		}
		retryAck := readEnvelope(t, sender, time.Second)
		retryData, err := protocol.DecodeData[protocol.AckData](retryAck.Data)
		if err != nil || retryData.MessageID != ackData.MessageID {
			t.Fatalf("retry ack = %#v data=%#v err=%v", retryAck, retryData, err)
		}
		for index, receiver := range []*websocket.Conn{receiverA, receiverB1, receiverB2} {
			if err := receiver.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := receiver.ReadMessage(); err == nil {
				t.Fatalf("receiver %d received duplicate realtime delivery", index)
			}
		}

		offlineEnvelope := integrationEnvelope(t, "offline-key", offlineID, "stored while offline")
		if err := sender.WriteMessage(websocket.TextMessage, offlineEnvelope); err != nil {
			t.Fatal(err)
		}
		offlineAck := readEnvelope(t, sender, time.Second)
		offlineAckData, err := protocol.DecodeData[protocol.AckData](offlineAck.Data)
		if err != nil || offlineAck.Event != protocol.EventChatAck || offlineAckData.MessageID == 0 {
			t.Fatalf("offline ack = %#v data=%#v err=%v", offlineAck, offlineAckData, err)
		}
		stored, err := repository.GetByID(context.Background(), offlineAckData.MessageID)
		if err != nil || stored.ReceiverID != offlineID {
			t.Fatalf("offline stored = %#v, %v", stored, err)
		}
	})

	t.Run("lost ack service retry", func(t *testing.T) {
		request := integrationRequest("lost-ack-key", receiverID, "retry after lost ack")
		first, err := service.SendPrivateMessage(context.Background(), senderID, request)
		if err != nil || !first.CreatedNew {
			t.Fatalf("first = %#v, %v", first, err)
		}
		second, err := service.SendPrivateMessage(context.Background(), senderID, request)
		if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID {
			t.Fatalf("retry = %#v, %v", second, err)
		}
	})

	var sessionCount int64
	if err := database.Table("chat_sessions").Count(&sessionCount).Error; err != nil || sessionCount != 0 {
		t.Fatalf("checkpoint 4 unexpectedly changed chat_sessions: count=%d err=%v", sessionCount, err)
	}
}

type chatMessageInstance struct {
	hub    *chat.Hub
	server *httptest.Server
}

func newChatMessageInstance(t *testing.T, ctx context.Context, service *message.Service, redisClient *redisv9.Client) *chatMessageInstance {
	t.Helper()
	hub := chat.NewHub()
	go hub.Run(ctx)
	bus, err := delivery.New(redisClient, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	inbound, err := message.NewInbound(service, hub, bus, nil)
	if err != nil {
		t.Fatal(err)
	}
	origins, err := chat.NewOriginPolicy([]string{"http://localhost:5173"})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := chat.NewHandler(hub, inbound, 64, origins, chat.HeartbeatConfig{
		ReadLimit: 1 << 20, PongWait: time.Minute, PingPeriod: 50 * time.Second, WriteWait: time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := bus.Subscribe(ctx, message.LocalDeliveryHandler(hub, time.Second, nil))
	if err != nil {
		t.Fatal(err)
	}
	go subscription.Run(ctx)
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
		_ = subscription.Close()
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = hub.Shutdown(shutdownContext)
	})
	return &chatMessageInstance{hub: hub, server: server}
}

func newChatMessageDatabase(t *testing.T, values platformconfig.Values) (*gorm.DB, *sql.DB) {
	t.Helper()
	baseConfig, err := platformmysql.FromValues(values)
	if err != nil {
		t.Fatalf("mysql config: %v", err)
	}
	name := fmt.Sprintf("gim_chat_message_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if !chatMessageDatabasePattern.MatchString(name) {
		t.Fatal("unsafe test database name")
	}
	adminConfig := baseConfig
	adminConfig.Database = ""
	admin, err := platformmysql.OpenSQL(context.Background(), adminConfig, false)
	if err != nil {
		t.Fatalf("open mysql admin: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = admin.Close()
		t.Fatalf("create database: %v", err)
	}
	testConfig := baseConfig
	testConfig.Database = name
	raw, err := platformmysql.OpenSQL(context.Background(), testConfig, true)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	runner, err := migrations.New(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	client, err := platformmysql.Open(context.Background(), testConfig)
	if err != nil {
		t.Fatalf("open gorm test database: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		_ = raw.Close()
		if chatMessageDatabasePattern.MatchString(name) {
			if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + name + "`"); err != nil {
				t.Errorf("drop chat message test database: %v", err)
			}
			var remaining int
			if err := admin.QueryRow(`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`, name).Scan(&remaining); err != nil || remaining != 0 {
				t.Errorf("chat message test database cleanup: remaining=%d err=%v", remaining, err)
			}
		}
		_ = admin.Close()
	})
	return client.DB(), raw
}

func seedChatUsersAndFriendships(t *testing.T, db *sql.DB, senderID, receiverID, offlineID uint64) {
	t.Helper()
	for _, id := range []uint64{senderID, receiverID, offlineID} {
		account := fmt.Sprintf("chat.integration.%d", id)
		if _, err := db.Exec(`INSERT INTO users(id, account, pwd_hash, nickname, role, status) VALUES (?, ?, 'integration-hash', ?, 2, 1)`, id, account, account); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	for _, peer := range []uint64{receiverID, offlineID} {
		if _, err := db.Exec(`INSERT INTO friends(user_id, friend_id) VALUES (?, ?)`, senderID, peer); err != nil {
			t.Fatalf("seed friendship: %v", err)
		}
	}
}

func integrationMessage(senderID, receiverID uint64, clientMsgID, content string) message.Message {
	return message.Message{
		SenderID: senderID, ReceiverID: receiverID, ClientMsgID: clientMsgID, Type: message.TextMessageType,
		Preview: content, Payload: protocol.MessagePayload{Type: message.TextMessageType, TextMsg: &protocol.TextMessage{Content: content}},
	}
}

func integrationRequest(clientMsgID string, receiverID uint64, content string) message.SendRequest {
	value := integrationMessage(1, receiverID, clientMsgID, content)
	return message.SendRequest{ClientMsgID: clientMsgID, ReceiverID: receiverID, Message: value.Payload}
}

func integrationEnvelope(t *testing.T, clientMsgID string, receiverID uint64, content string) []byte {
	t.Helper()
	data, err := protocol.Data(protocol.ChatSendData{
		ReceiverID: receiverID,
		Message:    protocol.MessagePayload{Type: message.TextMessageType, TextMsg: &protocol.TextMessage{Content: content}},
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

func dialChatMessageWS(t *testing.T, serverURL string, userID uint64) *websocket.Conn {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Scheme = "ws"
	header := http.Header{"Origin": []string{"http://localhost:5173"}, "User-ID": []string{fmt.Sprint(userID)}, "Role": []string{"2"}}
	connection, _, err := websocket.DefaultDialer.Dial(parsed.String(), header)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return connection
}

func readEnvelope(t *testing.T, connection *websocket.Conn, timeout time.Duration) protocol.Envelope {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatal(err)
	}
	_, payload, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}
	var envelope protocol.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode websocket envelope: %v", err)
	}
	return envelope
}
