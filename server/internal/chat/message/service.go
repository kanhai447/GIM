package message

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

const (
	TextMessageType = uint8(1)

	CodeInvalidMessage      = uint32(2201)
	CodeRecipientMissing    = uint32(2202)
	CodeFriendshipRequired  = uint32(2203)
	CodeMessageUnavailable  = uint32(2204)
	CodeIdempotencyConflict = uint32(2205)
	CodeChatRestricted      = uint32(2206)
	CodeInvalidChatQuery    = uint32(2207)
	CodeSessionMissing      = uint32(2208)
	CodeInvalidCursor       = uint32(2209)
	CodeInvalidReadMessage  = uint32(2210)
)

const (
	DefaultHistoryLimit = 20
	MaxHistoryLimit     = 100
	DefaultSessionLimit = 20
	MaxSessionLimit     = 100
)

var (
	ErrRecipientNotFound = errors.New("recipient not found")
	clientMsgIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
)

type PublicError struct {
	Code    uint32
	Message string
	cause   error
}

func (err *PublicError) Error() string { return err.Message }
func (err *PublicError) Unwrap() error { return err.cause }

type Config struct {
	MaxTextBytes      int
	MaxPayloadBytes   int
	DependencyTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{MaxTextBytes: 4096, MaxPayloadBytes: 16 * 1024, DependencyTimeout: 2 * time.Second}
}

type SendRequest struct {
	ClientMsgID string
	ReceiverID  uint64
	Message     protocol.MessagePayload
}

type SendResult struct {
	Message    Message
	CreatedNew bool
}

type Service struct {
	messages   Repository
	sends      SendRepository
	history    HistoryRepository
	sessions   SessionRepository
	recipients RecipientValidator
	friends    FriendshipRepository
	config     Config
}

func NewService(messages Repository, recipients RecipientValidator, friends FriendshipRepository, config Config) (*Service, error) {
	if messages == nil || recipients == nil || friends == nil || config.MaxTextBytes < 1 || config.MaxPayloadBytes < 1 || config.DependencyTimeout <= 0 {
		return nil, errors.New("invalid chat message service configuration")
	}
	sends, ok := messages.(SendRepository)
	if !ok {
		return nil, errors.New("chat message repository does not support transactional send")
	}
	history, _ := messages.(HistoryRepository)
	sessions, _ := messages.(SessionRepository)
	return &Service{
		messages: messages, sends: sends, history: history, sessions: sessions,
		recipients: recipients, friends: friends, config: config,
	}, nil
}

func (service *Service) SendPrivateMessage(ctx context.Context, senderID uint64, request SendRequest) (SendResult, error) {
	if err := service.validate(senderID, request); err != nil {
		return SendResult{}, err
	}

	existing, err := service.messages.GetBySenderAndClientMsgID(ctx, senderID, request.ClientMsgID)
	switch {
	case err == nil:
		if !sameLogicalMessage(existing, request) {
			return SendResult{}, public(CodeIdempotencyConflict, "clientMsgId 已用于其他消息", nil)
		}
		return SendResult{Message: existing}, nil
	case !errors.Is(err, ErrNotFound):
		return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}

	policyContext, policyCancel := context.WithTimeout(ctx, service.config.DependencyTimeout)
	canChat, err := service.friends.CanChat(policyContext, senderID)
	policyCancel()
	if err != nil {
		return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	if !canChat {
		return SendResult{}, public(CodeChatRestricted, "当前用户已被限制聊天", nil)
	}

	dependencyContext, cancel := context.WithTimeout(ctx, service.config.DependencyTimeout)
	recipient, err := service.recipients.GetByID(dependencyContext, request.ReceiverID)
	cancel()
	if err != nil {
		if errors.Is(err, ErrRecipientNotFound) {
			return SendResult{}, public(CodeRecipientMissing, "接收用户不存在", nil)
		}
		return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	if recipient.ID != request.ReceiverID || !recipient.Active {
		return SendResult{}, public(CodeRecipientMissing, "接收用户不可用", nil)
	}

	friendContext, friendCancel := context.WithTimeout(ctx, service.config.DependencyTimeout)
	areFriends, err := service.friends.AreFriends(friendContext, senderID, request.ReceiverID)
	friendCancel()
	if err != nil {
		return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	if !areFriends {
		return SendResult{}, public(CodeFriendshipRequired, "仅好友之间可以发送私聊消息", nil)
	}

	persisted, err := service.sends.PersistWithSessions(ctx, Message{
		SenderID: senderID, ReceiverID: request.ReceiverID, ClientMsgID: request.ClientMsgID,
		Type: request.Message.Type, Preview: preview(request.Message.TextMsg.Content), Payload: request.Message,
	})
	if err == nil {
		if !sameLogicalMessage(persisted.Message, request) {
			return SendResult{}, public(CodeIdempotencyConflict, "clientMsgId 已用于其他消息", nil)
		}
		return SendResult{Message: persisted.Message, CreatedNew: persisted.CreatedNew}, nil
	}
	if errors.Is(err, ErrDuplicate) {
		existing, queryErr := service.messages.GetBySenderAndClientMsgID(ctx, senderID, request.ClientMsgID)
		if queryErr == nil && sameLogicalMessage(existing, request) {
			return SendResult{Message: existing}, nil
		}
	}
	return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
}

type SessionList struct {
	Items []Session
	Count int64
}

type HistoryPage struct {
	Items               []Message
	NextBeforeMessageID uint64
	HasMore             bool
}

func (service *Service) ListSessions(ctx context.Context, userID uint64, limit int) (SessionList, error) {
	if userID == 0 || limit < 0 || limit > MaxSessionLimit {
		return SessionList{}, public(CodeInvalidChatQuery, "会话查询参数无效", nil)
	}
	if limit == 0 {
		limit = DefaultSessionLimit
	}
	if service.sessions == nil {
		return SessionList{}, public(CodeMessageUnavailable, "消息服务暂不可用", nil)
	}
	items, count, err := service.sessions.ListSessions(ctx, userID, limit)
	if err != nil {
		return SessionList{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	return SessionList{Items: items, Count: count}, nil
}

func (service *Service) History(ctx context.Context, userID, peerID, beforeMessageID uint64, limit int) (HistoryPage, error) {
	if userID == 0 || peerID == 0 || userID == peerID || limit < 0 || limit > MaxHistoryLimit {
		return HistoryPage{}, public(CodeInvalidChatQuery, "历史消息查询参数无效", nil)
	}
	if limit == 0 {
		limit = DefaultHistoryLimit
	}
	if service.history == nil {
		return HistoryPage{}, public(CodeMessageUnavailable, "消息服务暂不可用", nil)
	}
	if beforeMessageID != 0 {
		cursor, err := service.messages.GetByID(ctx, beforeMessageID)
		if err != nil || !belongsToConversation(cursor, userID, peerID) {
			return HistoryPage{}, public(CodeInvalidCursor, "历史消息游标无效", err)
		}
	}
	items, err := service.history.ListHistory(ctx, userID, peerID, beforeMessageID, limit+1)
	if err != nil {
		return HistoryPage{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	var next uint64
	if hasMore && len(items) > 0 {
		next = items[0].ID
	}
	return HistoryPage{Items: items, NextBeforeMessageID: next, HasMore: hasMore}, nil
}

func (service *Service) MarkRead(ctx context.Context, userID, peerID, readThroughMessageID uint64) (Session, error) {
	if userID == 0 || peerID == 0 || userID == peerID || readThroughMessageID == 0 {
		return Session{}, public(CodeInvalidReadMessage, "已读消息参数无效", nil)
	}
	if service.sessions == nil {
		return Session{}, public(CodeMessageUnavailable, "消息服务暂不可用", nil)
	}
	session, err := service.sessions.MarkRead(ctx, userID, peerID, readThroughMessageID)
	switch {
	case err == nil:
		return session, nil
	case errors.Is(err, ErrSessionNotFound):
		return Session{}, public(CodeSessionMissing, "会话不存在", nil)
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrMessageNotInConversation):
		return Session{}, public(CodeInvalidReadMessage, "已读消息不属于当前会话", nil)
	default:
		return Session{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
}

func (service *Service) SetSessionTop(ctx context.Context, userID, peerID uint64, isTop bool) (Session, error) {
	if userID == 0 || peerID == 0 || userID == peerID {
		return Session{}, public(CodeInvalidChatQuery, "会话参数无效", nil)
	}
	if service.sessions == nil {
		return Session{}, public(CodeMessageUnavailable, "消息服务暂不可用", nil)
	}
	session, err := service.sessions.SetSessionTop(ctx, userID, peerID, isTop)
	if errors.Is(err, ErrSessionNotFound) {
		return Session{}, public(CodeSessionMissing, "会话不存在", nil)
	}
	if err != nil {
		return Session{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	return session, nil
}

func (service *Service) ToggleSessionTop(ctx context.Context, userID, peerID uint64) (Session, error) {
	if userID == 0 || peerID == 0 || userID == peerID {
		return Session{}, public(CodeInvalidChatQuery, "会话参数无效", nil)
	}
	if service.sessions == nil {
		return Session{}, public(CodeMessageUnavailable, "消息服务暂不可用", nil)
	}
	session, err := service.sessions.ToggleSessionTop(ctx, userID, peerID)
	if errors.Is(err, ErrSessionNotFound) {
		return Session{}, public(CodeSessionMissing, "会话不存在", nil)
	}
	if err != nil {
		return Session{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	return session, nil
}

func (service *Service) HideSession(ctx context.Context, userID, peerID uint64) error {
	if userID == 0 || peerID == 0 || userID == peerID {
		return public(CodeInvalidChatQuery, "会话参数无效", nil)
	}
	if service.sessions == nil {
		return public(CodeMessageUnavailable, "消息服务暂不可用", nil)
	}
	if err := service.sessions.HideSession(ctx, userID, peerID); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return public(CodeSessionMissing, "会话不存在", nil)
		}
		return public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	return nil
}

func (service *Service) validate(senderID uint64, request SendRequest) error {
	if senderID == 0 || request.ReceiverID == 0 || senderID == request.ReceiverID {
		return public(CodeInvalidMessage, "消息发送者或接收者无效", nil)
	}
	if !clientMsgIDPattern.MatchString(request.ClientMsgID) {
		return public(CodeInvalidMessage, "clientMsgId 格式无效", nil)
	}
	if request.Message.Type != TextMessageType || request.Message.TextMsg == nil {
		return public(CodeInvalidMessage, "暂不支持该消息类型", nil)
	}
	content := request.Message.TextMsg.Content
	if strings.TrimSpace(content) == "" || !utf8.ValidString(content) || len([]byte(content)) > service.config.MaxTextBytes {
		return public(CodeInvalidMessage, "文本消息内容无效或过长", nil)
	}
	payload, err := json.Marshal(request.Message)
	if err != nil || len(payload) > service.config.MaxPayloadBytes {
		return public(CodeInvalidMessage, "消息 payload 无效或过大", nil)
	}
	return nil
}

func public(code uint32, message string, cause error) error {
	return &PublicError{Code: code, Message: message, cause: cause}
}

func sameLogicalMessage(existing Message, request SendRequest) bool {
	return existing.ReceiverID == request.ReceiverID && existing.Type == request.Message.Type &&
		existing.Payload.TextMsg != nil && request.Message.TextMsg != nil &&
		existing.Payload.TextMsg.Content == request.Message.TextMsg.Content
}

func belongsToConversation(value Message, first, second uint64) bool {
	return value.SenderID == first && value.ReceiverID == second || value.SenderID == second && value.ReceiverID == first
}

func preview(content string) string {
	runes := []rune(content)
	if len(runes) > 30 {
		return string(runes[:30])
	}
	return content
}
