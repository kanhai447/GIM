package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
	"github.com/kanhai447/GIM/server/internal/logs/repository"
)

var (
	ErrInvalidQuery = errors.New("invalid audit log query")
	filterPattern   = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,128}$`)
)

type Service struct{ repository repository.Repository }

func New(repo repository.Repository) *Service { return &Service{repository: repo} }

func (service *Service) List(ctx context.Context, query domain.Query) (domain.Page, error) {
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 || query.From != nil && query.To != nil && query.From.After(*query.To) ||
		!validFilter(query.Service) || !validFilter(query.EventType) || !validFilter(query.Status) || !validFilter(query.RequestID) {
		return domain.Page{}, ErrInvalidQuery
	}
	page, err := service.repository.List(ctx, query)
	if errors.Is(err, repository.ErrNotFound) {
		return domain.Page{}, ErrInvalidQuery
	}
	return page, err
}

func (service *Service) Get(ctx context.Context, id uint64) (domain.Log, error) {
	if id == 0 {
		return domain.Log{}, ErrInvalidQuery
	}
	return service.repository.Get(ctx, id)
}

func ParseTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, ErrInvalidQuery
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func validFilter(value string) bool {
	return value == "" || filterPattern.MatchString(value)
}
