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
	recipients RecipientValidator
	friends    FriendshipRepository
	config     Config
}

func NewService(messages Repository, recipients RecipientValidator, friends FriendshipRepository, config Config) (*Service, error) {
	if messages == nil || recipients == nil || friends == nil || config.MaxTextBytes < 1 || config.MaxPayloadBytes < 1 || config.DependencyTimeout <= 0 {
		return nil, errors.New("invalid chat message service configuration")
	}
	return &Service{messages: messages, recipients: recipients, friends: friends, config: config}, nil
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

	created, err := service.messages.Create(ctx, Message{
		SenderID: senderID, ReceiverID: request.ReceiverID, ClientMsgID: request.ClientMsgID,
		Type: request.Message.Type, Preview: preview(request.Message.TextMsg.Content), Payload: request.Message,
	})
	if err == nil {
		return SendResult{Message: created, CreatedNew: true}, nil
	}
	if !errors.Is(err, ErrDuplicate) {
		return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}

	existing, err = service.messages.GetBySenderAndClientMsgID(ctx, senderID, request.ClientMsgID)
	if err != nil {
		return SendResult{}, public(CodeMessageUnavailable, "消息服务暂不可用", err)
	}
	if !sameLogicalMessage(existing, request) {
		return SendResult{}, public(CodeIdempotencyConflict, "clientMsgId 已用于其他消息", nil)
	}
	return SendResult{Message: existing}, nil
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

func preview(content string) string {
	runes := []rune(content)
	if len(runes) > 30 {
		return string(runes[:30])
	}
	return content
}
