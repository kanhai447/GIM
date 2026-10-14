package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/file/domain"
)

var (
	ErrInvalid  = errors.New("invalid file admin request")
	ErrNotFound = errors.New("admin file not found")
)

type Query struct {
	BeforeID uint64
	Limit    int
	UserID   uint64
	Search   string
}

type File struct {
	ID           uint64              `json:"id"`
	UserID       uint64              `json:"userId"`
	UserNickname string              `json:"userNickname"`
	UID          string              `json:"uid"`
	FileName     string              `json:"fileName"`
	Size         int64               `json:"size"`
	MIMEType     string              `json:"mimeType"`
	SHA256       string              `json:"sha256"`
	DownloadURL  string              `json:"downloadUrl"`
	CreatedAt    time.Time           `json:"createdAt"`
	Metadata     domain.FileMetadata `json:"-"`
}
type Page struct {
	Items        []File `json:"list"`
	NextBeforeID uint64 `json:"nextBeforeId"`
	HasMore      bool   `json:"hasMore"`
}

type Repository interface {
	List(context.Context, Query) ([]File, error)
	Get(context.Context, uint64) (File, error)
	Delete(context.Context, uint64) error
}
type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }
func (service *Service) List(ctx context.Context, query Query) (Page, error) {
	query.Search = strings.TrimSpace(query.Search)
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Search) > 128 {
		return Page{}, ErrInvalid
	}
	rows, err := service.repository.List(ctx, Query{BeforeID: query.BeforeID, Limit: query.Limit + 1, UserID: query.UserID, Search: query.Search})
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
func (service *Service) Get(ctx context.Context, id uint64) (File, error) {
	if id == 0 {
		return File{}, ErrInvalid
	}
	return service.repository.Get(ctx, id)
}
func (service *Service) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return ErrInvalid
	}
	return service.repository.Delete(ctx, id)
}
