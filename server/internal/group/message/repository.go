package message

import (
	"context"
	"errors"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
)

var (
	ErrNotFound          = errors.New("group message not found")
	ErrSessionNotFound   = errors.New("group session not found")
	ErrMessageNotInGroup = errors.New("group message does not belong to group")
)

type Message struct {
	ID            uint64
	GroupID       uint64
	SenderID      uint64
	GroupMemberID uint64
	ClientMsgID   string
	Type          uint8
	Preview       string
	Payload       protocol.MessagePayload
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PersistResult struct {
	Message       Message
	TargetUserIDs []uint64
	CreatedNew    bool
}

type GroupInfo struct {
	ID     uint64
	Name   string
	Avatar string
}

type Session struct {
	ID                uint64
	UserID            uint64
	Group             GroupInfo
	LastMessage       Message
	LastMessageID     uint64
	LastMessageAt     time.Time
	LastReadMessageID *uint64
	UnreadCount       uint32
	IsTop             bool
	HiddenAt          *time.Time
}

type Repository interface {
	PersistWithSessions(context.Context, Message) (PersistResult, error)
	GetBySenderAndClientMsgID(context.Context, uint64, string) (Message, error)
	GetByID(context.Context, uint64) (Message, error)
	ListSessions(context.Context, uint64, int) ([]Session, int64, error)
	ListHistory(context.Context, uint64, uint64, uint64, int) ([]Message, error)
	MarkRead(context.Context, uint64, uint64, uint64) (Session, error)
}
