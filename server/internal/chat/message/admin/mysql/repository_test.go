package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	chatadmin "github.com/kanhai447/GIM/server/internal/chat/message/admin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// injectedFailure simulates a database that fails every statement. It lets the
// session-repair error handling be tested without a live MySQL instance.
var injectedFailure = errors.New("injected database failure")

type failingPool struct{}

func (failingPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, injectedFailure
}

func (failingPool) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, injectedFailure
}

func (failingPool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, injectedFailure
}

func (failingPool) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return &sql.Row{}
}

func failingDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(
		mysql.New(mysql.Config{Conn: failingPool{}, SkipInitializeWithVersion: true}),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open failing gorm database: %v", err)
	}
	return database
}

// TestRepairSessionPropagatesSessionLockFailure is the C1 regression test.
//
// Before the fix, a non-RecordNotFound error while locking the session left the
// zero-valued result in place, so `session.LastMessageID == nil` matched first
// and repairSession reported success. Admin deletion then committed the
// tombstone without repairing the session pointer.
func TestRepairSessionPropagatesSessionLockFailure(t *testing.T) {
	err := repairSession(failingDatabase(t), 2, 1, 99)
	if err == nil {
		t.Fatal("repairSession swallowed a session lock failure")
	}
	if !errors.Is(err, injectedFailure) && !strings.Contains(err.Error(), injectedFailure.Error()) {
		t.Fatalf("repairSession error = %v, want the injected database failure", err)
	}
}

func deadlockError() error {
	return &mysqldriver.MySQLError{Number: 1213, Message: "Deadlock found when trying to get lock"}
}

func TestIsDeadlockOnlyMatchesMySQL1213(t *testing.T) {
	if !isDeadlock(deadlockError()) {
		t.Fatal("MySQL 1213 must be classified as a retryable deadlock")
	}
	if !isDeadlock(&wrappedError{cause: deadlockError()}) {
		t.Fatal("wrapped MySQL 1213 must still be retryable")
	}
	if isDeadlock(&mysqldriver.MySQLError{Number: 1205}) {
		t.Fatal("lock wait timeout must not use the deadlock retry")
	}
	if isDeadlock(context.Canceled) || isDeadlock(nil) {
		t.Fatal("unrelated errors must not be classified as deadlocks")
	}
}

type wrappedError struct{ cause error }

func (err *wrappedError) Error() string { return "wrapped: " + err.cause.Error() }
func (err *wrappedError) Unwrap() error { return err.cause }

// TestSessionLockOrderMatchesSenderOrdering documents the C2 invariant: admin
// deletion must take the two session row locks in the same ascending user-ID
// order as message persistence.
func TestSessionLockOrderMatchesSenderOrdering(t *testing.T) {
	for _, test := range []struct {
		name             string
		sender, receiver uint64
		want             [][2]uint64
	}{
		{name: "sender lower", sender: 3, receiver: 9, want: [][2]uint64{{3, 9}, {9, 3}}},
		{name: "sender higher", sender: 9, receiver: 3, want: [][2]uint64{{3, 9}, {9, 3}}},
	} {
		got := sessionLockOrder(test.sender, test.receiver)
		if len(got) != 2 || got[0] != test.want[0] || got[1] != test.want[1] {
			t.Fatalf("%s: sessionLockOrder(%d,%d) = %v, want %v", test.name, test.sender, test.receiver, got, test.want)
		}
		if got[0][0] >= got[1][0] {
			t.Fatalf("%s: session locks are not in ascending user-ID order: %v", test.name, got)
		}
	}
}

func TestRetryDeadlockIsBoundedAndContextAware(t *testing.T) {
	t.Run("retries at most three times", func(t *testing.T) {
		attempts := 0
		err := retryDeadlock(context.Background(), func(context.Context) error {
			attempts++
			return deadlockError()
		})
		if err == nil || attempts != 3 {
			t.Fatalf("attempts = %d, err = %v; want 3 attempts and a retry error", attempts, err)
		}
		if !isDeadlock(err) {
			t.Fatalf("retry error lost the deadlock cause: %v", err)
		}
	})

	t.Run("stops after a success", func(t *testing.T) {
		attempts := 0
		err := retryDeadlock(context.Background(), func(context.Context) error {
			attempts++
			if attempts < 3 {
				return deadlockError()
			}
			return nil
		})
		if err != nil || attempts != 3 {
			t.Fatalf("attempts = %d, err = %v; want 3 attempts and no error", attempts, err)
		}
	})

	t.Run("does not retry other errors", func(t *testing.T) {
		attempts := 0
		err := retryDeadlock(context.Background(), func(context.Context) error {
			attempts++
			return chatadmin.ErrNotFound
		})
		if !errors.Is(err, chatadmin.ErrNotFound) || attempts != 1 {
			t.Fatalf("attempts = %d, err = %v; want 1 attempt and the original error", attempts, err)
		}
	})

	t.Run("cancellation stops the retry loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		err := retryDeadlock(ctx, func(context.Context) error {
			attempts++
			cancel()
			return deadlockError()
		})
		if !errors.Is(err, context.Canceled) || attempts != 1 {
			t.Fatalf("attempts = %d, err = %v; want cancellation after 1 attempt", attempts, err)
		}
	})
}

func TestDeadlockBackoffIsBounded(t *testing.T) {
	if deadlockBackoff(1) != 10*time.Millisecond || deadlockBackoff(2) != 20*time.Millisecond {
		t.Fatalf("unexpected deadlock backoff: %s / %s", deadlockBackoff(1), deadlockBackoff(2))
	}
}
