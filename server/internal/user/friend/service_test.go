package friend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user/domain"
)

type fakeRepository struct {
	searchUsers     func(context.Context, uint64, string, int) ([]domain.User, error)
	listFriends     func(context.Context, uint64, int) ([]FriendRow, error)
	getFriend       func(context.Context, uint64, uint64) (FriendRow, error)
	friendshipFlags func(context.Context, uint64, []uint64) (map[uint64]bool, error)
	createRequest   func(context.Context, uint64, uint64, string) (VerificationRow, error)
	listRequests    func(context.Context, uint64, int, int) ([]VerificationRow, error)
	respond         func(context.Context, uint64, uint64, bool) (VerificationRow, error)
	updateRemark    func(context.Context, uint64, uint64, string) error
	deleteFriend    func(context.Context, uint64, uint64) error
}

func (repository fakeRepository) SearchUsers(ctx context.Context, userID uint64, keyword string, limit int) ([]domain.User, error) {
	return repository.searchUsers(ctx, userID, keyword, limit)
}

func (repository fakeRepository) ListFriends(ctx context.Context, userID uint64, limit int) ([]FriendRow, error) {
	return repository.listFriends(ctx, userID, limit)
}

func (repository fakeRepository) GetFriend(ctx context.Context, userID, friendID uint64) (FriendRow, error) {
	return repository.getFriend(ctx, userID, friendID)
}

func (repository fakeRepository) FriendshipFlags(ctx context.Context, userID uint64, otherIDs []uint64) (map[uint64]bool, error) {
	return repository.friendshipFlags(ctx, userID, otherIDs)
}

func (repository fakeRepository) CreateRequest(ctx context.Context, requesterID, receiverID uint64, message string) (VerificationRow, error) {
	return repository.createRequest(ctx, requesterID, receiverID, message)
}

func (repository fakeRepository) ListRequests(ctx context.Context, receiverID uint64, status, limit int) ([]VerificationRow, error) {
	return repository.listRequests(ctx, receiverID, status, limit)
}

func (repository fakeRepository) Respond(ctx context.Context, requestID, reviewerID uint64, accept bool) (VerificationRow, error) {
	return repository.respond(ctx, requestID, reviewerID, accept)
}

func (repository fakeRepository) UpdateRemark(ctx context.Context, userID, friendID uint64, remark string) error {
	return repository.updateRemark(ctx, userID, friendID, remark)
}

func (repository fakeRepository) DeleteFriend(ctx context.Context, userID, friendID uint64) error {
	return repository.deleteFriend(ctx, userID, friendID)
}

type fakeDirectory struct {
	users map[uint64]domain.User
	err   error
}

func (directory fakeDirectory) GetUserByID(_ context.Context, userID uint64) (domain.User, error) {
	if directory.err != nil {
		return domain.User{}, directory.err
	}
	user, ok := directory.users[userID]
	if !ok {
		return domain.User{}, apperror.Business(1101, "用户不存在")
	}
	return user, nil
}

type fakePresence struct {
	online map[uint64]bool
	err    error
}

func (presence fakePresence) IsOnline(_ context.Context, userID uint64, _ time.Time) (bool, error) {
	if presence.err != nil {
		return false, presence.err
	}
	return presence.online[userID], nil
}

func publicCode(t *testing.T, err error) uint32 {
	t.Helper()
	if err == nil {
		t.Fatal("expected a public error, got nil")
	}
	_, code, _ := apperror.PublicFields(err)
	return code
}

func newTestService(repository Repository, directory Directory, presence Presence) *Service {
	service := New(repository, directory, presence)
	service.now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	return service
}

func TestSearchExcludesSelfAndMarksFriends(t *testing.T) {
	var receivedKeyword string
	var receivedLimit int
	service := newTestService(fakeRepository{
		searchUsers: func(_ context.Context, userID uint64, keyword string, limit int) ([]domain.User, error) {
			receivedKeyword, receivedLimit = keyword, limit
			return []domain.User{
				{ID: userID, Nickname: "self"},
				{ID: 8, Nickname: "Alice", Account: "alice"},
				{ID: 9, Nickname: "Bob", Account: "bob"},
			}, nil
		},
		friendshipFlags: func(_ context.Context, _ uint64, ids []uint64) (map[uint64]bool, error) {
			if len(ids) != 2 {
				t.Fatalf("flag ids = %v, want only the two other users", ids)
			}
			return map[uint64]bool{8: true}, nil
		},
	}, fakeDirectory{}, fakePresence{})

	results, err := service.Search(context.Background(), 7, "  ali  ", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if receivedKeyword != "ali" || receivedLimit != 10 {
		t.Fatalf("repository received keyword=%q limit=%d", receivedKeyword, receivedLimit)
	}
	if len(results) != 2 || results[0].UserID != 8 || !results[0].IsFriend || results[1].IsFriend {
		t.Fatalf("Search() results = %#v", results)
	}
}

func TestSearchValidation(t *testing.T) {
	service := newTestService(fakeRepository{
		searchUsers: func(context.Context, uint64, string, int) ([]domain.User, error) { return nil, nil },
		friendshipFlags: func(context.Context, uint64, []uint64) (map[uint64]bool, error) {
			return map[uint64]bool{}, nil
		},
	}, fakeDirectory{}, nil)
	if _, err := service.Search(context.Background(), 7, "   ", 10); publicCode(t, err) != CodeInvalid {
		t.Fatalf("blank keyword error = %v", err)
	}
	if _, err := service.Search(context.Background(), 7, "ok", 0); err != nil {
		t.Fatalf("limit 0 should fall back to the default: %v", err)
	}
}

func TestSendRequestPaths(t *testing.T) {
	created := VerificationRow{ID: 5, RequesterID: 7, ReceiverID: 8, Status: StatusPending, Message: "hi", RequesterNickname: "Seven"}
	service := newTestService(fakeRepository{
		createRequest: func(_ context.Context, requesterID, receiverID uint64, message string) (VerificationRow, error) {
			if requesterID != 7 || receiverID != 8 || message != "hi" {
				t.Fatalf("CreateRequest(%d,%d,%q)", requesterID, receiverID, message)
			}
			return created, nil
		},
	}, fakeDirectory{users: map[uint64]domain.User{8: {ID: 8, Nickname: "Eight"}}}, nil)

	request, err := service.SendRequest(context.Background(), 7, 8, " hi ")
	if err != nil || request.ID != 5 || request.Status != StatusPending || request.Requester.UserID != 7 {
		t.Fatalf("SendRequest() = %#v, err=%v", request, err)
	}

	if _, err := service.SendRequest(context.Background(), 7, 7, ""); publicCode(t, err) != CodeSelfTarget {
		t.Fatalf("self request error = %v", err)
	}
	if _, err := service.SendRequest(context.Background(), 7, 0, ""); publicCode(t, err) != CodeInvalid {
		t.Fatalf("zero target error = %v", err)
	}
	if _, err := service.SendRequest(context.Background(), 7, 8, string(make([]rune, maxMessageRunes+1))); publicCode(t, err) != CodeInvalid {
		t.Fatalf("long message error = %v", err)
	}
}

func TestSendRequestRepositoryErrorsAreMapped(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code uint32
	}{
		{name: "already friend", err: ErrAlreadyFriend, code: CodeAlreadyFriend},
		{name: "duplicate request", err: ErrDuplicateRequest, code: CodeDuplicateRequest},
	} {
		service := newTestService(fakeRepository{
			createRequest: func(context.Context, uint64, uint64, string) (VerificationRow, error) {
				return VerificationRow{}, test.err
			},
		}, fakeDirectory{users: map[uint64]domain.User{8: {ID: 8}}}, nil)
		if _, err := service.SendRequest(context.Background(), 7, 8, ""); publicCode(t, err) != test.code {
			t.Fatalf("%s: error = %v, want code %d", test.name, err, test.code)
		}
	}
}

func TestSendRequestUnknownTarget(t *testing.T) {
	service := newTestService(fakeRepository{}, fakeDirectory{users: map[uint64]domain.User{}}, nil)
	if _, err := service.SendRequest(context.Background(), 7, 99, ""); publicCode(t, err) != 1101 {
		t.Fatalf("unknown target error = %v", err)
	}
}

func TestListRequestsStatusValidationAndDTO(t *testing.T) {
	service := newTestService(fakeRepository{
		listRequests: func(_ context.Context, receiverID uint64, status, limit int) ([]VerificationRow, error) {
			if receiverID != 7 || status != -1 || limit != DefaultLimit {
				t.Fatalf("ListRequests(%d,%d,%d)", receiverID, status, limit)
			}
			return []VerificationRow{{ID: 3, RequesterID: 8, ReceiverID: 7, Status: StatusPending, RequesterNickname: "Eight"}}, nil
		},
	}, fakeDirectory{}, nil)

	items, err := service.ListRequests(context.Background(), 7, -1, 0)
	if err != nil || len(items) != 1 || items[0].Requester.UserID != 8 || items[0].Status != StatusPending {
		t.Fatalf("ListRequests() = %#v, err=%v", items, err)
	}
	if _, err := service.ListRequests(context.Background(), 7, 9, 20); publicCode(t, err) != CodeInvalid {
		t.Fatalf("invalid status error = %v", err)
	}
}

func TestRespondMapsReviewErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code uint32
	}{
		{name: "not receiver", err: ErrNotReceiver, code: CodeNotReceiver},
		{name: "handled", err: ErrRequestHandled, code: CodeRequestHandled},
		{name: "missing", err: ErrRequestNotFound, code: CodeRequestNotFound},
	} {
		service := newTestService(fakeRepository{
			respond: func(context.Context, uint64, uint64, bool) (VerificationRow, error) {
				return VerificationRow{}, test.err
			},
		}, fakeDirectory{}, nil)
		if _, err := service.Respond(context.Background(), 7, 3, StatusAccepted); publicCode(t, err) != test.code {
			t.Fatalf("%s: error = %v", test.name, err)
		}
	}
	if _, err := newTestService(fakeRepository{}, fakeDirectory{}, nil).Respond(context.Background(), 7, 3, 7); publicCode(t, err) != CodeInvalid {
		t.Fatal("invalid respond status must be rejected")
	}
}

func TestListFriendsUsesPresenceAndDegrades(t *testing.T) {
	rows := []FriendRow{{FriendID: 8, Nickname: "Eight", Remark: "A", CreatedAt: time.Unix(1, 0)}}
	repository := fakeRepository{listFriends: func(context.Context, uint64, int) ([]FriendRow, error) { return rows, nil }}

	online := newTestService(repository, fakeDirectory{}, fakePresence{online: map[uint64]bool{8: true}})
	items, err := online.ListFriends(context.Background(), 7, 20)
	if err != nil || len(items) != 1 || !items[0].IsOnline || items[0].Remark != "A" {
		t.Fatalf("ListFriends() = %#v, err=%v", items, err)
	}

	degraded := newTestService(repository, fakeDirectory{}, fakePresence{err: errors.New("redis down")})
	items, err = degraded.ListFriends(context.Background(), 7, 20)
	if err != nil || len(items) != 1 || items[0].IsOnline {
		t.Fatalf("presence failure must degrade to offline: %#v, err=%v", items, err)
	}
}

func TestFriendInfoAndRemarkAndDelete(t *testing.T) {
	row := FriendRow{FriendID: 8, Nickname: "Eight"}
	service := newTestService(fakeRepository{
		getFriend:    func(context.Context, uint64, uint64) (FriendRow, error) { return row, nil },
		updateRemark: func(context.Context, uint64, uint64, string) error { return nil },
		deleteFriend: func(context.Context, uint64, uint64) error { return nil },
	}, fakeDirectory{}, nil)

	if item, err := service.FriendInfo(context.Background(), 7, 8); err != nil || item.UserID != 8 {
		t.Fatalf("FriendInfo() = %#v, err=%v", item, err)
	}
	if item, err := service.UpdateRemark(context.Background(), 7, 8, " buddy "); err != nil || item.UserID != 8 {
		t.Fatalf("UpdateRemark() = %#v, err=%v", item, err)
	}
	if err := service.DeleteFriend(context.Background(), 7, 8); err != nil {
		t.Fatalf("DeleteFriend() error = %v", err)
	}
	if _, err := service.UpdateRemark(context.Background(), 7, 8, string(make([]rune, maxRemarkRunes+1))); publicCode(t, err) != CodeInvalid {
		t.Fatalf("long remark error = %v", err)
	}
}

func TestNotFriendErrorsAreStable(t *testing.T) {
	service := newTestService(fakeRepository{
		getFriend:    func(context.Context, uint64, uint64) (FriendRow, error) { return FriendRow{}, ErrNotFriend },
		updateRemark: func(context.Context, uint64, uint64, string) error { return ErrNotFriend },
		deleteFriend: func(context.Context, uint64, uint64) error { return ErrNotFriend },
	}, fakeDirectory{}, nil)

	if _, err := service.FriendInfo(context.Background(), 7, 8); publicCode(t, err) != CodeNotFriend {
		t.Fatalf("FriendInfo error = %v", err)
	}
	if _, err := service.UpdateRemark(context.Background(), 7, 8, "x"); publicCode(t, err) != CodeNotFriend {
		t.Fatalf("UpdateRemark error = %v", err)
	}
	if err := service.DeleteFriend(context.Background(), 7, 8); publicCode(t, err) != CodeNotFriend {
		t.Fatalf("DeleteFriend error = %v", err)
	}
}
