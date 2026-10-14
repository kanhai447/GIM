package admin

import (
	"context"
	"testing"
)

type pagingRepository struct {
	query Query
	rows  []Message
}

func (repository *pagingRepository) List(_ context.Context, query Query) ([]Message, error) {
	repository.query = query
	return repository.rows, nil
}

func (*pagingRepository) Delete(context.Context, uint64) error { return nil }

func TestListUsesLimitPlusOneAndReturnsCursor(t *testing.T) {
	repository := &pagingRepository{rows: []Message{{ID: 30}, {ID: 20}, {ID: 10}}}
	page, err := New(repository).List(context.Background(), Query{BeforeID: 40, Limit: 2, UserID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if repository.query.BeforeID != 40 || repository.query.Limit != 3 || repository.query.UserID != 7 {
		t.Fatalf("repository query = %#v", repository.query)
	}
	if len(page.Items) != 2 || page.Items[0].ID != 30 || page.Items[1].ID != 20 || !page.HasMore || page.NextBeforeID != 20 {
		t.Fatalf("page = %#v", page)
	}
}

func TestListRejectsUnboundedLimit(t *testing.T) {
	if _, err := New(&pagingRepository{}).List(context.Background(), Query{Limit: 101}); err != ErrInvalid {
		t.Fatalf("List error = %v", err)
	}
}
