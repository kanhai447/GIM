// Package repository defines persistence boundaries for the user domain.
package repository

import (
	"context"
	"errors"

	"github.com/kanhai447/GIM/server/internal/user/domain"
)

var (
	ErrNotFound         = errors.New("user repository: not found")
	ErrDuplicateAccount = errors.New("user repository: duplicate account")
)

type Repository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, userID uint64) (domain.User, error)
	GetByAccount(ctx context.Context, account string) (domain.User, error)
}
