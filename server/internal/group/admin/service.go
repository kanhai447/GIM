package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid group admin request")
	ErrNotFound = errors.New("group admin resource not found")
)

type Query struct {
	BeforeID uint64
	Limit    int
	Search   string
	Status   uint8
}

type Group struct {
	ID                uint64    `json:"id"`
	Name              string    `json:"name"`
	Avatar            string    `json:"avatar"`
	Description       string    `json:"description"`
	OwnerID           uint64    `json:"ownerId"`
	OwnerNickname     string    `json:"ownerNickname"`
	Status            uint8     `json:"status"`
	MemberCount       uint64    `json:"memberCount"`
	MemberOnlineCount uint64    `json:"memberOnlineCount"`
	MessageCount      uint64    `json:"messageCount"`
	CreatedAt         time.Time `json:"createdAt"`
	MemberIDs         []uint64  `json:"-" gorm:"-"`
}

type Page struct {
	Items        []Group `json:"list"`
	NextBeforeID uint64  `json:"nextBeforeId"`
	HasMore      bool    `json:"hasMore"`
}

type MessageQuery struct {
	GroupID  uint64
	BeforeID uint64
	Limit    int
}

type Message struct {
	ID             uint64          `json:"id"`
	GroupID        uint64          `json:"groupId"`
	SenderID       uint64          `json:"senderId"`
	SenderNickname string          `json:"senderNickname"`
	SenderAvatar   string          `json:"senderAvatar"`
	MemberNickname string          `json:"memberNickname"`
	MessageType    uint8           `json:"messageType"`
	Preview        string          `json:"preview"`
	Payload        json.RawMessage `json:"message"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type MessagePage struct {
	Items        []Message `json:"list"`
	NextBeforeID uint64    `json:"nextBeforeId"`
	HasMore      bool      `json:"hasMore"`
}

type Repository interface {
	List(context.Context, Query) ([]Group, error)
	ListMessages(context.Context, MessageQuery) ([]Message, error)
	Dissolve(context.Context, uint64) error
	DeleteMessage(context.Context, uint64) error
}

type Presence interface {
	IsOnline(context.Context, uint64, time.Time) (bool, error)
}

type Service struct {
	repository Repository
	presence   Presence
}

func New(repository Repository, presence Presence) *Service {
	return &Service{repository: repository, presence: presence}
}

func (service *Service) List(ctx context.Context, query Query) (Page, error) {
	query.Search = strings.TrimSpace(query.Search)
	if query.Limit == 0 {
		query.Limit = 30
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Search) > 64 || query.Status > 2 {
		return Page{}, ErrInvalid
	}
	rows, err := service.repository.List(ctx, Query{BeforeID: query.BeforeID, Limit: query.Limit + 1, Search: query.Search, Status: query.Status})
	if err != nil {
		return Page{}, err
	}
	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}
	if service.presence != nil {
		now := time.Now()
		states := make(map[uint64]bool)
		for index := range rows {
			for _, userID := range rows[index].MemberIDs {
				online, exists := states[userID]
				if !exists {
					var queryErr error
					online, queryErr = service.presence.IsOnline(ctx, userID, now)
					if queryErr != nil {
						continue
					}
					states[userID] = online
				}
				if online {
					rows[index].MemberOnlineCount++
				}
			}
		}
	}
	var next uint64
	if hasMore && len(rows) != 0 {
		next = rows[len(rows)-1].ID
	}
	return Page{Items: rows, NextBeforeID: next, HasMore: hasMore}, nil
}

func (service *Service) ListMessages(ctx context.Context, query MessageQuery) (MessagePage, error) {
	if query.GroupID == 0 {
		return MessagePage{}, ErrInvalid
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return MessagePage{}, ErrInvalid
	}
	rows, err := service.repository.ListMessages(ctx, MessageQuery{GroupID: query.GroupID, BeforeID: query.BeforeID, Limit: query.Limit + 1})
	if err != nil {
		return MessagePage{}, err
	}
	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}
	var next uint64
	if hasMore && len(rows) != 0 {
		next = rows[len(rows)-1].ID
	}
	return MessagePage{Items: rows, NextBeforeID: next, HasMore: hasMore}, nil
}

func (service *Service) Dissolve(ctx context.Context, groupID uint64) error {
	if groupID == 0 {
		return ErrInvalid
	}
	return service.repository.Dissolve(ctx, groupID)
}
func (service *Service) DeleteMessage(ctx context.Context, messageID uint64) error {
	if messageID == 0 {
		return ErrInvalid
	}
	return service.repository.DeleteMessage(ctx, messageID)
}
