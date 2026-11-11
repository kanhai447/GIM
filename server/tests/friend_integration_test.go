package tests

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user"
	"github.com/kanhai447/GIM/server/internal/user/friend"
	friendmysql "github.com/kanhai447/GIM/server/internal/user/friend/repository/mysql"
)

// TestFriendLifecycleMySQLAndConcurrency exercises the friend lifecycle against
// a real MySQL schema: search, request, concurrent acceptance, remark, deletion
// and the reverse/duplicate guards. It is skipped unless GIM_ENV_FILE is set.
func TestFriendLifecycleMySQLAndConcurrency(t *testing.T) {
	gormDB, rawDB, cleanup := openAdminTestDatabase(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	seedFriendUsers(t, ctx, rawDB)
	service := friend.New(friendmysql.New(gormDB), user.New(gormDB).Service, nil)

	results, err := service.Search(ctx, 1, "friend", 10)
	if err != nil || len(results) != 3 {
		t.Fatalf("search results = %#v, err = %v", results, err)
	}
	for _, result := range results {
		if result.UserID == 1 {
			t.Fatal("search returned the caller")
		}
		if result.IsFriend {
			t.Fatalf("unexpected existing friendship: %#v", result)
		}
	}

	created, err := service.SendRequest(ctx, 1, 2, "hello")
	if err != nil || created.Status != friend.StatusPending || created.Requester.UserID != 1 {
		t.Fatalf("SendRequest() = %#v, err = %v", created, err)
	}
	if _, err := service.SendRequest(ctx, 1, 2, ""); friendErrorCode(t, err) != friend.CodeDuplicateRequest {
		t.Fatalf("duplicate request code = %d, want %d", friendErrorCode(t, err), friend.CodeDuplicateRequest)
	}
	if _, err := service.SendRequest(ctx, 2, 1, ""); friendErrorCode(t, err) != friend.CodeDuplicateRequest {
		t.Fatalf("reverse duplicate request code = %d, want %d", friendErrorCode(t, err), friend.CodeDuplicateRequest)
	}
	if _, err := service.Respond(ctx, 3, created.ID, friend.StatusAccepted); friendErrorCode(t, err) != friend.CodeNotReceiver {
		t.Fatalf("non-receiver response code = %d, want %d", friendErrorCode(t, err), friend.CodeNotReceiver)
	}

	// Two concurrent reviewers must converge on one accepted request and one
	// handled error; the friendship rows must be written exactly once.
	type outcome struct {
		err  error
		item friend.Request
	}
	outcomes := make(chan outcome, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			item, respondErr := service.Respond(ctx, 2, created.ID, friend.StatusAccepted)
			outcomes <- outcome{err: respondErr, item: item}
		}()
	}
	wait.Wait()
	close(outcomes)

	successes, handled, other := 0, 0, 0
	for result := range outcomes {
		switch {
		case result.err == nil:
			successes++
			if result.item.Status != friend.StatusAccepted {
				t.Fatalf("accepted request status = %d", result.item.Status)
			}
		case friendErrorCode(t, result.err) == friend.CodeRequestHandled:
			handled++
		default:
			other++
			t.Logf("unexpected concurrent response error: %v", result.err)
		}
	}
	if successes != 1 || handled != 1 || other != 0 {
		t.Fatalf("concurrent accept: success=%d handled=%d other=%d", successes, handled, other)
	}
	if rows := friendRowCount(t, ctx, rawDB, 1, 2); rows != 2 {
		t.Fatalf("friendship rows = %d, want 2", rows)
	}

	results, err = service.Search(ctx, 1, "friend", 10)
	if err != nil || len(results) != 3 {
		t.Fatalf("search after accept = %#v, err = %v", results, err)
	}
	for _, result := range results {
		if result.UserID == 2 && !result.IsFriend {
			t.Fatal("accepted friend was not flagged by search")
		}
	}
	if _, err := service.SendRequest(ctx, 1, 2, ""); friendErrorCode(t, err) != friend.CodeAlreadyFriend {
		t.Fatalf("request to existing friend code = %d, want %d", friendErrorCode(t, err), friend.CodeAlreadyFriend)
	}

	friends, err := service.ListFriends(ctx, 1, 50)
	if err != nil || len(friends) != 1 || friends[0].UserID != 2 || friends[0].IsOnline {
		t.Fatalf("ListFriends() = %#v, err = %v", friends, err)
	}
	updated, err := service.UpdateRemark(ctx, 1, 2, "buddy")
	if err != nil || updated.Remark != "buddy" {
		t.Fatalf("UpdateRemark() = %#v, err = %v", updated, err)
	}
	peer, err := service.FriendInfo(ctx, 2, 1)
	if err != nil || peer.UserID != 1 || peer.Remark != "" {
		t.Fatalf("peer FriendInfo() = %#v, err = %v", peer, err)
	}

	if err := service.DeleteFriend(ctx, 1, 2); err != nil {
		t.Fatalf("DeleteFriend() error = %v", err)
	}
	if rows := friendRowCount(t, ctx, rawDB, 1, 2); rows != 0 {
		t.Fatalf("friendship rows after delete = %d", rows)
	}
	if err := service.DeleteFriend(ctx, 1, 2); friendErrorCode(t, err) != friend.CodeNotFriend {
		t.Fatalf("second delete code = %d, want %d", friendErrorCode(t, err), friend.CodeNotFriend)
	}

	rejected, err := service.SendRequest(ctx, 3, 4, "")
	if err != nil {
		t.Fatalf("reject flow request: %v", err)
	}
	if _, err := service.Respond(ctx, 4, rejected.ID, friend.StatusRejected); err != nil {
		t.Fatalf("reject request: %v", err)
	}
	if rows := friendRowCount(t, ctx, rawDB, 3, 4); rows != 0 {
		t.Fatalf("rejected request created %d friendship rows", rows)
	}
	if _, err := service.Respond(ctx, 4, rejected.ID, friend.StatusAccepted); friendErrorCode(t, err) != friend.CodeRequestHandled {
		t.Fatalf("respond to handled request code = %d", friendErrorCode(t, err))
	}
}

func friendErrorCode(t *testing.T, err error) uint32 {
	t.Helper()
	if err == nil {
		return 0
	}
	_, code, _ := apperror.PublicFields(err)
	return code
}

func seedFriendUsers(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, row := range []struct {
		id       int
		account  string
		nickname string
	}{
		{1, "friend.one", "Friend One"},
		{2, "friend.two", "Friend Two"},
		{3, "friend.three", "Friend Three"},
		{4, "friend.four", "Friend Four"},
	} {
		mustExec(t, ctx, db, `INSERT INTO users(id,account,pwd_hash,nickname,role,status) VALUES (?,?,?,?,2,1)`,
			row.id, row.account, "integration-hash-not-plaintext", row.nickname)
	}
}

func friendRowCount(t *testing.T, ctx context.Context, db *sql.DB, first, second uint64) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM friends WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
		first, second, second, first).Scan(&count); err != nil {
		t.Fatalf("count friendship rows: %v", err)
	}
	return count
}
