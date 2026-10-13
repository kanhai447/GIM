// Package repository defines persistence boundaries for file objects and user-owned references.
package repository

import (
	"context"
	"errors"

	"github.com/kanhai447/GIM/server/internal/file/domain"
)

var ErrNotFound = errors.New("file repository: not found")

type FileObjectRepository interface {
	GetBySHA256(context.Context, string) (domain.FileObject, error)
	GetByID(context.Context, uint64) (domain.FileObject, error)
	GetByUID(context.Context, string) (domain.FileObject, error)
	Create(context.Context, *domain.FileObject) error
	CountReferences(context.Context, uint64) (uint64, error)
}

type UserFileRepository interface {
	CreateUserFile(context.Context, *domain.UserFile) error
	GetOwnedByID(context.Context, uint64, uint64) (domain.FileMetadata, error)
	GetOwnedByUID(context.Context, uint64, string) (domain.FileMetadata, error)
	ListByUser(context.Context, uint64, uint64, int) ([]domain.FileMetadata, error)
	DeleteReference(context.Context, uint64, uint64) error
	Exists(context.Context, uint64, uint64) (bool, error)
}

type Repository interface {
	FileObjectRepository
	UserFileRepository
	// Attach atomically resolves/creates the content object and creates/reuses
	// the user's logical reference. Database unique keys remain the final race guard.
	Attach(context.Context, domain.FileObject, domain.UserFile) (domain.FileMetadata, bool, bool, error)
}
