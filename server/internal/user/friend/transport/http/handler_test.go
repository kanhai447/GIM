package http

import (
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user/friend"
)

type fakeApplication struct {
	search       func(context.Context, uint64, string, int) ([]friend.SearchResult, error)
	sendRequest  func(context.Context, uint64, uint64, string) (friend.Request, error)
	listRequests func(context.Context, uint64, int, int) ([]friend.Request, error)
	respond      func(context.Context, uint64, uint64, int) (friend.Request, error)
	listFriends  func(context.Context, uint64, int) ([]friend.Friend, error)
	friendInfo   func(context.Context, uint64, uint64) (friend.Friend, error)
	updateRemark func(context.Context, uint64, uint64, string) (friend.Friend, error)
	deleteFriend func(context.Context, uint64, uint64) error
}

func (application fakeApplication) Search(ctx context.Context, userID uint64, keyword string, limit int) ([]friend.SearchResult, error) {
	return application.search(ctx, userID, keyword, limit)
}

func (application fakeApplication) SendRequest(ctx context.Context, requesterID, friendID uint64, message string) (friend.Request, error) {
	return application.sendRequest(ctx, requesterID, friendID, message)
}

func (application fakeApplication) ListRequests(ctx context.Context, userID uint64, status, limit int) ([]friend.Request, error) {
	return application.listRequests(ctx, userID, status, limit)
}

func (application fakeApplication) Respond(ctx context.Context, reviewerID, requestID uint64, status int) (friend.Request, error) {
	return application.respond(ctx, reviewerID, requestID, status)
}

func (application fakeApplication) ListFriends(ctx context.Context, userID uint64, limit int) ([]friend.Friend, error) {
	return application.listFriends(ctx, userID, limit)
}

func (application fakeApplication) FriendInfo(ctx context.Context, userID, friendID uint64) (friend.Friend, error) {
	return application.friendInfo(ctx, userID, friendID)
}

func (application fakeApplication) UpdateRemark(ctx context.Context, userID, friendID uint64, remark string) (friend.Friend, error) {
	return application.updateRemark(ctx, userID, friendID, remark)
}

func (application fakeApplication) DeleteFriend(ctx context.Context, userID, friendID uint64) error {
	return application.deleteFriend(ctx, userID, friendID)
}

type envelope struct {
	Code uint32          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func request(t *testing.T, method, target, body string, userID string) *stdhttp.Request {
	t.Helper()
	var reader io.Reader = stdhttp.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if userID != "" {
		req.Header.Set("User-ID", userID)
	}
	return req
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder) envelope {
	t.Helper()
	var body envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode envelope %q: %v", recorder.Body.String(), err)
	}
	return body
}

func TestAllRoutesRejectMissingIdentity(t *testing.T) {
	handler := &Handler{application: fakeApplication{}}
	cases := []struct {
		name   string
		method string
		target string
		body   string
		call   func(stdhttp.ResponseWriter, *stdhttp.Request)
	}{
		{name: "search", method: stdhttp.MethodGet, target: "/api/user/search?keyword=a", call: handler.Search},
		{name: "list friends", method: stdhttp.MethodGet, target: "/api/user/friends", call: handler.ListFriends},
		{name: "send", method: stdhttp.MethodPost, target: "/api/user/friends", body: `{"friendID":8}`, call: handler.SendRequest},
		{name: "remark", method: stdhttp.MethodPut, target: "/api/user/friends", body: `{"friendID":8,"remark":"x"}`, call: handler.UpdateRemark},
		{name: "delete", method: stdhttp.MethodDelete, target: "/api/user/friends?friendID=8", call: handler.DeleteFriend},
		{name: "info", method: stdhttp.MethodGet, target: "/api/user/friend_info?friendID=8", call: handler.FriendInfo},
		{name: "requests", method: stdhttp.MethodGet, target: "/api/user/valid", call: handler.ListRequests},
		{name: "respond", method: stdhttp.MethodPut, target: "/api/user/valid_status", body: `{"validID":3,"status":1}`, call: handler.Respond},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.call(recorder, request(t, test.method, test.target, test.body, ""))
			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", recorder.Code)
			}
			if body := decode(t, recorder); body.Code != codeUnauthenticated {
				t.Fatalf("code = %d, want %d", body.Code, codeUnauthenticated)
			}
		})
	}
}

func TestSearchReturnsList(t *testing.T) {
	handler := &Handler{application: fakeApplication{
		search: func(_ context.Context, userID uint64, keyword string, limit int) ([]friend.SearchResult, error) {
			if userID != 7 || keyword != "ali" || limit != 5 {
				t.Fatalf("Search(%d,%q,%d)", userID, keyword, limit)
			}
			return []friend.SearchResult{{UserID: 8, Nickname: "Alice", IsFriend: true}}, nil
		},
	}}
	recorder := httptest.NewRecorder()
	handler.Search(recorder, request(t, stdhttp.MethodGet, "/api/user/search?keyword=ali&limit=5", "", "7"))
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code uint32 `json:"code"`
		Data struct {
			List []friend.SearchResult `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Code != 0 || len(body.Data.List) != 1 || !body.Data.List[0].IsFriend {
		t.Fatalf("body = %s, err=%v", recorder.Body.String(), err)
	}
}

func TestSearchAcceptsLegacySearchParameter(t *testing.T) {
	handler := &Handler{application: fakeApplication{
		search: func(_ context.Context, _ uint64, keyword string, _ int) ([]friend.SearchResult, error) {
			if keyword != "legacy" {
				t.Fatalf("keyword = %q", keyword)
			}
			return nil, nil
		},
	}}
	recorder := httptest.NewRecorder()
	handler.Search(recorder, request(t, stdhttp.MethodGet, "/api/user/search?search=legacy", "", "7"))
	if body := decode(t, recorder); body.Code != 0 {
		t.Fatalf("code = %d body=%s", body.Code, recorder.Body.String())
	}
}

func TestSearchRejectsBlankKeywordAndBadLimit(t *testing.T) {
	handler := &Handler{application: fakeApplication{}}
	for _, target := range []string{"/api/user/search", "/api/user/search?keyword=a&limit=500", "/api/user/search?keyword=a&limit=zero"} {
		recorder := httptest.NewRecorder()
		handler.Search(recorder, request(t, stdhttp.MethodGet, target, "", "7"))
		if body := decode(t, recorder); body.Code != friend.CodeInvalid {
			t.Fatalf("%s: code = %d, want %d", target, body.Code, friend.CodeInvalid)
		}
	}
}

func TestSendRequestDecodesAndPropagates(t *testing.T) {
	handler := &Handler{application: fakeApplication{
		sendRequest: func(_ context.Context, requesterID, friendID uint64, message string) (friend.Request, error) {
			if requesterID != 7 || friendID != 8 || message != "hi" {
				t.Fatalf("SendRequest(%d,%d,%q)", requesterID, friendID, message)
			}
			return friend.Request{ID: 11, Status: friend.StatusPending}, nil
		},
	}}
	recorder := httptest.NewRecorder()
	handler.SendRequest(recorder, request(t, stdhttp.MethodPost, "/api/user/friends", `{"friendID":8,"message":"hi"}`, "7"))
	if body := decode(t, recorder); body.Code != 0 {
		t.Fatalf("body = %s", recorder.Body.String())
	}

	for _, invalid := range []string{`{"friendID":0}`, `{"friendID":8,"unknown":1}`, ``, `not-json`} {
		recorder := httptest.NewRecorder()
		handler.SendRequest(recorder, request(t, stdhttp.MethodPost, "/api/user/friends", invalid, "7"))
		if body := decode(t, recorder); body.Code != friend.CodeInvalid {
			t.Fatalf("body %q: code = %d", invalid, body.Code)
		}
	}
}

func TestListRequestsStatusMapping(t *testing.T) {
	calls := 0
	handler := &Handler{application: fakeApplication{
		listRequests: func(_ context.Context, userID uint64, status, limit int) ([]friend.Request, error) {
			calls++
			if userID != 7 {
				t.Fatalf("userID = %d", userID)
			}
			if calls == 1 && (status != -1 || limit != 20) {
				t.Fatalf("status=all mapping: status=%d limit=%d", status, limit)
			}
			if calls == 2 && status != friend.StatusAccepted {
				t.Fatalf("explicit status mapping: %d", status)
			}
			return nil, nil
		},
	}}
	for _, target := range []string{"/api/user/valid?status=all", "/api/user/valid?status=1"} {
		recorder := httptest.NewRecorder()
		handler.ListRequests(recorder, request(t, stdhttp.MethodGet, target, "", "7"))
		if body := decode(t, recorder); body.Code != 0 {
			t.Fatalf("%s: body = %s", target, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	handler.ListRequests(recorder, request(t, stdhttp.MethodGet, "/api/user/valid?status=pending", "", "7"))
	if body := decode(t, recorder); body.Code != friend.CodeInvalid {
		t.Fatalf("invalid status code = %d", body.Code)
	}
}

func TestRespondDecodesStatus(t *testing.T) {
	handler := &Handler{application: fakeApplication{
		respond: func(_ context.Context, reviewerID, requestID uint64, status int) (friend.Request, error) {
			if reviewerID != 7 || requestID != 3 || status != friend.StatusRejected {
				t.Fatalf("Respond(%d,%d,%d)", reviewerID, requestID, status)
			}
			return friend.Request{ID: 3, Status: friend.StatusRejected}, nil
		},
	}}
	recorder := httptest.NewRecorder()
	handler.Respond(recorder, request(t, stdhttp.MethodPut, "/api/user/valid_status", `{"validID":3,"status":2}`, "7"))
	if body := decode(t, recorder); body.Code != 0 {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestListFriendsFriendInfoRemarkDelete(t *testing.T) {
	handler := &Handler{application: fakeApplication{
		listFriends: func(_ context.Context, userID uint64, limit int) ([]friend.Friend, error) {
			if userID != 7 || limit != friend.MaxLimit {
				t.Fatalf("ListFriends(%d,%d)", userID, limit)
			}
			return []friend.Friend{{UserID: 8, IsOnline: true}}, nil
		},
		friendInfo: func(_ context.Context, userID, friendID uint64) (friend.Friend, error) {
			if userID != 7 || friendID != 8 {
				t.Fatalf("FriendInfo(%d,%d)", userID, friendID)
			}
			return friend.Friend{UserID: 8, Remark: "buddy"}, nil
		},
		updateRemark: func(_ context.Context, userID, friendID uint64, remark string) (friend.Friend, error) {
			if userID != 7 || friendID != 8 || remark != "buddy" {
				t.Fatalf("UpdateRemark(%d,%d,%q)", userID, friendID, remark)
			}
			return friend.Friend{UserID: 8, Remark: remark}, nil
		},
		deleteFriend: func(_ context.Context, userID, friendID uint64) error {
			if userID != 7 || friendID != 8 {
				t.Fatalf("DeleteFriend(%d,%d)", userID, friendID)
			}
			return nil
		},
	}}

	for _, test := range []struct {
		name string
		call func(stdhttp.ResponseWriter, *stdhttp.Request)
		req  *stdhttp.Request
	}{
		{name: "friends", call: handler.ListFriends, req: request(t, stdhttp.MethodGet, "/api/user/friends", "", "7")},
		{name: "info", call: handler.FriendInfo, req: request(t, stdhttp.MethodGet, "/api/user/friend_info?friendID=8", "", "7")},
		{name: "remark", call: handler.UpdateRemark, req: request(t, stdhttp.MethodPut, "/api/user/friends", `{"friendID":8,"remark":"buddy"}`, "7")},
		{name: "delete", call: handler.DeleteFriend, req: request(t, stdhttp.MethodDelete, "/api/user/friends?friendID=8", "", "7")},
	} {
		recorder := httptest.NewRecorder()
		test.call(recorder, test.req)
		if body := decode(t, recorder); body.Code != 0 {
			t.Fatalf("%s: body = %s", test.name, recorder.Body.String())
		}
	}
}

func TestBusinessErrorStaysInEnvelope(t *testing.T) {
	handler := &Handler{application: fakeApplication{
		friendInfo: func(context.Context, uint64, uint64) (friend.Friend, error) {
			return friend.Friend{}, apperror.Business(friend.CodeNotFriend, "对方不是好友")
		},
	}}
	recorder := httptest.NewRecorder()
	handler.FriendInfo(recorder, request(t, stdhttp.MethodGet, "/api/user/friend_info?friendID=8", "", "7"))
	body := decode(t, recorder)
	if recorder.Code != stdhttp.StatusOK || body.Code != friend.CodeNotFriend {
		t.Fatalf("status=%d code=%d body=%s", recorder.Code, body.Code, recorder.Body.String())
	}
}
