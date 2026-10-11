package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/message"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
)

type applicationStub struct {
	userID   uint64
	peerID   uint64
	before   uint64
	limit    int
	readID   uint64
	top      bool
	toggled  bool
	sessions message.SessionList
	history  message.HistoryPage
	session  message.Session
	err      error
}

func (stub *applicationStub) ListSessions(_ context.Context, userID uint64, limit int) (message.SessionList, error) {
	stub.userID, stub.limit = userID, limit
	return stub.sessions, stub.err
}

func (stub *applicationStub) History(_ context.Context, userID, peerID, before uint64, limit int) (message.HistoryPage, error) {
	stub.userID, stub.peerID, stub.before, stub.limit = userID, peerID, before, limit
	return stub.history, stub.err
}

func (stub *applicationStub) MarkRead(_ context.Context, userID, peerID, readID uint64) (message.Session, error) {
	stub.userID, stub.peerID, stub.readID = userID, peerID, readID
	return stub.session, stub.err
}

func (stub *applicationStub) SetSessionTop(_ context.Context, userID, peerID uint64, top bool) (message.Session, error) {
	stub.userID, stub.peerID, stub.top = userID, peerID, top
	return stub.session, stub.err
}

func (stub *applicationStub) ToggleSessionTop(_ context.Context, userID, peerID uint64) (message.Session, error) {
	stub.userID, stub.peerID, stub.toggled = userID, peerID, true
	return stub.session, stub.err
}

func TestHistoryUsesTrustedIdentityAndMessageIDCursor(t *testing.T) {
	now := time.Now().UTC()
	stub := &applicationStub{history: message.HistoryPage{
		Items: []message.Message{{
			ID: 9, SenderID: 41, ReceiverID: 42, ClientMsgID: "client-9", Type: 1,
			Payload: protocol.MessagePayload{Type: 1, TextMsg: &protocol.TextMessage{Content: "hello"}}, CreatedAt: now,
		}},
		HasMore: true, NextBeforeMessageID: 9,
	}}
	handler := &Handler{application: stub}
	request := httptest.NewRequest("GET", "/api/chat/history?peerUserID=41&beforeMessageID=10&limit=20", nil)
	request.Header.Set("User-ID", "42")
	request.Header.Set("Role", "2")
	recorder := httptest.NewRecorder()
	handler.History(recorder, request)
	if stub.userID != 42 || stub.peerID != 41 || stub.before != 10 || stub.limit != 20 {
		t.Fatalf("application args user=%d peer=%d before=%d limit=%d", stub.userID, stub.peerID, stub.before, stub.limit)
	}
	body := decodeBody(t, recorder)
	if body.Code != 0 || !strings.Contains(recorder.Body.String(), `"messageId":9`) || !strings.Contains(recorder.Body.String(), `"nextBeforeMessageID":9`) {
		t.Fatalf("history response = %s", recorder.Body.String())
	}
}

func TestReadAndTopOnlyUseHeaderUserID(t *testing.T) {
	readID := uint64(88)
	stub := &applicationStub{session: message.Session{
		Peer: message.Peer{ID: 7}, LastMessageID: 90, LastMessageAt: time.Now().UTC(),
		LastReadMessageID: &readID, UnreadCount: 2,
	}}
	handler := &Handler{application: stub}

	readRequest := httptest.NewRequest("PUT", "/api/chat/read", strings.NewReader(`{"peerUserID":7,"lastReadMessageID":88}`))
	readRequest.Header.Set("User-ID", "42")
	readRequest.Header.Set("Role", "2")
	readRecorder := httptest.NewRecorder()
	handler.Read(readRecorder, readRequest)
	if decodeBody(t, readRecorder).Code != 0 || stub.userID != 42 || stub.peerID != 7 || stub.readID != 88 {
		t.Fatalf("read args user=%d peer=%d read=%d body=%s", stub.userID, stub.peerID, stub.readID, readRecorder.Body.String())
	}

	topRequest := httptest.NewRequest("POST", "/api/chat/user_top", strings.NewReader(`{"friendId":7,"isTop":true}`))
	topRequest.Header.Set("User-ID", "42")
	topRequest.Header.Set("Role", "2")
	topRecorder := httptest.NewRecorder()
	handler.Top(topRecorder, topRequest)
	if decodeBody(t, topRecorder).Code != 0 || stub.userID != 42 || stub.peerID != 7 || !stub.top {
		t.Fatalf("top args user=%d peer=%d top=%t body=%s", stub.userID, stub.peerID, stub.top, topRecorder.Body.String())
	}

	legacyRequest := httptest.NewRequest("POST", "/api/chat/user_top", strings.NewReader(`{"friendId":7}`))
	legacyRequest.Header.Set("User-ID", "42")
	legacyRequest.Header.Set("Role", "2")
	legacyRecorder := httptest.NewRecorder()
	handler.Top(legacyRecorder, legacyRequest)
	if decodeBody(t, legacyRecorder).Code != 0 || !stub.toggled {
		t.Fatalf("legacy top response = %s", legacyRecorder.Body.String())
	}
}

func TestHandlersRejectMissingIdentityAndMalformedInput(t *testing.T) {
	handler := &Handler{application: &applicationStub{}}

	history := httptest.NewRecorder()
	handler.History(history, httptest.NewRequest("GET", "/api/chat/history?peerUserID=7", nil))
	if decodeBody(t, history).Code != message.CodeInvalidChatQuery {
		t.Fatalf("missing identity response = %s", history.Body.String())
	}

	readRequest := httptest.NewRequest("PUT", "/api/chat/read", strings.NewReader(`{"peerUserID":7,"lastReadMessageID":8,"userID":99}`))
	readRequest.Header.Set("User-ID", "42")
	readRequest.Header.Set("Role", "2")
	read := httptest.NewRecorder()
	handler.Read(read, readRequest)
	if decodeBody(t, read).Code != message.CodeInvalidReadMessage {
		t.Fatalf("unknown body identity field response = %s", read.Body.String())
	}
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) httpresponse.Body {
	t.Helper()
	var body httpresponse.Body
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}
