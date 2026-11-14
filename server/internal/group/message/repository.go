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
	ErrImageFileNotFound = errors.New("group image file not found")
	ErrFileNotFound      = errors.New("group file not found")
)

// ImageFile is the verified metadata of a user-owned uploaded image.
type ImageFile struct {
	ID       uint64
	Size     int64
	MIMEType string
}

// ImageRepository resolves an image reference without trusting client-supplied
// URL/MIME/size. It is optional so existing repositories remain valid.
type ImageRepository interface {
	GetOwnedImage(context.Context, uint64, uint64) (ImageFile, error)
}

// FileInfo is the verified metadata of a user-owned uploaded file of any type.
type FileInfo struct {
	ID       uint64
	Name     string
	Size     int64
	MIMEType string
}

// FileRepository resolves a generic file reference without trusting
// client-supplied URL/name/size/MIME.
type FileRepository interface {
	GetOwnedFile(context.Context, uint64, uint64) (FileInfo, error)
}

// RecallRepository supports withdraw-message validation and read-side
// replacement. Optional so existing repositories remain valid.
type RecallRepository interface {
	GetByIDForRecall(context.Context, uint64) (Message, bool, error)
	IsRecalled(context.Context, uint64) (bool, error)
	GetWithdrawBySenderAndTarget(context.Context, uint64, uint64) (Message, bool, error)
	RecalledTargets(context.Context, []uint64) (map[uint64]bool, error)
}

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
