package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanhai447/GIM/server/internal/chat/friendnotify"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	chatmysql "github.com/kanhai447/GIM/server/internal/chat/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
)

func TestFriendPresenceNotifications(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run friend presence integration tests")
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

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 70_000)
	userA, userB, userC, userD := seed, seed+1, seed+2, seed+3
	for _, id := range []uint64{userA, userB, userC, userD} {
		status := 1
		if id == userD {
			status = 2
		}
		account := fmt.Sprintf("friend.presence.%d", id)
		if _, err := rawDB.Exec(`INSERT INTO users(id, account, pwd_hash, nickname, role, status) VALUES (?, ?, 'integration-hash', ?, 2, ?)`, id, account, account, status); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	for _, pair := range [][2]uint64{{userA, userB}, {userB, userA}, {userA, userD}, {userD, userA}} {
		if _, err := rawDB.Exec(`INSERT INTO friends(user_id, friend_id) VALUES (?, ?)`, pair[0], pair[1]); err != nil {
			t.Fatalf("seed friendship: %v", err)
		}
	}

	repository := chatmysql.New(database)
	service, err := message.NewService(repository, integrationRecipients{active: map[uint64]bool{
		userA: true, userB: true, userC: true, userD: false,
	}}, chatmysql.NewFriendshipRepository(database), message.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	notifier, err := friendnotify.New(chatmysql.NewFriendshipRepository(database), friendnotify.Config{
		Buffer: 16, Workers: 1, OperationTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	instance := newChatMessageInstance(t, ctx, service, redisClient.Raw(), notifier)
	notifier.SetSender(instance.hub)
	notifier.Start(ctx)
	t.Cleanup(func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = notifier.Shutdown(shutdownContext)
	})

	bConn := dialChatMessageWS(t, instance.server.URL, userB)
	defer bConn.Close()
	bReader := newWSReader(bConn)
	aFirst := dialChatMessageWS(t, instance.server.URL, userA)
	online := bReader.next(t, 3*time.Second)
	if online.Event != protocol.EventFriendOnline || online.Data.UserID != userA {
		t.Fatalf("online event = %s / %#v", online.Event, online.Data)
	}

	aSecond := dialChatMessageWS(t, instance.server.URL, userA)
	bReader.expectNone(t, 300*time.Millisecond)
	if err := aFirst.Close(); err != nil {
		t.Fatal(err)
	}
	bReader.expectNone(t, 300*time.Millisecond)
	if err := aSecond.Close(); err != nil {
		t.Fatal(err)
	}
	offline := bReader.next(t, 3*time.Second)
	if offline.Event != protocol.EventFriendOffline || offline.Data.UserID != userA {
		t.Fatalf("offline event = %s / %#v", offline.Event, offline.Data)
	}

	cConn := dialChatMessageWS(t, instance.server.URL, userC)
	defer cConn.Close()
	cReader := newWSReader(cConn)
	aThird := dialChatMessageWS(t, instance.server.URL, userA)
	online = bReader.next(t, 3*time.Second)
	if online.Event != protocol.EventFriendOnline {
		t.Fatalf("second online event = %s", online.Event)
	}
	cReader.expectNone(t, 300*time.Millisecond)
	if err := aThird.Close(); err != nil {
		t.Fatal(err)
	}
	offline = bReader.next(t, 3*time.Second)
	if offline.Event != protocol.EventFriendOffline {
		t.Fatalf("second offline event = %s", offline.Event)
	}
	cReader.expectNone(t, 300*time.Millisecond)

	dConn := dialChatMessageWS(t, instance.server.URL, userD)
	defer dConn.Close()
	dReader := newWSReader(dConn)
	aFourth := dialChatMessageWS(t, instance.server.URL, userA)
	online = bReader.next(t, 3*time.Second)
	if online.Event != protocol.EventFriendOnline {
		t.Fatalf("third online event = %s", online.Event)
	}
	dReader.expectNone(t, 300*time.Millisecond)
	if err := aFourth.Close(); err != nil {
		t.Fatal(err)
	}
	offline = bReader.next(t, 3*time.Second)
	if offline.Event != protocol.EventFriendOffline {
		t.Fatalf("third offline event = %s", offline.Event)
	}
	dReader.expectNone(t, 300*time.Millisecond)
}

type wsReader struct {
	messages chan []byte
	errs     chan error
}

func newWSReader(connection *websocket.Conn) *wsReader {
	reader := &wsReader{messages: make(chan []byte, 16), errs: make(chan error, 1)}
	go func() {
		for {
			_, payload, err := connection.ReadMessage()
			if err != nil {
				reader.errs <- err
				return
			}
			reader.messages <- payload
		}
	}()
	return reader
}

func (reader *wsReader) next(t *testing.T, timeout time.Duration) friendEvent {
	t.Helper()
	select {
	case payload := <-reader.messages:
		var envelope protocol.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		data, err := protocol.DecodeData[protocol.FriendPresenceData](envelope.Data)
		if err != nil {
			t.Fatalf("decode friend event: %v", err)
		}
		return friendEvent{Event: envelope.Event, Data: data}
	case err := <-reader.errs:
		t.Fatalf("read websocket: %v", err)
	case <-time.After(timeout):
		t.Fatal("timed out waiting for friend event")
	}
	return friendEvent{}
}

func (reader *wsReader) expectNone(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case payload := <-reader.messages:
		t.Fatalf("unexpected friend event: %s", string(payload))
	case err := <-reader.errs:
		t.Fatalf("read websocket while expecting no event: %v", err)
	case <-time.After(timeout):
	}
}

type friendEvent struct {
	Event string
	Data  protocol.FriendPresenceData
}
