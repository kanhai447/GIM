// Package storage defines the intentionally small local-object storage boundary.
package storage

import (
	"context"
	"io"
)

type TempObject interface {
	io.Writer
	io.Seeker
	Close() error
}

type Object interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Storage interface {
	CreateTemp(context.Context) (TempObject, error)
	Commit(context.Context, TempObject, string) (bool, error)
	Abort(TempObject) error
	Open(context.Context, string) (Object, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
}
