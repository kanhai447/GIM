package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	groupconfig "github.com/kanhai447/GIM/server/internal/group/config"
	"github.com/kanhai447/GIM/server/internal/group/delivery"
	"github.com/kanhai447/GIM/server/internal/group/domain"
	groupmessage "github.com/kanhai447/GIM/server/internal/group/message"
	groupmessagemysql "github.com/kanhai447/GIM/server/internal/group/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/group/ws"
	groupprotocol "github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"gorm.io/gorm"
)

type groupMessagePolicy struct {
	group   domain.Group
	members map[uint64]bool
	muted   map[uint64]bool
}

func (p *groupMessagePolicy) GetGroupByID(context.Context, uint64) (domain.Group, error) {
	return p.group, nil
}
func (p *groupMessagePolicy) IsMember(_ context.Context, _ uint64, userID uint64) (bool, error) {
	return p.members[userID], nil
}
func (p *groupMessagePolicy) IsMuted(_ context.Context, _ uint64, userID uint64) (bool, error) {
	return p.muted[userID], nil
}
func (p *groupMessagePolicy) GetMemberRole(_ context.Context, _ uint64, userID uint64) (domain.GroupRole, error) {
	if userID == p.group.OwnerID {
		return domain.GroupRoleOwner, nil
	}
	return domain.GroupRoleMember, nil
}

func TestGroupMessageReliabilitySessionsHistoryAndDelivery(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Group message integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	database, rawDB := newChatMessageDatabase(t, values)
	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 70_000)
	owner, member, third := seed, seed+1, seed+2
	for _, id := range []uint64{owner, member, third} {
		account := fmt.Sprintf("group.message.%d", id)
		if _, err := rawDB.Exec(`INSERT INTO users(id,account,pwd_hash,nickname,role,status) VALUES (?,?,'integration-hash',?,2,1)`, id, account, account); err != nil {
			t.Fatal(err)
		}
	}
	result, err := rawDB.Exec("INSERT INTO `groups`(name,owner_id,status,join_policy,member_limit) VALUES ('Group Message Integration',?,1,1,100)", owner)
	if err != nil {
		t.Fatal(err)
	}
	groupIDRaw, _ := result.LastInsertId()
	groupID := uint64(groupIDRaw)
	for _, id := range []uint64{owner, member, third} {
		role := 3
		if id == owner {
			role = 1
		}
		if _, err := rawDB.Exec(`INSERT INTO group_members(group_id,user_id,role) VALUES (?,?,?)`, groupID, id, role); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = rawDB.Exec(`DELETE FROM group_message_deletions WHERE group_id=?`, groupID)
		_, _ = rawDB.Exec(`DELETE FROM group_sessions WHERE group_id=?`, groupID)
		_, _ = rawDB.Exec(`DELETE FROM group_messages WHERE group_id=?`, groupID)
		_, _ = rawDB.Exec(`DELETE FROM group_members WHERE group_id=?`, groupID)
		_, _ = rawDB.Exec(`DELETE FROM groups WHERE id=?`, groupID)
		_, _ = rawDB.Exec(`DELETE FROM users WHERE id IN (?,?,?)`, owner, member, third)
	})
	policy := &groupMessagePolicy{group: domain.Group{ID: groupID, OwnerID: owner, Status: domain.GroupStatusActive}, members: map[uint64]bool{owner: true, member: true, third: true}, muted: map[uint64]bool{}}
	repository := groupmessagemysql.New(database)
	service, err := groupmessage.NewService(repository, policy, groupmessage.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("fifty duplicate sends converge without unread duplication", func(t *testing.T) {
		const attempts = 50
		ids := make(chan uint64, attempts)
		errs := make(chan error, attempts)
		var wait sync.WaitGroup
		for range attempts {
			wait.Add(1)
			go func() {
				defer wait.Done()
				sent, sendErr := service.Send(context.Background(), owner, groupRequest(groupID, "group-duplicate", "only once"))
				if sendErr != nil {
					errs <- sendErr
					return
				}
				ids <- sent.Message.ID
			}()
		}
		wait.Wait()
		close(ids)
		close(errs)
		for sendErr := range errs {
			t.Fatalf("duplicate send: public=%v private=%v", sendErr, errors.Unwrap(sendErr))
		}
		var first uint64
		for id := range ids {
			if first == 0 {
				first = id
			}
			if id != first {
				t.Fatalf("duplicate IDs %d/%d", first, id)
			}
		}
		assertGroupSession(t, database, owner, groupID, first, 0)
		assertGroupSession(t, database, member, groupID, first, 1)
		assertGroupSession(t, database, third, groupID, first, 1)
	})

	t.Run("fifty distinct sends preserve atomic unread", func(t *testing.T) {
		const attempts = 50
		ids := make(chan uint64, attempts)
		errs := make(chan error, attempts)
		var wait sync.WaitGroup
		for index := range attempts {
			wait.Add(1)
			go func(index int) {
				defer wait.Done()
				sent, sendErr := service.Send(context.Background(), owner, groupRequest(groupID, fmt.Sprintf("group-distinct-%02d", index), fmt.Sprintf("distinct-%02d", index)))
				if sendErr != nil {
					errs <- sendErr
					return
				}
				ids <- sent.Message.ID
			}(index)
		}
		wait.Wait()
		close(ids)
		close(errs)
		for sendErr := range errs {
			t.Fatal(sendErr)
		}
		all := make([]uint64, 0, attempts)
		for id := range ids {
			all = append(all, id)
		}
		sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
		last := all[len(all)-1]
		assertGroupSession(t, database, owner, groupID, last, 0)
		assertGroupSession(t, database, member, groupID, last, 51)
		assertGroupSession(t, database, third, groupID, last, 51)
		_, conflict := service.Send(context.Background(), owner, groupRequest(groupID, "group-duplicate", "changed"))
		assertGroupPublicCode(t, conflict, groupmessage.CodeIdempotencyConflict)
	})

	t.Run("mute membership read and history semantics", func(t *testing.T) {
		sessions, listErr := service.ListSessions(context.Background(), member, 20)
		if listErr != nil || sessions.Count != 1 || len(sessions.Items) != 1 || sessions.Items[0].Group.ID != groupID || sessions.Items[0].LastMessage.ID == 0 {
			t.Fatalf("group session list=%#v err=%v", sessions, listErr)
		}
		policy.group.IsAllMuted = true
		_, allMuteErr := service.Send(context.Background(), member, groupRequest(groupID, "all-muted", "no"))
		assertGroupPublicCode(t, allMuteErr, groupmessage.CodeMuted)
		if _, ownerErr := service.Send(context.Background(), owner, groupRequest(groupID, "all-muted-owner", "manager allowed")); ownerErr != nil {
			t.Fatalf("owner blocked by all mute: %v", ownerErr)
		}
		policy.group.IsAllMuted = false
		policy.muted[member] = true
		_, muteErr := service.Send(context.Background(), member, groupRequest(groupID, "muted", "no"))
		assertGroupPublicCode(t, muteErr, groupmessage.CodeMuted)
		policy.muted[member] = false
		policy.members[member] = false
		_, memberErr := service.Send(context.Background(), member, groupRequest(groupID, "left", "no"))
		assertGroupPublicCode(t, memberErr, groupmessage.CodeNotMember)
		_, historyErr := service.History(context.Background(), member, groupID, 0, 20)
		assertGroupPublicCode(t, historyErr, groupmessage.CodeNotMember)
		policy.members[member] = true
		var ids []uint64
		if err := database.Table("group_messages").Where("group_id=? AND send_user_id=?", groupID, owner).Order("id ASC").Pluck("id", &ids).Error; err != nil {
			t.Fatal(err)
		}
		partial := ids[len(ids)-2]
		state, err := service.MarkRead(context.Background(), member, groupID, partial)
		if err != nil || state.UnreadCount != 1 {
			t.Fatalf("partial read %#v %v", state, err)
		}
		stale, err := service.MarkRead(context.Background(), member, groupID, ids[0])
		if err != nil || stale.UnreadCount != 1 || *stale.LastReadMessageID != partial {
			t.Fatalf("stale read %#v %v", stale, err)
		}
		latest, err := service.MarkRead(context.Background(), member, groupID, ids[len(ids)-1])
		if err != nil || latest.UnreadCount != 0 {
			t.Fatalf("latest read %#v %v", latest, err)
		}
		first, err := service.History(context.Background(), member, groupID, 0, 10)
		if err != nil || len(first.Items) != 10 || !first.HasMore {
			t.Fatalf("history first %#v %v", first, err)
		}
		second, err := service.History(context.Background(), member, groupID, first.NextBeforeMessageID, 10)
		if err != nil || len(second.Items) == 0 {
			t.Fatalf("history second %#v %v", second, err)
		}
		deleted := first.Items[len(first.Items)-1].ID
		if _, err := rawDB.Exec(`INSERT INTO group_message_deletions(user_id,message_id,group_id) VALUES (?,?,?)`, member, deleted, groupID); err != nil {
			t.Fatal(err)
		}
		memberHistory, _ := service.History(context.Background(), member, groupID, 0, 100)
		ownerHistory, _ := service.History(context.Background(), owner, groupID, 0, 100)
		if containsGroupMessage(memberHistory.Items, deleted) || !containsGroupMessage(ownerHistory.Items, deleted) {
			t.Fatal("group history deletion isolation failed")
		}
	})

	t.Run("session failure rolls back and publish failure keeps ack", func(t *testing.T) {
		failureID := seed + 20
		account := fmt.Sprintf("group.message.%d", failureID)
		if _, err := rawDB.Exec(`INSERT INTO users(id,account,pwd_hash,nickname,role,status) VALUES (?,?,'integration-hash',?,2,1)`, failureID, account, account); err != nil {
			t.Fatal(err)
		}
		if _, err := rawDB.Exec(`INSERT INTO group_members(group_id,user_id,role) VALUES (?,?,3)`, groupID, failureID); err != nil {
			t.Fatal(err)
		}
		policy.members[failureID] = true
		t.Cleanup(func() { _, _ = rawDB.Exec(`DELETE FROM users WHERE id=?`, failureID) })
		failureSender := &groupCaptureSender{}
		failurePublisher := &groupFailPublisher{}
		failureInbound, _ := groupmessage.NewInbound(service, failureSender, failurePublisher, log.New(io.Discard, "", 0))
		mustCreateTrigger(t, rawDB, "group_cp3_fail_message", `BEFORE INSERT ON group_messages FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected group message failure'`)
		handleGroupInbound(t, failureInbound, owner, groupID, "group-fail-message")
		mustDropTrigger(t, rawDB, "group_cp3_fail_message")
		if failureSender.eventCount(t, groupprotocol.EventGroupAck) != 0 || failureSender.eventCount(t, groupprotocol.EventError) != 1 || len(failurePublisher.events) != 0 {
			t.Fatal("message failure emitted ACK or publish")
		}
		mustCreateTrigger(t, rawDB, "group_cp3_fail_session", fmt.Sprintf(`BEFORE INSERT ON group_sessions FOR EACH ROW BEGIN IF NEW.user_id = %d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected group session failure'; END IF; END`, failureID))
		failureSender = &groupCaptureSender{}
		failurePublisher = &groupFailPublisher{}
		failureInbound, _ = groupmessage.NewInbound(service, failureSender, failurePublisher, log.New(io.Discard, "", 0))
		handleGroupInbound(t, failureInbound, owner, groupID, "group-fail-session")
		mustDropTrigger(t, rawDB, "group_cp3_fail_session")
		if failureSender.eventCount(t, groupprotocol.EventGroupAck) != 0 || failureSender.eventCount(t, groupprotocol.EventError) != 1 || len(failurePublisher.events) != 0 {
			t.Fatal("session failure emitted ACK or publish")
		}
		var count int64
		_ = database.Table("group_messages").Where("send_user_id=? AND client_msg_id=?", owner, "group-fail-session").Count(&count).Error
		if count != 0 {
			t.Fatal("failed group session left message")
		}
		sender := &groupCaptureSender{}
		publisher := &groupFailPublisher{err: errors.New("redis unavailable")}
		inbound, _ := groupmessage.NewInbound(service, sender, publisher, log.New(io.Discard, "", 0))
		data, _ := groupprotocol.Data(groupprotocol.GroupSendData{GroupID: groupID, Message: groupprotocol.MessagePayload{Type: 1, TextMsg: &groupprotocol.TextMessage{Content: "redis degraded"}}})
		if err := inbound.Handle(context.Background(), ws.ClientIdentity{UserID: owner, ClientID: "origin"}, groupprotocol.Envelope{Event: groupprotocol.EventGroupSend, ClientMsgID: "group-redis-degraded", Data: data}); err != nil {
			t.Fatal(err)
		}
		if sender.eventCount(t, groupprotocol.EventGroupAck) != 1 || len(publisher.events) != 1 {
			t.Fatalf("ack=%d publish=%d", sender.eventCount(t, groupprotocol.EventGroupAck), len(publisher.events))
		}
	})

	t.Run("redis channel fans out to every instance without Presence mutation", func(t *testing.T) {
		redisConfig, err := platformredis.FromValues(values)
		if err != nil {
			t.Fatal(err)
		}
		client, err := platformredis.Open(context.Background(), redisConfig)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		before := redisKeys(t, client.Raw(), "gim:presence:*")
		busOne, _ := delivery.New(client.Raw(), time.Second, log.New(io.Discard, "", 0))
		busTwo, _ := delivery.New(client.Raw(), time.Second, log.New(io.Discard, "", 0))
		receivedOne := make(chan delivery.Event, 1)
		receivedTwo := make(chan delivery.Event, 1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		subOne, err := busOne.Subscribe(ctx, func(_ context.Context, event delivery.Event) { receivedOne <- event })
		if err != nil {
			t.Fatal(err)
		}
		defer subOne.Close()
		subTwo, err := busTwo.Subscribe(ctx, func(_ context.Context, event delivery.Event) { receivedTwo <- event })
		if err != nil {
			t.Fatal(err)
		}
		defer subTwo.Close()
		go subOne.Run(ctx)
		go subTwo.Run(ctx)
		event := delivery.Event{MessageID: 999, TargetUserIDs: []uint64{owner, member}, Payload: json.RawMessage(`{"event":"group.message"}`)}
		if err := busOne.Publish(ctx, event); err != nil {
			t.Fatal(err)
		}
		for index, ch := range []chan delivery.Event{receivedOne, receivedTwo} {
			select {
			case got := <-ch:
				if got.MessageID != event.MessageID || len(got.TargetUserIDs) != 2 {
					t.Fatalf("subscription %d got %#v", index, got)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("subscription %d timeout", index)
			}
		}
		after := redisKeys(t, client.Raw(), "gim:presence:*")
		if strings.Join(before, ",") != strings.Join(after, ",") {
			t.Fatal("group delivery changed Presence")
		}
	})

	t.Run("actual websocket multi instance and multi device delivery", func(t *testing.T) {
		redisConfig, err := platformredis.FromValues(values)
		if err != nil {
			t.Fatal(err)
		}
		client, err := platformredis.Open(context.Background(), redisConfig)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		busA, _ := delivery.New(client.Raw(), time.Second, log.New(io.Discard, "", 0))
		busB, _ := delivery.New(client.Raw(), time.Second, log.New(io.Discard, "", 0))
		configuration := groupconfig.WebSocketConfig{Path: "/api/group/ws/chat", SendBuffer: 32, AllowedOrigins: []string{"http://gim.integration"}, ReadLimit: 1 << 20, PongWait: 5 * time.Second, PingPeriod: 4 * time.Second, WriteWait: time.Second}
		moduleA, err := ws.NewWithInboundFactory(configuration, func(hub *ws.Hub) (ws.InboundHandler, error) {
			return groupmessage.NewInbound(service, hub, busA, log.New(io.Discard, "", 0))
		})
		if err != nil {
			t.Fatal(err)
		}
		moduleB, err := ws.NewWithInboundFactory(configuration, func(hub *ws.Hub) (ws.InboundHandler, error) {
			return groupmessage.NewInbound(service, hub, busB, log.New(io.Discard, "", 0))
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		subA, err := busA.Subscribe(ctx, groupmessage.LocalDeliveryHandler(moduleA.Hub, time.Second, log.New(io.Discard, "", 0)))
		if err != nil {
			t.Fatal(err)
		}
		defer subA.Close()
		subB, err := busB.Subscribe(ctx, groupmessage.LocalDeliveryHandler(moduleB.Hub, time.Second, log.New(io.Discard, "", 0)))
		if err != nil {
			t.Fatal(err)
		}
		defer subB.Close()
		go moduleA.Hub.Run(ctx)
		go moduleB.Hub.Run(ctx)
		go subA.Run(ctx)
		go subB.Run(ctx)
		serverA := httptest.NewServer(moduleA.Handler)
		defer serverA.Close()
		serverB := httptest.NewServer(moduleB.Handler)
		defer serverB.Close()
		dial := func(serverURL string, userID uint64) *websocket.Conn {
			header := http.Header{"Origin": []string{"http://gim.integration"}, "User-ID": []string{fmt.Sprint(userID)}, "Role": []string{"2"}}
			connection, response, dialErr := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(serverURL, "http"), header)
			if dialErr != nil {
				if response != nil {
					t.Fatalf("dial status=%d err=%v", response.StatusCode, dialErr)
				}
				t.Fatal(dialErr)
			}
			t.Cleanup(func() { _ = connection.Close() })
			return connection
		}
		origin := dial(serverA.URL, owner)
		sibling := dial(serverB.URL, owner)
		memberOne := dial(serverB.URL, member)
		memberTwo := dial(serverB.URL, member)
		data, _ := groupprotocol.Data(groupprotocol.GroupSendData{GroupID: groupID, Message: groupprotocol.MessagePayload{Type: 1, TextMsg: &groupprotocol.TextMessage{Content: "cross instance"}}})
		frame, _ := groupprotocol.Encode(groupprotocol.Envelope{Event: groupprotocol.EventGroupSend, ClientMsgID: "group-ws-cross-instance", Data: data})
		if err := origin.WriteMessage(websocket.TextMessage, frame); err != nil {
			t.Fatal(err)
		}
		originEvents := readGroupEvents(t, origin, 2)
		if originEvents[groupprotocol.EventGroupAck] != 1 || originEvents[groupprotocol.EventGroupMessage] != 1 {
			t.Fatalf("origin events=%v", originEvents)
		}
		for name, connection := range map[string]*websocket.Conn{"sibling": sibling, "member-one": memberOne, "member-two": memberTwo} {
			events := readGroupEvents(t, connection, 1)
			if events[groupprotocol.EventGroupMessage] != 1 || events[groupprotocol.EventGroupAck] != 0 {
				t.Fatalf("%s events=%v", name, events)
			}
		}
	})

	t.Run("query plans expose designed indexes", func(t *testing.T) {
		plans := []struct{ name, index, query string }{{"session", "ix_group_sessions_user_sort", fmt.Sprintf(`SELECT * FROM group_sessions WHERE user_id=%d AND hidden_at IS NULL ORDER BY is_top DESC,last_message_at DESC,id DESC LIMIT 20`, member)}, {"history", "ix_group_messages_group_cursor", fmt.Sprintf(`SELECT * FROM group_messages WHERE group_id=%d AND id<18446744073709551615 ORDER BY id DESC LIMIT 20`, groupID)}, {"membership", "ux_group_members_group_user", fmt.Sprintf(`SELECT id FROM group_members WHERE group_id=%d AND user_id=%d`, groupID, member)}}
		for _, item := range plans {
			plan := explainIndex(t, context.Background(), rawDB, item.query)
			if !strings.Contains(plan, item.index) {
				t.Fatalf("EXPLAIN %s missing %s: %s", item.name, item.index, plan)
			}
		}
	})
}

func groupRequest(groupID uint64, clientID, content string) groupmessage.SendRequest {
	return groupmessage.SendRequest{GroupID: groupID, ClientMsgID: clientID, Message: groupprotocol.MessagePayload{Type: 1, TextMsg: &groupprotocol.TextMessage{Content: content}}}
}
func assertGroupSession(t *testing.T, db *gorm.DB, userID, groupID, lastID uint64, unread uint32) {
	t.Helper()
	var row struct {
		Last   uint64 `gorm:"column:last_message_id"`
		Unread uint32 `gorm:"column:unread_count"`
	}
	if err := db.Table("group_sessions").Select("last_message_id,unread_count").Where("user_id=? AND group_id=?", userID, groupID).Take(&row).Error; err != nil || row.Last != lastID || row.Unread != unread {
		t.Fatalf("group session user=%d row=%#v err=%v want last=%d unread=%d", userID, row, err, lastID, unread)
	}
}
func assertGroupPublicCode(t *testing.T, err error, want uint32) {
	t.Helper()
	var public *groupmessage.PublicError
	if !errors.As(err, &public) || public.Code != want {
		t.Fatalf("error=%v want code=%d", err, want)
	}
}
func containsGroupMessage(items []groupmessage.Message, id uint64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

type groupCaptureSender struct {
	mu       sync.Mutex
	payloads [][]byte
}

func (s *groupCaptureSender) SendToClient(_ context.Context, _ ws.ClientIdentity, payload []byte) (ws.DeliveryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads = append(s.payloads, append([]byte(nil), payload...))
	return ws.DeliveryResult{Delivered: 1}, nil
}
func (s *groupCaptureSender) eventCount(t *testing.T, event string) int {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, payload := range s.payloads {
		var envelope groupprotocol.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Event == event {
			count++
		}
	}
	return count
}

type groupFailPublisher struct {
	events []delivery.Event
	err    error
}

func readGroupEvents(t *testing.T, connection *websocket.Conn, count int) map[string]int {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	events := map[string]int{}
	for range count {
		_, payload, err := connection.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var envelope groupprotocol.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		events[envelope.Event]++
	}
	return events
}

func (p *groupFailPublisher) Publish(_ context.Context, event delivery.Event) error {
	p.events = append(p.events, event)
	return p.err
}

func handleGroupInbound(t *testing.T, inbound *groupmessage.Inbound, senderID, groupID uint64, clientMsgID string) {
	t.Helper()
	data, _ := groupprotocol.Data(groupprotocol.GroupSendData{GroupID: groupID, Message: groupprotocol.MessagePayload{Type: 1, TextMsg: &groupprotocol.TextMessage{Content: clientMsgID}}})
	if err := inbound.Handle(context.Background(), ws.ClientIdentity{UserID: senderID, ClientID: "integration"}, groupprotocol.Envelope{Event: groupprotocol.EventGroupSend, ClientMsgID: clientMsgID, Data: data}); err != nil {
		t.Fatal(err)
	}
}
