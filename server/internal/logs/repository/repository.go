package repository

import (
	"context"
	"errors"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
)

var ErrNotFound = errors.New("audit log not found")

type Repository interface {
	Insert(context.Context, audit.Event) (bool, error)
	List(context.Context, domain.Query) (domain.Page, error)
	Get(context.Context, uint64) (domain.Log, error)
}
