// Package userclient adapts the User RPC batch API for Group membership views.
package userclient

import (
	"context"
	"errors"
	"fmt"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/group/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct{ rpc userv1.UserServiceClient }

func New(rpc userv1.UserServiceClient) *Client { return &Client{rpc: rpc} }

func (client *Client) GetByID(ctx context.Context, userID uint64) (domain.UserProfile, error) {
	if client == nil || client.rpc == nil {
		return domain.UserProfile{}, errors.New("user rpc client is not initialized")
	}
	response, err := client.rpc.GetUserByID(ctx, &userv1.GetUserByIDRequest{UserId: userID})
	if err != nil {
		return domain.UserProfile{}, mapError(err)
	}
	if response.GetUser() == nil {
		return domain.UserProfile{}, errors.New("user rpc returned an incomplete user")
	}
	return profile(response.GetUser()), nil
}

func (client *Client) BatchGet(ctx context.Context, userIDs []uint64) (map[uint64]domain.UserProfile, error) {
	if client == nil || client.rpc == nil {
		return nil, errors.New("user rpc client is not initialized")
	}
	response, err := client.rpc.BatchGetUsers(ctx, &userv1.BatchGetUsersRequest{UserIds: userIDs})
	if err != nil {
		return nil, mapError(err)
	}
	result := make(map[uint64]domain.UserProfile, len(response.GetUsers()))
	for _, user := range response.GetUsers() {
		if user != nil && user.GetId() != 0 {
			result[user.GetId()] = profile(user)
		}
	}
	return result, nil
}

func profile(user *userv1.UserInfo) domain.UserProfile {
	return domain.UserProfile{ID: user.GetId(), Account: user.GetAccount(), Nickname: user.GetNickname(), Avatar: user.GetAvatar(), Active: user.GetStatus() == 1}
}

func mapError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return ErrUserNotFound
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return fmt.Errorf("user rpc lookup failed: %w", err)
	}
}

var ErrUserNotFound = errors.New("group user client: user not found")
