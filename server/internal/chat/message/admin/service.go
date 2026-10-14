package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid chat admin request")
	ErrNotFound = errors.New("chat admin message not found")
)

type Query struct {
	BeforeID   uint64
	Limit      int
	UserID     uint64
	SenderID   uint64
	ReceiverID uint64
}

type User struct {
	ID       uint64 `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type Message struct {
	ID          uint64          `json:"id"`
	Sender      User            `json:"sender"`
	Receiver    User            `json:"receiver"`
	ClientMsgID string          `json:"clientMsgId"`
	MessageType uint8           `json:"messageType"`
	Preview     string          `json:"preview"`
	Payload     json.RawMessage `json:"message"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type Page struct {
	Items        []Message `json:"list"`
	NextBeforeID uint64    `json:"nextBeforeId"`
	HasMore      bool      `json:"hasMore"`
}

type Repository interface {
	List(context.Context, Query) ([]Message, error)
	Delete(context.Context, uint64) error
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) List(ctx context.Context, query Query) (Page, error) {
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return Page{}, ErrInvalid
	}
	rows, err := service.repository.List(ctx, Query{BeforeID: query.BeforeID, Limit: query.Limit + 1, UserID: query.UserID, SenderID: query.SenderID, ReceiverID: query.ReceiverID})
	if err != nil {
		return Page{}, err
	}
	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}
	var next uint64
	if hasMore && len(rows) != 0 {
		next = rows[len(rows)-1].ID
	}
	return Page{Items: rows, NextBeforeID: next, HasMore: hasMore}, nil
}

func (service *Service) Delete(ctx context.Context, messageID uint64) error {
	if messageID == 0 {
		return ErrInvalid
	}
	return service.repository.Delete(ctx, messageID)
}
