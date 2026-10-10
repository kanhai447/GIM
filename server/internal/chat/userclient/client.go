package userclient

import (
	"context"
	"errors"
	"fmt"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const activeStatus = int32(1)

type Client struct{ rpc userv1.UserServiceClient }

func New(rpc userv1.UserServiceClient) *Client { return &Client{rpc: rpc} }

func (client *Client) GetByID(ctx context.Context, userID uint64) (message.Recipient, error) {
	if client == nil || client.rpc == nil {
		return message.Recipient{}, errors.New("user rpc client is not initialized")
	}
	response, err := client.rpc.GetUserByID(ctx, &userv1.GetUserByIDRequest{UserId: userID})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return message.Recipient{}, message.ErrRecipientNotFound
		case codes.Canceled:
			return message.Recipient{}, context.Canceled
		case codes.DeadlineExceeded:
			return message.Recipient{}, context.DeadlineExceeded
		default:
			return message.Recipient{}, fmt.Errorf("user rpc lookup failed: %w", err)
		}
	}
	user := response.GetUser()
	if user == nil || user.GetId() == 0 {
		return message.Recipient{}, errors.New("user rpc returned an incomplete user")
	}
	return message.Recipient{ID: user.GetId(), Active: user.GetStatus() == activeStatus}, nil
}
