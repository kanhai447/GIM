package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/user/domain"
)

var (
	ErrInvalid    = errors.New("invalid user admin request")
	ErrNotFound   = errors.New("admin user not found")
	ErrSelfAction = errors.New("admin cannot disable own account")
)

type Query struct {
	BeforeID uint64
	Limit    int
	Search   string
	Status   domain.Status
}

type User struct {
	ID                 uint64        `json:"id"`
	Account            string        `json:"account"`
	Nickname           string        `json:"nickname"`
	Avatar             string        `json:"avatar"`
	IP                 string        `json:"ip,omitempty"`
	Address            string        `json:"address,omitempty"`
	Role               int32         `json:"role"`
	Status             domain.Status `json:"status"`
	IsOnline           bool          `json:"isOnline"`
	SendMessageCount   uint64        `json:"sendMessageCount"`
	GroupAdminCount    uint64        `json:"groupAdminCount"`
	GroupCount         uint64        `json:"groupCount"`
	CurtailChat        bool          `json:"curtailChat"`
	CurtailAddUser     bool          `json:"curtailAddUser"`
	CurtailCreateGroup bool          `json:"curtailCreateGroup"`
	CurtailInGroupChat bool          `json:"curtailInGroupChat"`
	CreatedAt          time.Time     `json:"createdAt"`
}

type Restrictions struct {
	UserID             uint64 `json:"userId"`
	CurtailChat        bool   `json:"curtailChat"`
	CurtailAddUser     bool   `json:"curtailAddUser"`
	CurtailCreateGroup bool   `json:"curtailCreateGroup"`
	CurtailInGroupChat bool   `json:"curtailInGroupChat"`
}

type Page struct {
	Items        []User `json:"list"`
	NextBeforeID uint64 `json:"nextBeforeId"`
	HasMore      bool   `json:"hasMore"`
}

type Repository interface {
	List(context.Context, Query) ([]User, error)
	UpdateRestrictions(context.Context, Restrictions) error
	UpdateStatus(context.Context, uint64, domain.Status) error
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
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Search) > 64 || (query.Status != 0 && !query.Status.Valid()) {
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
		for index := range rows {
			online, queryErr := service.presence.IsOnline(ctx, rows[index].ID, now)
			if queryErr == nil {
				rows[index].IsOnline = online
			}
		}
	}
	var next uint64
	if hasMore && len(rows) > 0 {
		next = rows[len(rows)-1].ID
	}
	return Page{Items: rows, NextBeforeID: next, HasMore: hasMore}, nil
}

func (service *Service) UpdateRestrictions(ctx context.Context, input Restrictions) error {
	if input.UserID == 0 {
		return ErrInvalid
	}
	return service.repository.UpdateRestrictions(ctx, input)
}

func (service *Service) Disable(ctx context.Context, actorID, userID uint64) error {
	if actorID == 0 || userID == 0 {
		return ErrInvalid
	}
	if actorID == userID {
		return ErrSelfAction
	}
	return service.repository.UpdateStatus(ctx, userID, domain.StatusDisabled)
}

func (service *Service) Enable(ctx context.Context, actorID, userID uint64) error {
	if actorID == 0 || userID == 0 {
		return ErrInvalid
	}
	return service.repository.UpdateStatus(ctx, userID, domain.StatusActive)
}
