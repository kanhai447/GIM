// Package userclient adapts the internal User gRPC contract for Auth.
package userclient

import (
	"context"
	"errors"
	"fmt"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrNotFound         = errors.New("user not found")
	ErrDuplicateAccount = errors.New("duplicate account")
)

type User struct {
	ID           uint64
	Account      string
	Nickname     string
	Avatar       string
	PasswordHash string
	Role         int32
	Status       int32
}

type CreateInput struct {
	Account      string
	Nickname     string
	Avatar       string
	PasswordHash string
	Role         int32
	Status       int32
}

type Client struct{ rpc userv1.UserServiceClient }

func New(rpc userv1.UserServiceClient) *Client { return &Client{rpc: rpc} }

func (client *Client) Create(ctx context.Context, input CreateInput) (User, error) {
	response, err := client.rpc.CreateUser(ctx, &userv1.CreateUserRequest{
		Account: input.Account, Nickname: input.Nickname, Avatar: input.Avatar,
		PasswordHash: input.PasswordHash, Role: input.Role, Status: input.Status,
	})
	if err != nil {
		return User{}, translate(err)
	}
	return User{
		ID: response.GetUserId(), Account: input.Account, Nickname: input.Nickname,
		Avatar: input.Avatar, Role: input.Role, Status: input.Status,
	}, nil
}

func (client *Client) GetByAccount(ctx context.Context, account string) (User, error) {
	response, err := client.rpc.GetUserByAccount(ctx, &userv1.GetUserByAccountRequest{Account: account})
	if err != nil {
		return User{}, translate(err)
	}
	credential := response.GetCredential()
	if credential == nil || credential.GetUser() == nil {
		return User{}, fmt.Errorf("user rpc returned an incomplete credential")
	}
	user := credential.GetUser()
	return User{
		ID: user.GetId(), Account: user.GetAccount(), Nickname: user.GetNickname(),
		Avatar: user.GetAvatar(), PasswordHash: credential.GetPasswordHash(),
		Role: user.GetRole(), Status: user.GetStatus(),
	}, nil
}

func translate(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return ErrNotFound
	case codes.AlreadyExists:
		return ErrDuplicateAccount
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return fmt.Errorf("user rpc failed: %w", err)
	}
}

const (
	MemberRole   = int32(domain.RoleMember)
	ActiveStatus = int32(domain.StatusActive)
	Disabled     = int32(domain.StatusDisabled)
)
