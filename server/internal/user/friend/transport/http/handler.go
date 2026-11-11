// Package http exposes the GIM V1 friend API consumed by the user Web app.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/kanhai447/GIM/server/internal/user/friend"
	"github.com/zeromicro/go-zero/rest"
)

const (
	codeUnauthenticated uint32 = 1203
	maxJSONBody                = 64 * 1024
)

type Application interface {
	Search(ctx context.Context, userID uint64, keyword string, limit int) ([]friend.SearchResult, error)
	SendRequest(ctx context.Context, requesterID, friendID uint64, message string) (friend.Request, error)
	ListRequests(ctx context.Context, userID uint64, status int, limit int) ([]friend.Request, error)
	Respond(ctx context.Context, reviewerID, requestID uint64, status int) (friend.Request, error)
	ListFriends(ctx context.Context, userID uint64, limit int) ([]friend.Friend, error)
	FriendInfo(ctx context.Context, userID, friendID uint64) (friend.Friend, error)
	UpdateRemark(ctx context.Context, userID, friendID uint64, remark string) (friend.Friend, error)
	DeleteFriend(ctx context.Context, userID, friendID uint64) error
}

type Handler struct{ application Application }

func RegisterRoutes(server *rest.Server, application Application) {
	handler := &Handler{application: application}
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodGet, Path: "/api/user/search", Handler: handler.Search},
		{Method: stdhttp.MethodGet, Path: "/api/user/friends", Handler: handler.ListFriends},
		{Method: stdhttp.MethodPost, Path: "/api/user/friends", Handler: handler.SendRequest},
		{Method: stdhttp.MethodPut, Path: "/api/user/friends", Handler: handler.UpdateRemark},
		{Method: stdhttp.MethodDelete, Path: "/api/user/friends", Handler: handler.DeleteFriend},
		{Method: stdhttp.MethodGet, Path: "/api/user/friend_info", Handler: handler.FriendInfo},
		{Method: stdhttp.MethodGet, Path: "/api/user/valid", Handler: handler.ListRequests},
		{Method: stdhttp.MethodPut, Path: "/api/user/valid_status", Handler: handler.Respond},
	})
}

func (handler *Handler) Search(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	query := request.URL.Query()
	keyword := strings.TrimSpace(query.Get("keyword"))
	if keyword == "" {
		keyword = strings.TrimSpace(query.Get("search"))
	}
	limit, ok := parseLimit(query, friend.DefaultLimit)
	if !ok {
		writeError(writer, apperror.Business(friend.CodeInvalid, "搜索参数无效"))
		return
	}
	if keyword == "" {
		writeError(writer, apperror.Business(friend.CodeInvalid, "请输入搜索关键词"))
		return
	}
	items, err := handler.application.Search(request.Context(), userID, keyword, limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]any{"list": items})
}

func (handler *Handler) SendRequest(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	var input struct {
		FriendID uint64 `json:"friendID"`
		Message  string `json:"message"`
	}
	if !decodeJSON(request, &input) || input.FriendID == 0 {
		writeError(writer, apperror.Business(friend.CodeInvalid, "好友申请参数无效"))
		return
	}
	item, err := handler.application.SendRequest(request.Context(), userID, input.FriendID, input.Message)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, item)
}

func (handler *Handler) ListRequests(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	query := request.URL.Query()
	status := friend.StatusPending
	if text := strings.TrimSpace(query.Get("status")); text != "" {
		switch text {
		case "all":
			status = -1
		default:
			parsed, err := strconv.Atoi(text)
			if err != nil {
				writeError(writer, apperror.Business(friend.CodeInvalid, "申请状态无效"))
				return
			}
			status = parsed
		}
	}
	limit, ok := parseLimit(query, friend.DefaultLimit)
	if !ok {
		writeError(writer, apperror.Business(friend.CodeInvalid, "查询参数无效"))
		return
	}
	items, err := handler.application.ListRequests(request.Context(), userID, status, limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]any{"list": items})
}

func (handler *Handler) Respond(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	var input struct {
		ValidID uint64 `json:"validID"`
		Status  int    `json:"status"`
	}
	if !decodeJSON(request, &input) || input.ValidID == 0 {
		writeError(writer, apperror.Business(friend.CodeInvalid, "申请处理参数无效"))
		return
	}
	item, err := handler.application.Respond(request.Context(), userID, input.ValidID, input.Status)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, item)
}

func (handler *Handler) ListFriends(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	limit, ok := parseLimit(request.URL.Query(), friend.MaxLimit)
	if !ok {
		writeError(writer, apperror.Business(friend.CodeInvalid, "查询参数无效"))
		return
	}
	items, err := handler.application.ListFriends(request.Context(), userID, limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]any{"list": items})
}

func (handler *Handler) FriendInfo(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	friendID, ok := parseID(request.URL.Query(), "friendID")
	if !ok {
		writeError(writer, apperror.Business(friend.CodeInvalid, "好友参数无效"))
		return
	}
	item, err := handler.application.FriendInfo(request.Context(), userID, friendID)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, item)
}

func (handler *Handler) UpdateRemark(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	var input struct {
		FriendID uint64 `json:"friendID"`
		Remark   string `json:"remark"`
	}
	if !decodeJSON(request, &input) || input.FriendID == 0 {
		writeError(writer, apperror.Business(friend.CodeInvalid, "好友备注参数无效"))
		return
	}
	item, err := handler.application.UpdateRemark(request.Context(), userID, input.FriendID, input.Remark)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, item)
}

func (handler *Handler) DeleteFriend(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, unauthenticated())
		return
	}
	friendID, ok := parseID(request.URL.Query(), "friendID")
	if !ok {
		writeError(writer, apperror.Business(friend.CodeInvalid, "好友参数无效"))
		return
	}
	if err := handler.application.DeleteFriend(request.Context(), userID, friendID); err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"deleted": true})
}

func trustedUserID(request *stdhttp.Request) (uint64, bool) {
	if request == nil {
		return 0, false
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	return userID, err == nil && userID > 0
}

func parseLimit(values url.Values, fallback int) (int, bool) {
	text := strings.TrimSpace(values.Get("limit"))
	if text == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(text)
	if err != nil || parsed < 1 || parsed > friend.MaxLimit {
		return 0, false
	}
	return parsed, true
}

func parseID(values url.Values, key string) (uint64, bool) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(values.Get(key)), 10, 64)
	return parsed, err == nil && parsed > 0
}

func decodeJSON(request *stdhttp.Request, target any) bool {
	if request == nil || request.Body == nil {
		return false
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, maxJSONBody+1))
	if err != nil || len(payload) == 0 || len(payload) > maxJSONBody {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return errors.Is(decoder.Decode(&struct{}{}), io.EOF)
}

func unauthenticated() error {
	return apperror.HTTP(stdhttp.StatusUnauthorized, codeUnauthenticated, "未认证")
}

func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }
