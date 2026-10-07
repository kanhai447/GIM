package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/kanhai447/GIM/server/internal/user/repository"
)

const testPasswordHash = "$2a$12$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWX"

type fakeRepository struct {
	create       func(context.Context, *domain.User) error
	getByID      func(context.Context, uint64) (domain.User, error)
	getByAccount func(context.Context, string) (domain.User, error)
}

func (repo fakeRepository) Create(ctx context.Context, user *domain.User) error {
	return repo.create(ctx, user)
}

func (repo fakeRepository) GetByID(ctx context.Context, userID uint64) (domain.User, error) {
	return repo.getByID(ctx, userID)
}

func (repo fakeRepository) GetByAccount(ctx context.Context, account string) (domain.User, error) {
	return repo.getByAccount(ctx, account)
}

func TestCreateUser(t *testing.T) {
	type contextKey string
	const key contextKey = "trace"
	ctx := context.WithValue(context.Background(), key, "checkpoint-3")
	service := New(fakeRepository{
		create: func(received context.Context, user *domain.User) error {
			if received.Value(key) != "checkpoint-3" {
				t.Fatal("CreateUser did not propagate the caller context")
			}
			if user.PasswordHash != testPasswordHash {
				t.Fatal("CreateUser changed the Auth-provided password hash")
			}
			user.ID = 1001
			return nil
		},
	})

	user, err := service.CreateUser(ctx, CreateUserInput{
		Account:      "gim.user",
		Nickname:     "GIM 用户",
		PasswordHash: testPasswordHash,
		Role:         domain.RoleMember,
		Status:       domain.StatusActive,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.ID != 1001 || user.Account != "gim.user" {
		t.Fatalf("CreateUser() = %#v", user)
	}
}

func TestCreateUserDuplicateAccount(t *testing.T) {
	service := New(fakeRepository{create: func(context.Context, *domain.User) error {
		return repository.ErrDuplicateAccount
	}})
	_, err := service.CreateUser(context.Background(), validCreateInput())
	_, code, message := apperror.PublicFields(err)
	if code != CodeDuplicateAccount || message != "账号已存在" {
		t.Fatalf("CreateUser() public error = %d, %q", code, message)
	}
}

func TestGetUserByID(t *testing.T) {
	service := New(fakeRepository{getByID: func(_ context.Context, userID uint64) (domain.User, error) {
		return domain.User{ID: userID, Account: "gim-user", PasswordHash: testPasswordHash}, nil
	}})
	user, err := service.GetUserByID(context.Background(), 9)
	if err != nil || user.ID != 9 {
		t.Fatalf("GetUserByID() = %#v, %v", user, err)
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	service := New(fakeRepository{getByID: func(context.Context, uint64) (domain.User, error) {
		return domain.User{}, repository.ErrNotFound
	}})
	_, err := service.GetUserByID(context.Background(), 9)
	_, code, message := apperror.PublicFields(err)
	if code != CodeUserNotFound || message != "用户不存在" {
		t.Fatalf("GetUserByID() public error = %d, %q", code, message)
	}
}

func TestGetUserByAccount(t *testing.T) {
	service := New(fakeRepository{getByAccount: func(_ context.Context, account string) (domain.User, error) {
		return domain.User{ID: 9, Account: account, PasswordHash: testPasswordHash}, nil
	}})
	user, err := service.GetUserByAccount(context.Background(), "gim-user")
	if err != nil || user.Account != "gim-user" || user.PasswordHash != testPasswordHash {
		t.Fatalf("GetUserByAccount() = %#v, %v", user, err)
	}
}

func TestPublicUserInfoDoesNotContainPasswordHash(t *testing.T) {
	user := domain.User{ID: 9, Account: "gim-user", PasswordHash: testPasswordHash}.PublicInfo()
	payload, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(payload), testPasswordHash) || strings.Contains(strings.ToLower(string(payload)), "password") {
		t.Fatal("public UserInfo exposed password material")
	}
}

func TestRepositoryErrorDoesNotLeak(t *testing.T) {
	privateDetail := "database password=private-test-value"
	service := New(fakeRepository{getByID: func(context.Context, uint64) (domain.User, error) {
		return domain.User{}, errors.New(privateDetail)
	}})
	_, err := service.GetUserByID(context.Background(), 9)
	status, code, message := apperror.PublicFields(err)
	if status != http.StatusInternalServerError || code != apperror.CodeInternal || strings.Contains(message, privateDetail) {
		t.Fatalf("GetUserByID() leaked repository error: status=%d code=%d message=%q", status, code, message)
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || errors.Unwrap(err) == nil {
		t.Fatal("GetUserByID() did not retain a private application error cause")
	}
}

func TestCanceledContextDoesNotCallRepository(t *testing.T) {
	called := false
	service := New(fakeRepository{getByID: func(context.Context, uint64) (domain.User, error) {
		called = true
		return domain.User{}, nil
	}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.GetUserByID(ctx, 9)
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("GetUserByID() error = %v, repository called = %t", err, called)
	}
}

func validCreateInput() CreateUserInput {
	return CreateUserInput{
		Account:      "gim-user",
		Nickname:     "GIM User",
		PasswordHash: testPasswordHash,
		Role:         domain.RoleMember,
		Status:       domain.StatusActive,
	}
}
