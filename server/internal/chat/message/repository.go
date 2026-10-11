package message

import (
	"context"
	"errors"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

var (
	ErrNotFound                 = errors.New("chat message not found")
	ErrDuplicate                = errors.New("duplicate chat message")
	ErrSessionNotFound          = errors.New("chat session not found")
	ErrMessageNotInConversation = errors.New("chat message does not belong to conversation")
)

type Message struct {
	ID          uint64
	SenderID    uint64
	ReceiverID  uint64
	ClientMsgID string
	Type        uint8
	Preview     string
	Payload     protocol.MessagePayload
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository interface {
	Create(context.Context, Message) (Message, error)
	GetBySenderAndClientMsgID(context.Context, uint64, string) (Message, error)
	GetByID(context.Context, uint64) (Message, error)
}

type PersistResult struct {
	Message    Message
	CreatedNew bool
}

// SendRepository owns the atomic message + two session transaction.
type SendRepository interface {
	PersistWithSessions(context.Context, Message) (PersistResult, error)
}

type HistoryRepository interface {
	ListHistory(context.Context, uint64, uint64, uint64, int) ([]Message, error)
}

type Peer struct {
	ID       uint64
	Nickname string
	Avatar   string
}

type Session struct {
	ID                uint64
	UserID            uint64
	Peer              Peer
	LastMessage       Message
	LastMessageID     uint64
	LastMessageAt     time.Time
	LastReadMessageID *uint64
	UnreadCount       uint32
	IsTop             bool
	HiddenAt          *time.Time
}

type SessionRepository interface {
	GetSession(context.Context, uint64, uint64) (Session, error)
	ListSessions(context.Context, uint64, int) ([]Session, int64, error)
	MarkRead(context.Context, uint64, uint64, uint64) (Session, error)
	SetSessionTop(context.Context, uint64, uint64, bool) (Session, error)
	ToggleSessionTop(context.Context, uint64, uint64) (Session, error)
	HideSession(context.Context, uint64, uint64) error
}

type Recipient struct {
	ID     uint64
	Active bool
}

type RecipientValidator interface {
	GetByID(context.Context, uint64) (Recipient, error)
}

type FriendshipRepository interface {
	AreFriends(context.Context, uint64, uint64) (bool, error)
	CanChat(context.Context, uint64) (bool, error)
}
