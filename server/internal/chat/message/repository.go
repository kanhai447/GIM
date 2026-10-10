package message

import (
	"context"
	"errors"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

var (
	ErrNotFound  = errors.New("chat message not found")
	ErrDuplicate = errors.New("duplicate chat message")
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
