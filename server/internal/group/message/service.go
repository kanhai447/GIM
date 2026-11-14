package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
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
	CodeImageFileInvalid    uint32 = 2412
	CodeImageMIMEType       uint32 = 2413
	CodeImageTooLarge       uint32 = 2414
	CodeImagePayload        uint32 = 2415
	CodeFileInvalid         uint32 = 2416
	CodeFileTooLarge        uint32 = 2417
	CodeFilePayload         uint32 = 2418
	CodeWithdrawTarget      uint32 = 2419
	CodeWithdrawNotOwner    uint32 = 2420
	CodeWithdrawWindow      uint32 = 2421
	CodeWithdrawRecalled    uint32 = 2422
	CodeWithdrawPayload     uint32 = 2423
	TextMessageType         uint8  = 1
	ImageMessageType        uint8  = 2
	FileMessageType         uint8  = 3
	WithdrawMessageType     uint8  = 4
	DefaultLimit                   = 20
	MaxLimit                       = 100
)

var clientMsgIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

type Policy interface {
	GetGroupByID(context.Context, uint64) (domain.Group, error)
	IsMember(context.Context, uint64, uint64) (bool, error)
	IsMuted(context.Context, uint64, uint64) (bool, error)
}

type Config struct {
	MaxTextBytes, MaxPayloadBytes, MaxImageBytes, MaxFileBytes int
	RecallWindow                                               time.Duration
}

func DefaultConfig() Config {
	return Config{MaxTextBytes: 4096, MaxPayloadBytes: 16384, MaxImageBytes: 10 << 20, MaxFileBytes: 25 << 20, RecallWindow: 120 * time.Second}
}

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
	images     ImageRepository
	files      FileRepository
	recalls    RecallRepository
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
	if repository == nil || policy == nil || config.MaxTextBytes < 1 || config.MaxPayloadBytes < 1 || config.MaxImageBytes < 1 || config.MaxFileBytes < 1 || config.RecallWindow <= 0 {
		return nil, errors.New("invalid group message service configuration")
	}
	images, _ := repository.(ImageRepository)
	files, _ := repository.(FileRepository)
	recalls, _ := repository.(RecallRepository)
	return &Service{repository: repository, policy: policy, images: images, files: files, recalls: recalls, config: config}, nil
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

	if request.Message.Type == WithdrawMessageType {
		return service.sendWithdraw(ctx, senderID, request)
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

	payload := request.Message
	switch payload.Type {
	case ImageMessageType:
		normalized, normalizeErr := service.normalizeImage(ctx, senderID, payload)
		if normalizeErr != nil {
			return SendResult{}, normalizeErr
		}
		payload = normalized
	case FileMessageType:
		normalized, normalizeErr := service.normalizeFile(ctx, senderID, payload)
		if normalizeErr != nil {
			return SendResult{}, normalizeErr
		}
		payload = normalized
	}

	result, err := service.repository.PersistWithSessions(ctx, Message{GroupID: request.GroupID, SenderID: senderID, ClientMsgID: request.ClientMsgID, Type: payload.Type, Preview: previewOf(payload), Payload: payload})
	if err != nil {
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	if !sameLogicalMessage(result.Message, request) {
		return SendResult{}, public(CodeIdempotencyConflict, "clientMsgId 已用于其他消息", nil)
	}
	return SendResult{Message: result.Message, TargetUserIDs: result.TargetUserIDs, CreatedNew: result.CreatedNew}, nil
}

func (service *Service) sendWithdraw(ctx context.Context, senderID uint64, request SendRequest) (SendResult, error) {
	if service.recalls == nil {
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", nil)
	}
	targetID := request.Message.WithdrawMsg.MsgID
	if existing, found, err := service.recalls.GetWithdrawBySenderAndTarget(ctx, senderID, targetID); err != nil {
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	} else if found {
		return SendResult{Message: existing}, nil
	}
	target, tombstoned, err := service.recalls.GetByIDForRecall(ctx, targetID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SendResult{}, public(CodeWithdrawTarget, "撤回目标消息不存在", nil)
		}
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	if target.GroupID != request.GroupID {
		return SendResult{}, public(CodeWithdrawTarget, "撤回目标消息不存在", nil)
	}
	if target.SenderID != senderID {
		return SendResult{}, public(CodeWithdrawNotOwner, "只能撤回自己发送的消息", nil)
	}
	if tombstoned {
		return SendResult{}, public(CodeWithdrawRecalled, "消息已撤回或不可撤回", nil)
	}
	recalled, err := service.recalls.IsRecalled(ctx, targetID)
	if err != nil {
		return SendResult{}, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	if recalled {
		return SendResult{}, public(CodeWithdrawRecalled, "消息已撤回或不可撤回", nil)
	}
	if time.Since(target.CreatedAt) > service.config.RecallWindow {
		return SendResult{}, public(CodeWithdrawWindow, "超过撤回时间窗口", nil)
	}
	member, err := service.policy.IsMember(ctx, request.GroupID, senderID)
	if err != nil {
		return SendResult{}, policyError(err)
	}
	if !member {
		return SendResult{}, public(CodeNotMember, "不是群成员", nil)
	}
	result, err := service.repository.PersistWithSessions(ctx, Message{
		GroupID: request.GroupID, SenderID: senderID, ClientMsgID: request.ClientMsgID,
		Type: WithdrawMessageType, Preview: "[撤回]", Payload: request.Message,
	})
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
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.LastMessage.ID != 0 {
			ids = append(ids, item.LastMessage.ID)
		}
	}
	recalled, err := service.recalledSet(ctx, ids)
	if err != nil {
		return SessionList{}, err
	}
	for index := range items {
		if recalled[items[index].LastMessage.ID] {
			markRecalled(&items[index].LastMessage)
		}
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
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	recalled, err := service.recalledSet(ctx, ids)
	if err != nil {
		return HistoryPage{}, err
	}
	for index := range items {
		if recalled[items[index].ID] {
			markRecalled(&items[index])
		}
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
func (service *Service) recalledSet(ctx context.Context, ids []uint64) (map[uint64]bool, error) {
	if service.recalls == nil || len(ids) == 0 {
		return map[uint64]bool{}, nil
	}
	recalled, err := service.recalls.RecalledTargets(ctx, ids)
	if err != nil {
		return nil, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	return recalled, nil
}

func markRecalled(item *Message) {
	if item.ID == 0 {
		return
	}
	item.Type = WithdrawMessageType
	item.Preview = "[撤回]"
	item.Payload = protocol.MessagePayload{Type: WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: item.ID}}
}

func (service *Service) validate(senderID uint64, request SendRequest) error {
	if senderID == 0 || request.GroupID == 0 || !clientMsgIDPattern.MatchString(request.ClientMsgID) {
		return public(CodeInvalidMessage, "群消息参数无效", nil)
	}
	switch request.Message.Type {
	case TextMessageType:
		if request.Message.TextMsg == nil {
			return public(CodeInvalidMessage, "暂不支持该群消息类型", nil)
		}
		content := request.Message.TextMsg.Content
		if strings.TrimSpace(content) == "" || !utf8.ValidString(content) || len([]byte(content)) > service.config.MaxTextBytes {
			return public(CodeInvalidMessage, "文本消息内容无效或过长", nil)
		}
	case ImageMessageType:
		if err := validateImagePayload(request.Message); err != nil {
			return err
		}
	case FileMessageType:
		if err := validateFilePayload(request.Message); err != nil {
			return err
		}
	case WithdrawMessageType:
		if request.Message.WithdrawMsg == nil || request.Message.WithdrawMsg.MsgID == 0 {
			return public(CodeWithdrawPayload, "撤回消息参数无效", nil)
		}
	default:
		return public(CodeInvalidMessage, "暂不支持该群消息类型", nil)
	}
	payload, err := json.Marshal(request.Message)
	if err != nil || len(payload) > service.config.MaxPayloadBytes {
		return public(CodeInvalidMessage, "群消息 payload 无效或过大", nil)
	}
	return nil
}

const maxImageDimension = 20000

func validateImagePayload(payload protocol.MessagePayload) error {
	image := payload.ImageMsg
	if image == nil || image.FileID == 0 {
		return public(CodeImagePayload, "图片消息参数无效", nil)
	}
	if image.Width < 0 || image.Height < 0 || image.Width > maxImageDimension || image.Height > maxImageDimension || image.Size < 0 {
		return public(CodeImagePayload, "图片消息参数无效", nil)
	}
	return nil
}

func (service *Service) normalizeImage(ctx context.Context, senderID uint64, payload protocol.MessagePayload) (protocol.MessagePayload, error) {
	if service.images == nil {
		return payload, public(CodeUnavailable, "群消息服务暂不可用", nil)
	}
	file, err := service.images.GetOwnedImage(ctx, senderID, payload.ImageMsg.FileID)
	if err != nil {
		if errors.Is(err, ErrImageFileNotFound) {
			return payload, public(CodeImageFileInvalid, "图片文件不存在或无权使用", nil)
		}
		return payload, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	if !strings.HasPrefix(strings.ToLower(file.MIMEType), "image/") {
		return payload, public(CodeImageMIMEType, "文件不是图片", nil)
	}
	if file.Size < 0 || file.Size > int64(service.config.MaxImageBytes) {
		return payload, public(CodeImageTooLarge, "图片超过大小限制", nil)
	}
	normalized := payload
	image := *payload.ImageMsg
	image.URL = fmt.Sprintf("/api/file/%d", file.ID)
	image.MIMEType = file.MIMEType
	image.Size = file.Size
	normalized.ImageMsg = &image
	return normalized, nil
}

func validateFilePayload(payload protocol.MessagePayload) error {
	file := payload.FileMsg
	if file == nil || file.FileID == 0 || file.Size < 0 {
		return public(CodeFilePayload, "文件消息参数无效", nil)
	}
	return nil
}

func (service *Service) normalizeFile(ctx context.Context, senderID uint64, payload protocol.MessagePayload) (protocol.MessagePayload, error) {
	if service.files == nil {
		return payload, public(CodeUnavailable, "群消息服务暂不可用", nil)
	}
	file, err := service.files.GetOwnedFile(ctx, senderID, payload.FileMsg.FileID)
	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			return payload, public(CodeFileInvalid, "文件不存在或无权使用", nil)
		}
		return payload, public(CodeUnavailable, "群消息服务暂不可用", err)
	}
	if file.Size < 0 || file.Size > int64(service.config.MaxFileBytes) {
		return payload, public(CodeFileTooLarge, "文件超过大小限制", nil)
	}
	normalized := payload
	value := *payload.FileMsg
	value.URL = fmt.Sprintf("/api/file/%d", file.ID)
	value.FileName = file.Name
	value.Size = file.Size
	value.MIMEType = file.MIMEType
	normalized.FileMsg = &value
	return normalized, nil
}

func previewOf(payload protocol.MessagePayload) string {
	switch payload.Type {
	case ImageMessageType:
		return "[图片]"
	case FileMessageType:
		return "[文件]"
	}
	if payload.TextMsg == nil {
		return ""
	}
	return preview(payload.TextMsg.Content)
}
func sameLogicalMessage(existing Message, request SendRequest) bool {
	if existing.GroupID != request.GroupID || existing.Type != request.Message.Type {
		return false
	}
	switch existing.Type {
	case TextMessageType:
		return existing.Payload.TextMsg != nil && request.Message.TextMsg != nil && existing.Payload.TextMsg.Content == request.Message.TextMsg.Content
	case ImageMessageType:
		return existing.Payload.ImageMsg != nil && request.Message.ImageMsg != nil && existing.Payload.ImageMsg.FileID == request.Message.ImageMsg.FileID
	case FileMessageType:
		return existing.Payload.FileMsg != nil && request.Message.FileMsg != nil && existing.Payload.FileMsg.FileID == request.Message.FileMsg.FileID
	case WithdrawMessageType:
		return existing.Payload.WithdrawMsg != nil && request.Message.WithdrawMsg != nil && existing.Payload.WithdrawMsg.MsgID == request.Message.WithdrawMsg.MsgID
	default:
		return false
	}
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
