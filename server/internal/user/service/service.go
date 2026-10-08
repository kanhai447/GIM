// Package service contains transport-independent user business logic.
package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/kanhai447/GIM/server/internal/user/repository"
)

const (
	CodeInvalidArgument  uint32 = 1001
	CodeUserNotFound     uint32 = 1101
	CodeDuplicateAccount uint32 = 1102
)

type Service struct {
	users repository.Repository
}

type CreateUserInput struct {
	Account      string
	Nickname     string
	PasswordHash string
	Avatar       string
	Role         domain.Role
	Status       domain.Status
}

func New(users repository.Repository) *Service {
	return &Service{users: users}
}

// CreateUser accepts an already-hashed password. Hashing and plaintext password
// validation belong to Auth; User never hashes or logs credentials again.
func (service *Service) CreateUser(ctx context.Context, input CreateUserInput) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	input.Account = strings.TrimSpace(input.Account)
	input.Nickname = strings.TrimSpace(input.Nickname)
	if !validAccount(input.Account) || utf8.RuneCountInString(input.Nickname) < 1 || utf8.RuneCountInString(input.Nickname) > 32 || len(input.PasswordHash) < 20 || len(input.PasswordHash) > 255 || !input.Role.Valid() || !input.Status.Valid() {
		return domain.User{}, apperror.Business(CodeInvalidArgument, "参数错误")
	}

	user := domain.User{
		Account:        input.Account,
		Nickname:       input.Nickname,
		PasswordHash:   input.PasswordHash,
		Avatar:         strings.TrimSpace(input.Avatar),
		RegisterSource: "account",
		Role:           input.Role,
		Status:         input.Status,
	}
	if err := service.users.Create(ctx, &user); err != nil {
		return domain.User{}, translateRepositoryError(err)
	}
	return user, nil
}

func (service *Service) GetUserByID(ctx context.Context, userID uint64) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	if userID == 0 {
		return domain.User{}, apperror.Business(CodeInvalidArgument, "参数错误")
	}
	user, err := service.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, translateRepositoryError(err)
	}
	return user, nil
}

func (service *Service) GetUserByAccount(ctx context.Context, account string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	account = strings.TrimSpace(account)
	if !validAccount(account) {
		return domain.User{}, apperror.Business(CodeInvalidArgument, "参数错误")
	}
	user, err := service.users.GetByAccount(ctx, account)
	if err != nil {
		return domain.User{}, translateRepositoryError(err)
	}
	return user, nil
}

func validAccount(account string) bool {
	if len(account) < 3 || len(account) > 64 {
		return false
	}
	for _, character := range account {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func translateRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrDuplicateAccount):
		return apperror.Business(CodeDuplicateAccount, "账号已存在")
	case errors.Is(err, repository.ErrNotFound):
		return apperror.Business(CodeUserNotFound, "用户不存在")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperror.Wrap(err, http.StatusInternalServerError, apperror.CodeInternal, "服务器错误")
	}
}
