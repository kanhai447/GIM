package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/delivery"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	chatmysql "github.com/kanhai447/GIM/server/internal/chat/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"gorm.io/gorm"
)

func TestChatSessionUnreadReadAndHistoryMySQL(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Chat session MySQL integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	database, rawDB := newChatMessageDatabase(t, values)
	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 20_000)
	senderID, receiverID, otherPeerID := seed, seed+1, seed+2
	seedChatUsersAndFriendships(t, rawDB, senderID, receiverID, otherPeerID)
	repository := chatmysql.New(database)
	recipients := integrationRecipients{active: map[uint64]bool{
		senderID: true, receiverID: true, otherPeerID: true,
	}}
	service, err := message.NewService(repository, recipients, chatmysql.NewFriendshipRepository(database), message.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("fifty duplicate sends mutate message and unread once", func(t *testing.T) {
		const attempts = 50
		request := integrationRequest("cp5-duplicate", receiverID, "duplicate once")
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
			t.Fatalf("duplicate send: %v", sendErr)
		}
		var firstID uint64
		for id := range ids {
			if firstID == 0 {
				firstID = id
			}
			if id != firstID {
				t.Fatalf("duplicate send returned IDs %d and %d", firstID, id)
			}
		}
		assertMessageCount(t, database, senderID, "cp5-duplicate", 1)
		assertSessionState(t, repository, receiverID, senderID, firstID, 1)
		assertSessionState(t, repository, senderID, receiverID, firstID, 0)
	})

	t.Run("fifty distinct sends preserve every unread increment and maximum message", func(t *testing.T) {
		const attempts = 50
		ids := make(chan uint64, attempts)
		errorsFound := make(chan error, attempts)
		var wait sync.WaitGroup
		for index := range attempts {
			wait.Add(1)
			go func(index int) {
				defer wait.Done()
				result, sendErr := service.SendPrivateMessage(context.Background(), senderID, integrationRequest(
					fmt.Sprintf("cp5-distinct-%02d", index), receiverID, fmt.Sprintf("distinct %02d", index),
				))
				if sendErr != nil {
					errorsFound <- sendErr
					return
				}
				ids <- result.Message.ID
			}(index)
		}
		wait.Wait()
		close(ids)
		close(errorsFound)
		for sendErr := range errorsFound {
			t.Fatalf("distinct send: %v", sendErr)
		}
		var allIDs []uint64
		for id := range ids {
			allIDs = append(allIDs, id)
		}
		if len(allIDs) != attempts {
			t.Fatalf("distinct IDs = %d, want %d", len(allIDs), attempts)
		}
		sort.Slice(allIDs, func(first, second int) bool { return allIDs[first] < allIDs[second] })
		assertSessionState(t, repository, receiverID, senderID, allIDs[len(allIDs)-1], 51)
		assertSessionState(t, repository, senderID, receiverID, allIDs[len(allIDs)-1], 0)
	})

	t.Run("partial stale latest and concurrent read semantics", func(t *testing.T) {
		ids := incomingMessageIDs(t, rawDB, senderID, receiverID)
		if len(ids) < 3 {
			t.Fatalf("incoming messages = %d", len(ids))
		}
		partialID := ids[len(ids)-2]
		partial, err := service.MarkRead(context.Background(), receiverID, senderID, partialID)
		if err != nil || partial.UnreadCount != 1 || partial.LastReadMessageID == nil || *partial.LastReadMessageID != partialID {
			t.Fatalf("partial read = %#v, %v", partial, err)
		}
		stale, err := service.MarkRead(context.Background(), receiverID, senderID, ids[0])
		if err != nil || stale.UnreadCount != 1 || stale.LastReadMessageID == nil || *stale.LastReadMessageID != partialID {
			t.Fatalf("stale read = %#v, %v", stale, err)
		}
		latestID := ids[len(ids)-1]
		latest, err := service.MarkRead(context.Background(), receiverID, senderID, latestID)
		if err != nil || latest.UnreadCount != 0 || latest.LastReadMessageID == nil || *latest.LastReadMessageID != latestID {
			t.Fatalf("latest read = %#v, %v", latest, err)
		}
		duplicate, err := service.MarkRead(context.Background(), receiverID, senderID, latestID)
		if err != nil || duplicate.UnreadCount != 0 || duplicate.LastReadMessageID == nil || *duplicate.LastReadMessageID != latestID {
			t.Fatalf("duplicate read = %#v, %v", duplicate, err)
		}

		other, err := service.SendPrivateMessage(context.Background(), senderID, integrationRequest("cp5-other-conversation", otherPeerID, "other"))
		if err != nil {
			t.Fatal(err)
		}
		_, invalidConversationErr := service.MarkRead(context.Background(), receiverID, senderID, other.Message.ID)
		assertPublicCode(t, invalidConversationErr, message.CodeInvalidReadMessage)
		_, missingMessageErr := service.MarkRead(context.Background(), receiverID, senderID, ^uint64(0))
		assertPublicCode(t, missingMessageErr, message.CodeInvalidReadMessage)

		baseline, err := service.SendPrivateMessage(context.Background(), senderID, integrationRequest("cp5-read-baseline", receiverID, "baseline"))
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var newMessage message.SendResult
		var sendErr, readErr error
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			_, readErr = service.MarkRead(context.Background(), receiverID, senderID, baseline.Message.ID)
		}()
		go func() {
			defer wait.Done()
			<-start
			newMessage, sendErr = service.SendPrivateMessage(context.Background(), senderID, integrationRequest("cp5-read-race", receiverID, "arrived during read"))
		}()
		close(start)
		wait.Wait()
		if sendErr != nil || readErr != nil {
			t.Fatalf("concurrent send/read: send=%v read=%v", sendErr, readErr)
		}
		state, err := repository.GetSession(context.Background(), receiverID, senderID)
		if err != nil || state.UnreadCount != 1 || state.LastReadMessageID == nil || *state.LastReadMessageID != baseline.Message.ID {
			t.Fatalf("concurrent read state = %#v, %v", state, err)
		}
		finalState, err := service.MarkRead(context.Background(), receiverID, senderID, newMessage.Message.ID)
		if err != nil || finalState.UnreadCount != 0 {
			t.Fatalf("final read = %#v, %v", finalState, err)
		}
	})

	t.Run("session list top hidden user scope and reappearance", func(t *testing.T) {
		other, err := service.SendPrivateMessage(context.Background(), senderID, integrationRequest("cp5-session-other", otherPeerID, "newer other session"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.SetSessionTop(context.Background(), senderID, receiverID, true); err != nil {
			t.Fatal(err)
		}
		list, err := service.ListSessions(context.Background(), senderID, 100)
		if err != nil || list.Count != 2 || len(list.Items) != 2 {
			t.Fatalf("session list = %#v, %v", list, err)
		}
		if list.Items[0].Peer.ID != receiverID || !list.Items[0].IsTop || list.Items[1].Peer.ID != otherPeerID || list.Items[1].LastMessage.ID != other.Message.ID {
			t.Fatalf("session order/items = %#v", list.Items)
		}
		if err := service.HideSession(context.Background(), senderID, receiverID); err != nil {
			t.Fatal(err)
		}
		hiddenList, err := service.ListSessions(context.Background(), senderID, 100)
		if err != nil || hiddenList.Count != 1 || hiddenList.Items[0].Peer.ID != otherPeerID {
			t.Fatalf("hidden sender list = %#v, %v", hiddenList, err)
		}
		peerList, err := service.ListSessions(context.Background(), receiverID, 100)
		if err != nil || peerList.Count != 1 || peerList.Items[0].Peer.ID != senderID {
			t.Fatalf("peer session changed by sender hide = %#v, %v", peerList, err)
		}
		reappeared, err := service.SendPrivateMessage(context.Background(), senderID, integrationRequest("cp5-session-reappear", receiverID, "session is visible again"))
		if err != nil {
			t.Fatal(err)
		}
		visibleList, err := service.ListSessions(context.Background(), senderID, 100)
		if err != nil || visibleList.Count != 2 || visibleList.Items[0].Peer.ID != receiverID || !visibleList.Items[0].IsTop || visibleList.Items[0].LastMessage.ID != reappeared.Message.ID || visibleList.Items[0].LastMessage.Preview != "session is visible again" {
			t.Fatalf("reappeared list = %#v, %v", visibleList, err)
		}
		receiverState, err := repository.GetSession(context.Background(), receiverID, senderID)
		if err != nil || receiverState.LastReadMessageID == nil || receiverState.UnreadCount != 1 {
			t.Fatalf("new message did not preserve read cursor/increment unread = %#v, %v", receiverState, err)
		}
		if _, err := service.SetSessionTop(context.Background(), senderID, receiverID, false); err != nil {
			t.Fatal(err)
		}
		toggled, err := service.ToggleSessionTop(context.Background(), senderID, receiverID)
		if err != nil || !toggled.IsTop {
			t.Fatalf("legacy atomic top toggle = %#v, %v", toggled, err)
		}
	})

	t.Run("history cursor isolation deletion and limit", func(t *testing.T) {
		first, err := service.History(context.Background(), senderID, receiverID, 0, 10)
		if err != nil || len(first.Items) != 10 || !first.HasMore || first.NextBeforeMessageID == 0 {
			t.Fatalf("first history page = %#v, %v", first, err)
		}
		assertAscendingMessages(t, first.Items)
		second, err := service.History(context.Background(), senderID, receiverID, first.NextBeforeMessageID, 10)
		if err != nil || len(second.Items) == 0 {
			t.Fatalf("second history page = %#v, %v", second, err)
		}
		assertAscendingMessages(t, second.Items)
		seen := make(map[uint64]struct{}, len(first.Items))
		for _, item := range first.Items {
			seen[item.ID] = struct{}{}
		}
		for _, item := range second.Items {
			if _, duplicate := seen[item.ID]; duplicate {
				t.Fatalf("history cursor duplicated message %d", item.ID)
			}
			if !conversationMatches(item, senderID, receiverID) {
				t.Fatalf("history leaked another conversation: %#v", item)
			}
		}
		_, invalidCursorErr := service.History(context.Background(), senderID, receiverID, otherConversationMessageID(t, rawDB, senderID, otherPeerID), 10)
		assertPublicCode(t, invalidCursorErr, message.CodeInvalidCursor)
		empty, err := service.History(context.Background(), otherPeerID, receiverID, 0, 10)
		if err != nil || len(empty.Items) != 0 {
			t.Fatalf("unrelated history = %#v, %v", empty, err)
		}
		_, excessiveLimitErr := service.History(context.Background(), senderID, receiverID, 0, message.MaxHistoryLimit+1)
		assertPublicCode(t, excessiveLimitErr, message.CodeInvalidChatQuery)

		allForReceiver, err := service.History(context.Background(), receiverID, senderID, 0, 100)
		if err != nil || len(allForReceiver.Items) == 0 {
			t.Fatalf("receiver history = %#v, %v", allForReceiver, err)
		}
		deletedID := allForReceiver.Items[len(allForReceiver.Items)-1].ID
		if _, err := rawDB.Exec(`INSERT INTO chat_message_deletions(user_id, message_id) VALUES (?, ?)`, receiverID, deletedID); err != nil {
			t.Fatal(err)
		}
		receiverHistory, err := service.History(context.Background(), receiverID, senderID, 0, 100)
		if err != nil || containsMessage(receiverHistory.Items, deletedID) {
			t.Fatalf("receiver deletion filter = %#v, %v", receiverHistory, err)
		}
		senderHistory, err := service.History(context.Background(), senderID, receiverID, 0, 100)
		if err != nil || !containsMessage(senderHistory.Items, deletedID) {
			t.Fatalf("sender lost peer-deleted message = %#v, %v", senderHistory, err)
		}
	})

	t.Run("transaction failures rollback and publish only after commit", func(t *testing.T) {
		failureSender, failureReceiver := seed+10, seed+11
		seedUserPair(t, rawDB, failureSender, failureReceiver)
		failureRecipients := integrationRecipients{active: map[uint64]bool{failureSender: true, failureReceiver: true}}
		failureService, err := message.NewService(repository, failureRecipients, chatmysql.NewFriendshipRepository(database), message.DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}

		mustCreateTrigger(t, rawDB, "cp5_fail_message", `BEFORE INSERT ON chat_messages FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected message failure'`)
		assertInboundFailure(t, failureService, failureSender, failureReceiver, "cp5-fail-message")
		mustDropTrigger(t, rawDB, "cp5_fail_message")
		assertPairAbsent(t, database, failureSender, failureReceiver, "cp5-fail-message")

		mustCreateTrigger(t, rawDB, "cp5_fail_sender_session", fmt.Sprintf(`BEFORE INSERT ON chat_sessions FOR EACH ROW BEGIN IF NEW.user_id = %d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected sender session failure'; END IF; END`, failureSender))
		assertInboundFailure(t, failureService, failureSender, failureReceiver, "cp5-fail-sender")
		mustDropTrigger(t, rawDB, "cp5_fail_sender_session")
		assertPairAbsent(t, database, failureSender, failureReceiver, "cp5-fail-sender")

		mustCreateTrigger(t, rawDB, "cp5_fail_receiver_session", fmt.Sprintf(`BEFORE INSERT ON chat_sessions FOR EACH ROW BEGIN IF NEW.user_id = %d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected receiver session failure'; END IF; END`, failureReceiver))
		assertInboundFailure(t, failureService, failureSender, failureReceiver, "cp5-fail-receiver")
		mustDropTrigger(t, rawDB, "cp5_fail_receiver_session")
		assertPairAbsent(t, database, failureSender, failureReceiver, "cp5-fail-receiver")

		sender := &integrationCaptureSender{}
		publisher := &integrationCapturePublisher{err: errors.New("redis unavailable")}
		inbound, err := message.NewInbound(failureService, sender, publisher, log.New(io.Discard, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		handleIntegrationInbound(t, inbound, failureSender, failureReceiver, "cp5-redis-degraded")
		if sender.countEvent(t, protocol.EventChatAck) != 1 || len(publisher.events) != 1 {
			t.Fatalf("redis degraded ack=%d publishes=%d", sender.countEvent(t, protocol.EventChatAck), len(publisher.events))
		}
		assertMessageCount(t, database, failureSender, "cp5-redis-degraded", 1)
		assertSessionState(t, repository, failureReceiver, failureSender, publisher.events[0].MessageID, 1)
	})

	t.Run("actual query plans expose designed indexes", func(t *testing.T) {
		plans := []struct {
			name, index, query string
		}{
			{"session list", "ix_chat_sessions_user_sort", fmt.Sprintf(`SELECT * FROM chat_sessions WHERE user_id = %d AND hidden_at IS NULL ORDER BY is_top DESC, last_message_at DESC, id DESC LIMIT 20`, senderID)},
			{"history cursor", "ix_chat_messages_conversation_cursor", fmt.Sprintf(`SELECT * FROM chat_messages WHERE conversation_low_id = %d AND conversation_high_id = %d AND id < 18446744073709551615 ORDER BY id DESC LIMIT 20`, senderID, receiverID)},
			{"unread count", "ix_chat_messages_receiver_sender_cursor", fmt.Sprintf(`SELECT COUNT(*) FROM chat_messages WHERE rev_user_id = %d AND send_user_id = %d AND id > 0`, receiverID, senderID)},
		}
		for _, testCase := range plans {
			plan := explainIndex(t, context.Background(), rawDB, testCase.query)
			if !strings.Contains(plan, testCase.index) {
				t.Fatalf("EXPLAIN %s did not expose %s: %s", testCase.name, testCase.index, plan)
			}
			t.Logf("EXPLAIN %s index=%s plan=%s", testCase.name, testCase.index, plan)
		}
	})
}

type integrationCaptureSender struct {
	mu       sync.Mutex
	payloads [][]byte
}

func (sender *integrationCaptureSender) SendToClient(_ context.Context, _ uint64, _ string, payload []byte) (chat.DeliveryResult, error) {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.payloads = append(sender.payloads, append([]byte(nil), payload...))
	return chat.DeliveryResult{Delivered: 1}, nil
}

func (sender *integrationCaptureSender) countEvent(t *testing.T, event string) int {
	t.Helper()
	sender.mu.Lock()
	defer sender.mu.Unlock()
	count := 0
	for _, payload := range sender.payloads {
		var envelope protocol.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Event == event {
			count++
		}
	}
	return count
}

type integrationCapturePublisher struct {
	events []delivery.Event
	err    error
}

func (publisher *integrationCapturePublisher) Publish(_ context.Context, event delivery.Event) error {
	publisher.events = append(publisher.events, event)
	return publisher.err
}

func assertInboundFailure(t *testing.T, service *message.Service, senderID, receiverID uint64, clientMsgID string) {
	t.Helper()
	sender := &integrationCaptureSender{}
	publisher := &integrationCapturePublisher{}
	inbound, err := message.NewInbound(service, sender, publisher, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	handleIntegrationInbound(t, inbound, senderID, receiverID, clientMsgID)
	if sender.countEvent(t, protocol.EventChatAck) != 0 || sender.countEvent(t, protocol.EventError) != 1 || len(publisher.events) != 0 {
		t.Fatalf("failure path ack=%d error=%d publishes=%d", sender.countEvent(t, protocol.EventChatAck), sender.countEvent(t, protocol.EventError), len(publisher.events))
	}
}

func handleIntegrationInbound(t *testing.T, inbound *message.Inbound, senderID, receiverID uint64, clientMsgID string) {
	t.Helper()
	data, err := protocol.Data(protocol.ChatSendData{
		ReceiverID: receiverID,
		Message:    protocol.MessagePayload{Type: message.TextMessageType, TextMsg: &protocol.TextMessage{Content: clientMsgID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := inbound.Handle(context.Background(), chat.ClientIdentity{UserID: senderID, ClientID: "integration"}, protocol.Envelope{
		Event: protocol.EventChatSend, ClientMsgID: clientMsgID, Data: data,
	}); err != nil {
		t.Fatal(err)
	}
}

func mustCreateTrigger(t *testing.T, db *sql.DB, name, definition string) {
	t.Helper()
	if _, err := db.Exec("CREATE TRIGGER `" + name + "` " + definition); err != nil {
		t.Fatalf("create trigger %s: %v", name, err)
	}
}

func mustDropTrigger(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	if _, err := db.Exec("DROP TRIGGER IF EXISTS `" + name + "`"); err != nil {
		t.Fatalf("drop trigger %s: %v", name, err)
	}
}

func seedUserPair(t *testing.T, db *sql.DB, senderID, receiverID uint64) {
	t.Helper()
	for _, id := range []uint64{senderID, receiverID} {
		account := fmt.Sprintf("chat.failure.%d", id)
		if _, err := db.Exec(`INSERT INTO users(id, account, pwd_hash, nickname, role, status) VALUES (?, ?, 'integration-hash', ?, 2, 1)`, id, account, account); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO friends(user_id, friend_id) VALUES (?, ?)`, senderID, receiverID); err != nil {
		t.Fatal(err)
	}
}

func assertPairAbsent(t *testing.T, database *gorm.DB, senderID, receiverID uint64, clientMsgID string) {
	t.Helper()
	var messages, sessions int64
	if err := database.Table("chat_messages").Where("send_user_id = ? AND client_msg_id = ?", senderID, clientMsgID).Count(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Table("chat_sessions").Where("(user_id = ? AND peer_user_id = ?) OR (user_id = ? AND peer_user_id = ?)", senderID, receiverID, receiverID, senderID).Count(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if messages != 0 || sessions != 0 {
		t.Fatalf("rolled back pair messages=%d sessions=%d", messages, sessions)
	}
}

func assertMessageCount(t *testing.T, database *gorm.DB, senderID uint64, clientMsgID string, want int64) {
	t.Helper()
	var count int64
	if err := database.Table("chat_messages").Where("send_user_id = ? AND client_msg_id = ?", senderID, clientMsgID).Count(&count).Error; err != nil || count != want {
		t.Fatalf("message count = %d, want %d, err=%v", count, want, err)
	}
}

func assertSessionState(t *testing.T, repository *chatmysql.Repository, userID, peerID, messageID uint64, unread uint32) {
	t.Helper()
	session, err := repository.GetSession(context.Background(), userID, peerID)
	if err != nil || session.LastMessageID != messageID || session.UnreadCount != unread {
		t.Fatalf("session %d->%d = %#v, err=%v; want message=%d unread=%d", userID, peerID, session, err, messageID, unread)
	}
}

func incomingMessageIDs(t *testing.T, db *sql.DB, senderID, receiverID uint64) []uint64 {
	t.Helper()
	rows, err := db.Query(`SELECT id FROM chat_messages WHERE send_user_id = ? AND rev_user_id = ? ORDER BY id`, senderID, receiverID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func otherConversationMessageID(t *testing.T, db *sql.DB, senderID, peerID uint64) uint64 {
	t.Helper()
	var id uint64
	if err := db.QueryRow(`SELECT id FROM chat_messages WHERE send_user_id = ? AND rev_user_id = ? ORDER BY id DESC LIMIT 1`, senderID, peerID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertPublicCode(t *testing.T, err error, want uint32) {
	t.Helper()
	var publicError *message.PublicError
	if !errors.As(err, &publicError) || publicError.Code != want {
		t.Fatalf("error = %v, want public code %d", err, want)
	}
}

func assertAscendingMessages(t *testing.T, items []message.Message) {
	t.Helper()
	for index := 1; index < len(items); index++ {
		if items[index-1].ID >= items[index].ID {
			t.Fatalf("history is not ascending: %d then %d", items[index-1].ID, items[index].ID)
		}
	}
}

func conversationMatches(item message.Message, first, second uint64) bool {
	return item.SenderID == first && item.ReceiverID == second || item.SenderID == second && item.ReceiverID == first
}

func containsMessage(items []message.Message, messageID uint64) bool {
	for _, item := range items {
		if item.ID == messageID {
			return true
		}
	}
	return false
}
