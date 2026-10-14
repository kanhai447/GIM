package repository

import (
	"context"
	"encoding/json"
)

type Value struct {
	Key  string
	JSON json.RawMessage
}
type Repository interface {
	ListPublic(context.Context, []string) (map[string]json.RawMessage, error)
	UpsertPublic(context.Context, []Value) error
}
