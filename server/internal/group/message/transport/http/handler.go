package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/kanhai447/GIM/server/internal/group/message"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"
)

type Application interface {
	ListSessions(context.Context, uint64, int) (message.SessionList, error)
	History(context.Context, uint64, uint64, uint64, int) (message.HistoryPage, error)
	MarkRead(context.Context, uint64, uint64, uint64) (message.Session, error)
}
type Handler struct{ application Application }

func RegisterRoutes(server *rest.Server, application Application) {
	h := &Handler{application: application}
	server.AddRoutes([]rest.Route{{Method: stdhttp.MethodGet, Path: "/api/group/session", Handler: h.Sessions}, {Method: stdhttp.MethodGet, Path: "/api/group/history/:id", Handler: h.History}, {Method: stdhttp.MethodPut, Path: "/api/group/read", Handler: h.Read}})
}

type groupResponse struct {
	GroupID uint64 `json:"groupID"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
}
type sessionResponse struct {
	Group             groupResponse          `json:"group"`
	LastMessage       *protocol.GroupMessage `json:"lastMessage,omitempty"`
	LastMessageID     uint64                 `json:"lastMessageID"`
	LastMessageAt     string                 `json:"lastMessageAt"`
	LastReadMessageID *uint64                `json:"lastReadMessageID"`
	UnreadCount       uint32                 `json:"unreadCount"`
	IsTop             bool                   `json:"isTop"`
}
type readRequest struct {
	GroupID           uint64 `json:"groupID"`
	LastReadMessageID uint64 `json:"lastReadMessageID"`
}

func (h *Handler) Sessions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	userID, ok := trustedUserID(r)
	limit, limitOK := optionalInt(r.URL.Query().Get("limit"))
	if !ok || !limitOK {
		failure(w, message.CodeInvalidQuery, "群会话查询参数无效")
		return
	}
	result, err := h.application.ListSessions(r.Context(), userID, limit)
	if err != nil {
		applicationError(w, err)
		return
	}
	items := make([]sessionResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, sessionDTO(item, true))
	}
	_ = httpresponse.Success(w, map[string]any{"list": items, "count": result.Count})
}
func (h *Handler) History(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	userID, ok := trustedUserID(r)
	groupID, groupOK := parseUint(pathvar.Vars(r)["id"])
	before, beforeOK := optionalUint(r.URL.Query().Get("beforeMessageID"))
	limit, limitOK := optionalInt(r.URL.Query().Get("limit"))
	if !ok || !groupOK || !beforeOK || !limitOK {
		failure(w, message.CodeInvalidQuery, "群历史查询参数无效")
		return
	}
	page, err := h.application.History(r.Context(), userID, groupID, before, limit)
	if err != nil {
		applicationError(w, err)
		return
	}
	items := make([]protocol.GroupMessage, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, messageDTO(item))
	}
	_ = httpresponse.Success(w, map[string]any{"list": items, "hasMore": page.HasMore, "nextBeforeMessageID": page.NextBeforeMessageID})
}
func (h *Handler) Read(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	userID, ok := trustedUserID(r)
	var input readRequest
	if !ok || !decodeJSON(r, &input) {
		failure(w, message.CodeInvalidRead, "群已读参数无效")
		return
	}
	session, err := h.application.MarkRead(r.Context(), userID, input.GroupID, input.LastReadMessageID)
	if err != nil {
		applicationError(w, err)
		return
	}
	_ = httpresponse.Success(w, sessionDTO(session, false))
}
func sessionDTO(value message.Session, include bool) sessionResponse {
	result := sessionResponse{Group: groupResponse{GroupID: value.Group.ID, Name: value.Group.Name, Avatar: value.Group.Avatar}, LastMessageID: value.LastMessageID, LastMessageAt: value.LastMessageAt.Format(time.RFC3339Nano), LastReadMessageID: value.LastReadMessageID, UnreadCount: value.UnreadCount, IsTop: value.IsTop}
	if include && value.LastMessage.ID != 0 {
		last := messageDTO(value.LastMessage)
		result.LastMessage = &last
	}
	return result
}
func messageDTO(value message.Message) protocol.GroupMessage {
	return protocol.GroupMessage{MessageID: value.ID, GroupID: value.GroupID, SenderID: value.SenderID, ClientMsgID: value.ClientMsgID, Message: value.Payload, CreatedAt: value.CreatedAt.Format(time.RFC3339Nano)}
}
func trustedUserID(r *stdhttp.Request) (uint64, bool) {
	if r == nil {
		return 0, false
	}
	id, err := strconv.ParseUint(strings.TrimSpace(r.Header.Get("User-ID")), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	role, err := strconv.Atoi(strings.TrimSpace(r.Header.Get("Role")))
	return id, err == nil && (role == 1 || role == 2)
}
func parseUint(value string) (uint64, bool) {
	v, err := strconv.ParseUint(value, 10, 64)
	return v, err == nil && v != 0
}
func optionalUint(value string) (uint64, bool) {
	if value == "" {
		return 0, true
	}
	return parseUint(value)
}
func optionalInt(value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	v, err := strconv.Atoi(value)
	return v, err == nil && v > 0
}
func decodeJSON(r *stdhttp.Request, target any) bool {
	if r == nil || r.Body == nil {
		return false
	}
	payload, err := io.ReadAll(io.LimitReader(r.Body, 64*1024+1))
	if err != nil || len(payload) == 0 || len(payload) > 64*1024 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return errors.Is(decoder.Decode(&struct{}{}), io.EOF)
}
func applicationError(w stdhttp.ResponseWriter, err error) {
	var publicError *message.PublicError
	if errors.As(err, &publicError) {
		failure(w, publicError.Code, publicError.Message)
		return
	}
	_ = httpresponse.Failure(w, err)
}
func failure(w stdhttp.ResponseWriter, code uint32, text string) {
	_ = httpresponse.Failure(w, apperror.Business(code, text))
}
