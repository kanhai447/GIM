package userclient

import (
	"context"
	"errors"
	"testing"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRPC struct {
	userv1.UserServiceClient
	response *userv1.GetUserByIDResponse
	err      error
}

func (rpc fakeRPC) GetUserByID(context.Context, *userv1.GetUserByIDRequest, ...grpc.CallOption) (*userv1.GetUserByIDResponse, error) {
	return rpc.response, rpc.err
}

func TestGetByIDTranslatesUserRPC(t *testing.T) {
	client := New(fakeRPC{response: &userv1.GetUserByIDResponse{User: &userv1.UserInfo{Id: 42, Status: 1}}})
	recipient, err := client.GetByID(context.Background(), 42)
	if err != nil || recipient.ID != 42 || !recipient.Active {
		t.Fatalf("recipient=%#v err=%v", recipient, err)
	}

	client = New(fakeRPC{err: status.Error(codes.NotFound, "private internal detail")})
	if _, err := client.GetByID(context.Background(), 404); !errors.Is(err, message.ErrRecipientNotFound) {
		t.Fatalf("not found error = %v", err)
	}

	client = New(fakeRPC{err: status.Error(codes.DeadlineExceeded, "timeout")})
	if _, err := client.GetByID(context.Background(), 42); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
}
