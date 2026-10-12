package http

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/message"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

type applicationStub struct {
	sessions                         message.SessionList
	history                          message.HistoryPage
	read                             message.Session
	lastUser, lastGroup, lastMessage uint64
}

func (stub *applicationStub) ListSessions(_ context.Context, userID uint64, _ int) (message.SessionList, error) {
	stub.lastUser = userID
	return stub.sessions, nil
}
func (stub *applicationStub) History(_ context.Context, userID, groupID, _ uint64, _ int) (message.HistoryPage, error) {
	stub.lastUser, stub.lastGroup = userID, groupID
	return stub.history, nil
}
func (stub *applicationStub) MarkRead(_ context.Context, userID, groupID, messageID uint64) (message.Session, error) {
	stub.lastUser, stub.lastGroup, stub.lastMessage = userID, groupID, messageID
	return stub.read, nil
}

func TestGroupMessageHTTPHandlers(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	payload := protocol.MessagePayload{Type: 1, TextMsg: &protocol.TextMessage{Content: "hello"}}
	last := message.Message{ID: 9, GroupID: 5, SenderID: 2, ClientMsgID: "http-group", Type: 1, Payload: payload, CreatedAt: now}
	stub := &applicationStub{sessions: message.SessionList{Items: []message.Session{{UserID: 3, Group: message.GroupInfo{ID: 5, Name: "group"}, LastMessage: last, LastMessageID: 9, LastMessageAt: now, UnreadCount: 1}}, Count: 1}, history: message.HistoryPage{Items: []message.Message{last}}, read: message.Session{UserID: 3, Group: message.GroupInfo{ID: 5}, LastMessageID: 9, LastMessageAt: now}}
	handler := &Handler{application: stub}

	sessionsRequest := httptest.NewRequest("GET", "/api/group/session?limit=20", nil)
	sessionsRequest.Header.Set("User-ID", "3")
	sessionsRequest.Header.Set("Role", "2")
	sessionsResponse := httptest.NewRecorder()
	handler.Sessions(sessionsResponse, sessionsRequest)
	if sessionsResponse.Code != 200 || stub.lastUser != 3 || !bytes.Contains(sessionsResponse.Body.Bytes(), []byte(`"unreadCount":1`)) {
		t.Fatalf("sessions status=%d body=%s", sessionsResponse.Code, sessionsResponse.Body.String())
	}

	historyRequest := httptest.NewRequest("GET", "/api/group/history/5?limit=20", nil)
	historyRequest = pathvar.WithVars(historyRequest, map[string]string{"id": "5"})
	historyRequest.Header.Set("User-ID", "3")
	historyRequest.Header.Set("Role", "2")
	historyResponse := httptest.NewRecorder()
	handler.History(historyResponse, historyRequest)
	if historyResponse.Code != 200 || stub.lastGroup != 5 || !bytes.Contains(historyResponse.Body.Bytes(), []byte(`"messageId":9`)) {
		t.Fatalf("history status=%d body=%s", historyResponse.Code, historyResponse.Body.String())
	}

	readRequest := httptest.NewRequest("PUT", "/api/group/read", bytes.NewBufferString(`{"groupID":5,"lastReadMessageID":9}`))
	readRequest.Header.Set("User-ID", "3")
	readRequest.Header.Set("Role", "2")
	readResponse := httptest.NewRecorder()
	handler.Read(readResponse, readRequest)
	if readResponse.Code != 200 || stub.lastMessage != 9 {
		t.Fatalf("read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
}
