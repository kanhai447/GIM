package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
	"github.com/kanhai447/GIM/server/internal/logs/repository"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
)

type fakeRepository struct{ query domain.Query }

func (*fakeRepository) Insert(context.Context, audit.Event) (bool, error) { return true, nil }
func (repo *fakeRepository) List(_ context.Context, query domain.Query) (domain.Page, error) {
	repo.query = query
	return domain.Page{}, nil
}
func (*fakeRepository) Get(context.Context, uint64) (domain.Log, error) {
	return domain.Log{}, repository.ErrNotFound
}

func TestListValidationAndDefaults(t *testing.T) {
	repo := &fakeRepository{}
	service := New(repo)
	if _, err := service.List(context.Background(), domain.Query{}); err != nil || repo.query.Limit != 50 {
		t.Fatalf("default query = %#v, %v", repo.query, err)
	}
	for _, query := range []domain.Query{{Limit: 101}, {Limit: -1}, {RequestID: "bad request id"}, {Status: "success' OR 1=1"}} {
		if _, err := service.List(context.Background(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("query %#v error = %v", query, err)
		}
	}
}
