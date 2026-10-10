package presence

import (
	"context"
	"time"
)

// Store persists one Chat API instance's contribution to a user's global
// Presence. A user is globally online while any unexpired contribution exists.
type Store interface {
	MarkOnline(context.Context, uint64, string, time.Time, time.Duration) error
	MarkOffline(context.Context, uint64, string) error
	IsOnline(context.Context, uint64, time.Time) (bool, error)
}
