package userclient

import (
	"context"
	"errors"
	"testing"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRPC struct {
	create func(context.Context, *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error)
	get    func(context.Context, *userv1.GetUserByAccountRequest) (*userv1.GetUserByAccountResponse, error)
	userv1.UserServiceClient
}

func (rpc fakeRPC) CreateUser(ctx context.Context, request *userv1.CreateUserRequest, _ ...grpc.CallOption) (*userv1.CreateUserResponse, error) {
	return rpc.create(ctx, request)
}

func (rpc fakeRPC) GetUserByAccount(ctx context.Context, request *userv1.GetUserByAccountRequest, _ ...grpc.CallOption) (*userv1.GetUserByAccountResponse, error) {
	return rpc.get(ctx, request)
}

func TestClientPropagatesContextAndMapsCredential(t *testing.T) {
	type key string
	ctx := context.WithValue(context.Background(), key("trace"), "auth")
	client := New(fakeRPC{get: func(received context.Context, request *userv1.GetUserByAccountRequest) (*userv1.GetUserByAccountResponse, error) {
		if received.Value(key("trace")) != "auth" || request.GetAccount() != "gim-user" {
			t.Fatal("GetByAccount did not propagate input/context")
		}
		return &userv1.GetUserByAccountResponse{Credential: &userv1.UserCredential{
			User: &userv1.UserInfo{Id: 7, Account: "gim-user", Role: 2, Status: 1}, PasswordHash: "private-hash",
		}}, nil
	}})
	user, err := client.GetByAccount(ctx, "gim-user")
	if err != nil || user.ID != 7 || user.PasswordHash != "private-hash" {
		t.Fatalf("GetByAccount() = %#v, %v", user, err)
	}
}

func TestClientMapsStableErrors(t *testing.T) {
	client := New(fakeRPC{get: func(context.Context, *userv1.GetUserByAccountRequest) (*userv1.GetUserByAccountResponse, error) {
		return nil, status.Error(codes.NotFound, "private remote detail")
	}})
	_, err := client.GetByAccount(context.Background(), "gim-user")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByAccount() error = %v", err)
	}
}
