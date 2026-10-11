// Package http exposes authenticated private-chat session and history APIs.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/message"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
)

type Application interface {
	ListSessions(context.Context, uint64, int) (message.SessionList, error)
	History(context.Context, uint64, uint64, uint64, int) (message.HistoryPage, error)
	MarkRead(context.Context, uint64, uint64, uint64) (message.Session, error)
	SetSessionTop(context.Context, uint64, uint64, bool) (message.Session, error)
	ToggleSessionTop(context.Context, uint64, uint64) (message.Session, error)
}

type Handler struct{ application Application }

func RegisterRoutes(server *rest.Server, application Application) {
	handler := &Handler{application: application}
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodGet, Path: "/api/chat/session", Handler: handler.Sessions},
		{Method: stdhttp.MethodGet, Path: "/api/chat/history", Handler: handler.History},
		{Method: stdhttp.MethodPut, Path: "/api/chat/read", Handler: handler.Read},
		{Method: stdhttp.MethodPost, Path: "/api/chat/user_top", Handler: handler.Top},
	})
}

type peerResponse struct {
	UserID   uint64 `json:"userID"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type sessionResponse struct {
	Peer              peerResponse             `json:"peer"`
	LastMessage       *protocol.PrivateMessage `json:"lastMessage,omitempty"`
	LastMessageID     uint64                   `json:"lastMessageID"`
	LastMessageAt     string                   `json:"lastMessageAt"`
	LastReadMessageID *uint64                  `json:"lastReadMessageID"`
	UnreadCount       uint32                   `json:"unreadCount"`
	IsTop             bool                     `json:"isTop"`
}

type sessionListResponse struct {
	List  []sessionResponse `json:"list"`
	Count int64             `json:"count"`
}

type historyResponse struct {
	List                []protocol.PrivateMessage `json:"list"`
	HasMore             bool                      `json:"hasMore"`
	NextBeforeMessageID uint64                    `json:"nextBeforeMessageID,omitempty"`
}

type readRequest struct {
	PeerUserID        uint64 `json:"peerUserID"`
	LastReadMessageID uint64 `json:"lastReadMessageID"`
}

type topRequest struct {
	PeerUserID uint64 `json:"peerUserID"`
	FriendID   uint64 `json:"friendId"`
	IsTop      *bool  `json:"isTop"`
}

func (handler *Handler) Sessions(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "身份信息无效")
		return
	}
	limit, ok := optionalInt(request.URL.Query().Get("limit"))
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "会话查询参数无效")
		return
	}
	result, err := handler.application.ListSessions(request.Context(), userID, limit)
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	items := make([]sessionResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, sessionDTO(item, true))
	}
	_ = httpresponse.Success(writer, sessionListResponse{List: items, Count: result.Count})
}

func (handler *Handler) History(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "身份信息无效")
		return
	}
	peerID, ok := peerFromQuery(request)
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "历史消息查询参数无效")
		return
	}
	before, ok := optionalUint64(request.URL.Query().Get("beforeMessageID"))
	if !ok {
		writeFailure(writer, message.CodeInvalidCursor, "历史消息游标无效")
		return
	}
	limit, ok := optionalInt(request.URL.Query().Get("limit"))
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "历史消息查询参数无效")
		return
	}
	page, err := handler.application.History(request.Context(), userID, peerID, before, limit)
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	items := make([]protocol.PrivateMessage, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, messageDTO(item))
	}
	_ = httpresponse.Success(writer, historyResponse{
		List: items, HasMore: page.HasMore, NextBeforeMessageID: page.NextBeforeMessageID,
	})
}

func (handler *Handler) Read(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "身份信息无效")
		return
	}
	var input readRequest
	if !decodeJSON(request, &input) {
		writeFailure(writer, message.CodeInvalidReadMessage, "已读消息参数无效")
		return
	}
	result, err := handler.application.MarkRead(request.Context(), userID, input.PeerUserID, input.LastReadMessageID)
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, sessionDTO(result, false))
}

func (handler *Handler) Top(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeFailure(writer, message.CodeInvalidChatQuery, "身份信息无效")
		return
	}
	var input topRequest
	if !decodeJSON(request, &input) {
		writeFailure(writer, message.CodeInvalidChatQuery, "会话置顶参数无效")
		return
	}
	peerID := input.PeerUserID
	if peerID == 0 {
		peerID = input.FriendID
	}
	if input.PeerUserID != 0 && input.FriendID != 0 && input.PeerUserID != input.FriendID {
		writeFailure(writer, message.CodeInvalidChatQuery, "会话置顶参数无效")
		return
	}
	var result message.Session
	var err error
	if input.IsTop != nil {
		result, err = handler.application.SetSessionTop(request.Context(), userID, peerID, *input.IsTop)
	} else if input.PeerUserID == 0 && input.FriendID != 0 {
		result, err = handler.application.ToggleSessionTop(request.Context(), userID, peerID)
	} else {
		writeFailure(writer, message.CodeInvalidChatQuery, "会话置顶参数无效")
		return
	}
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, sessionDTO(result, false))
}

func sessionDTO(value message.Session, includeMessage bool) sessionResponse {
	result := sessionResponse{
		Peer:          peerResponse{UserID: value.Peer.ID, Nickname: value.Peer.Nickname, Avatar: value.Peer.Avatar},
		LastMessageID: value.LastMessageID, LastMessageAt: value.LastMessageAt.Format(time.RFC3339Nano),
		LastReadMessageID: value.LastReadMessageID, UnreadCount: value.UnreadCount, IsTop: value.IsTop,
	}
	if includeMessage && value.LastMessage.ID != 0 {
		last := messageDTO(value.LastMessage)
		result.LastMessage = &last
	}
	return result
}

func messageDTO(value message.Message) protocol.PrivateMessage {
	return protocol.PrivateMessage{
		MessageID: value.ID, SenderID: value.SenderID, ReceiverID: value.ReceiverID,
		ClientMsgID: value.ClientMsgID, Message: value.Payload, CreatedAt: value.CreatedAt.Format(time.RFC3339Nano),
	}
}

func trustedUserID(request *stdhttp.Request) (uint64, bool) {
	if request == nil {
		return 0, false
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	if err != nil || userID == 0 {
		return 0, false
	}
	role, err := strconv.ParseInt(strings.TrimSpace(request.Header.Get("Role")), 10, 32)
	return userID, err == nil && (role == 1 || role == 2)
}

func peerFromQuery(request *stdhttp.Request) (uint64, bool) {
	peerText := request.URL.Query().Get("peerUserID")
	friendText := request.URL.Query().Get("friendId")
	if peerText != "" && friendText != "" && peerText != friendText {
		return 0, false
	}
	if peerText == "" {
		peerText = friendText
	}
	peerID, err := strconv.ParseUint(peerText, 10, 64)
	return peerID, err == nil && peerID != 0
}

func optionalUint64(value string) (uint64, bool) {
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil && parsed != 0
}

func optionalInt(value string) (int, bool) {
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}

func decodeJSON(request *stdhttp.Request, target any) bool {
	if request == nil || request.Body == nil {
		return false
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, 64*1024+1))
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

func writeApplicationError(writer stdhttp.ResponseWriter, err error) {
	var publicError *message.PublicError
	if errors.As(err, &publicError) {
		writeFailure(writer, publicError.Code, publicError.Message)
		return
	}
	_ = httpresponse.Failure(writer, err)
}

func writeFailure(writer stdhttp.ResponseWriter, code uint32, text string) {
	_ = httpresponse.Failure(writer, apperror.Business(code, text))
}
