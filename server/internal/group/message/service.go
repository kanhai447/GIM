package message

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/group/domain"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

const (
	CodeInvalidMessage      uint32 = 2403
	CodeNotMember           uint32 = 2404
	CodeMuted               uint32 = 2405
	CodeUnavailable         uint32 = 2406
	CodeIdempotencyConflict uint32 = 2407
	CodeInvalidQuery        uint32 = 2408
	CodeInvalidCursor       uint32 = 2409
	CodeInvalidRead         uint32 = 2410
	CodeSessionMissing      uint32 = 2411
	TextMessageType         uint8  = 1
	DefaultLimit                   = 20
	MaxLimit                       = 100
)

var clientMsgIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

type Policy interface {
	GetGroupByID(context.Context, uint64) (domain.Group, error)
	IsMember(context.Context, uint64, uint64) (bool, error)
	IsMuted(context.Context, uint64, uint64) (bool, error)
}

type Config struct{ MaxTextBytes, MaxPayloadBytes int }

func DefaultConfig() Config { return Config{MaxTextBytes: 4096, MaxPayloadBytes: 16384} }

type PublicError struct {
	Code    uint32
	Message string
	cause   error
}

func (e *PublicError) Error() string { return e.Message }
func (e *PublicError) Unwrap() error { return e.cause }

type Service struct {
	repository Repository
	policy     Policy
	config     Config
}

type SendRequest struct {
	ClientMsgID string
	GroupID     uint64
	Message     protocol.MessagePayload
}
type SendResult struct {
	Message       Message
	TargetUserIDs []uint64
	CreatedNew    bool
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

func NewService(repository Repository, policy Policy, config Config) (*Service, error) {
	if repository == nil || policy == nil || config.MaxTextBytes < 1 || config.MaxPayloadBytes < 1 {
		return nil, errors.New("invalid group message service configuration")
	}
	return &Service{repository: repository, policy: policy, config: config}, nil
}

func (service *Service) Send(ctx context.Context, senderID uint64, request SendRequest) (SendResult, error) {
	if err := service.validate(senderID, request); err != nil {
		return SendResult{}, err
	}
	if existing, err := service.repository.GetBySenderAndClientMsgID(ctx, senderID, request.ClientMsgID); err == nil {
		if !sameLogicalMessage(existing, request) {
			return SendResult{}, public(CodeIdempotencyConflict, "clientMsgId 已用于其他消息", nil)
		}
		return SendResult{Message: existing}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}

	group, err := service.policy.GetGroupByID(ctx, request.GroupID)
	if err != nil || group.Status != domain.GroupStatusActive {
		return SendResult{}, policyError(err)
	}
	member, err := service.policy.IsMember(ctx, request.GroupID, senderID)
	if err != nil {
		return SendResult{}, policyError(err)
	}
	if !member {
		return SendResult{}, public(CodeNotMember, "不是群成员", nil)
	}
	if group.IsAllMuted {
		role, roleErr := service.memberRole(ctx, request.GroupID, senderID)
		if roleErr != nil {
			return SendResult{}, roleErr
		}
		if role == domain.GroupRoleMember {
			return SendResult{}, public(CodeMuted, "群聊已开启全员禁言", nil)
		}
	}
	muted, err := service.policy.IsMuted(ctx, request.GroupID, senderID)
	if err != nil {
		return SendResult{}, policyError(err)
	}
	if muted {
		return SendResult{}, public(CodeMuted, "你已被禁言", nil)
	}

	result, err := service.repository.PersistWithSessions(ctx, Message{GroupID: request.GroupID, SenderID: senderID, ClientMsgID: request.ClientMsgID, Type: request.Message.Type, Preview: preview(request.Message.TextMsg.Content), Payload: request.Message})
	if err != nil {
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	if !sameLogicalMessage(result.Message, request) {
		return SendResult{}, public(CodeIdempotencyConflict, "clientMsgId 已用于其他消息", nil)
	}
	return SendResult{Message: result.Message, TargetUserIDs: result.TargetUserIDs, CreatedNew: result.CreatedNew}, nil
}

func (service *Service) memberRole(ctx context.Context, groupID, userID uint64) (domain.GroupRole, error) {
	if roles, ok := service.policy.(interface {
		GetMemberRole(context.Context, uint64, uint64) (domain.GroupRole, error)
	}); ok {
		role, err := roles.GetMemberRole(ctx, groupID, userID)
		if err != nil {
			return 0, policyError(err)
		}
		return role, nil
	}
	return 0, public(CodeUnavailable, "群消息服务暂不可用", nil)
}

func (service *Service) ListSessions(ctx context.Context, userID uint64, limit int) (SessionList, error) {
	if userID == 0 || limit < 0 || limit > MaxLimit {
		return SessionList{}, public(CodeInvalidQuery, "群会话查询参数无效", nil)
	}
	if limit == 0 {
		limit = DefaultLimit
	}
	items, count, err := service.repository.ListSessions(ctx, userID, limit)
	if err != nil {
		return SessionList{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	return SessionList{Items: items, Count: count}, nil
}

func (service *Service) History(ctx context.Context, userID, groupID, before uint64, limit int) (HistoryPage, error) {
	if userID == 0 || groupID == 0 || limit < 0 || limit > MaxLimit {
		return HistoryPage{}, public(CodeInvalidQuery, "群历史查询参数无效", nil)
	}
	if err := service.requireMember(ctx, groupID, userID); err != nil {
		return HistoryPage{}, err
	}
	if limit == 0 {
		limit = DefaultLimit
	}
	if before != 0 {
		cursor, err := service.repository.GetByID(ctx, before)
		if err != nil || cursor.GroupID != groupID {
			return HistoryPage{}, public(CodeInvalidCursor, "群历史游标无效", err)
		}
	}
	items, err := service.repository.ListHistory(ctx, userID, groupID, before, limit+1)
	if err != nil {
		return HistoryPage{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	for l, r := 0, len(items)-1; l < r; l, r = l+1, r-1 {
		items[l], items[r] = items[r], items[l]
	}
	var next uint64
	if hasMore && len(items) > 0 {
		next = items[0].ID
	}
	return HistoryPage{Items: items, NextBeforeMessageID: next, HasMore: hasMore}, nil
}

func (service *Service) MarkRead(ctx context.Context, userID, groupID, messageID uint64) (Session, error) {
	if userID == 0 || groupID == 0 || messageID == 0 {
		return Session{}, public(CodeInvalidRead, "群已读参数无效", nil)
	}
	if err := service.requireMember(ctx, groupID, userID); err != nil {
		return Session{}, err
	}
	result, err := service.repository.MarkRead(ctx, userID, groupID, messageID)
	switch {
	case err == nil:
		return result, nil
	case errors.Is(err, ErrSessionNotFound):
		return Session{}, public(CodeSessionMissing, "群会话不存在", nil)
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrMessageNotInGroup):
		return Session{}, public(CodeInvalidRead, "已读消息不属于当前群", nil)
	default:
		return Session{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
}

func (service *Service) requireMember(ctx context.Context, groupID, userID uint64) error {
	ok, err := service.policy.IsMember(ctx, groupID, userID)
	if err != nil {
		return policyError(err)
	}
	if !ok {
		return public(CodeNotMember, "不是群成员", nil)
	}
	return nil
}
func (service *Service) validate(senderID uint64, request SendRequest) error {
	if senderID == 0 || request.GroupID == 0 || !clientMsgIDPattern.MatchString(request.ClientMsgID) {
		return public(CodeInvalidMessage, "群消息参数无效", nil)
	}
	if request.Message.Type != TextMessageType || request.Message.TextMsg == nil {
		return public(CodeInvalidMessage, "暂不支持该群消息类型", nil)
	}
	content := request.Message.TextMsg.Content
	if strings.TrimSpace(content) == "" || !utf8.ValidString(content) || len([]byte(content)) > service.config.MaxTextBytes {
		return public(CodeInvalidMessage, "文本消息内容无效或过长", nil)
	}
	payload, err := json.Marshal(request.Message)
	if err != nil || len(payload) > service.config.MaxPayloadBytes {
		return public(CodeInvalidMessage, "群消息 payload 无效或过大", nil)
	}
	return nil
}
func sameLogicalMessage(existing Message, request SendRequest) bool {
	return existing.GroupID == request.GroupID && existing.Type == request.Message.Type && existing.Payload.TextMsg != nil && request.Message.TextMsg != nil && existing.Payload.TextMsg.Content == request.Message.TextMsg.Content
}
func preview(content string) string {
	r := []rune(content)
	if len(r) > 30 {
		return string(r[:30])
	}
	return content
}
func public(code uint32, text string, cause error) error {
	return &PublicError{Code: code, Message: text, cause: cause}
}
func policyError(err error) error {
	if err == nil {
		return public(CodeNotMember, "群不存在或不可用", nil)
	}
	_, code, text := apperror.PublicFields(err)
	if code == 2304 {
		return public(CodeNotMember, text, err)
	}
	if code == 2311 {
		return public(CodeUnavailable, text, err)
	}
	if code >= 2301 && code <= 2313 {
		return public(code, text, err)
	}
	return &PublicError{Code: CodeUnavailable, Message: "群消息服务暂不可用", cause: err}
}
