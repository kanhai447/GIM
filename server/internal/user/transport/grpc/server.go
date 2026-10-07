// Package grpc adapts the user application service to the internal gRPC contract.
package grpc

import (
	"context"
	"errors"
	"net/http"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Application interface {
	CreateUser(ctx context.Context, input userservice.CreateUserInput) (domain.User, error)
	GetUserByID(ctx context.Context, userID uint64) (domain.User, error)
	GetUserByAccount(ctx context.Context, account string) (domain.User, error)
}

type Server struct {
	userv1.UnimplementedUserServiceServer
	users Application
}

func NewServer(users Application) *Server {
	return &Server{users: users}
}

// Register returns a function directly usable as go-zero zrpc.NewServer's registrar.
func Register(users Application) func(*googlegrpc.Server) {
	return func(server *googlegrpc.Server) {
		userv1.RegisterUserServiceServer(server, NewServer(users))
	}
}

func (server *Server) CreateUser(ctx context.Context, request *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	user, err := server.users.CreateUser(ctx, userservice.CreateUserInput{
		Account:      request.GetAccount(),
		Nickname:     request.GetNickname(),
		PasswordHash: request.GetPasswordHash(),
		Avatar:       request.GetAvatar(),
		Role:         domain.Role(request.GetRole()),
		Status:       domain.Status(request.GetStatus()),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &userv1.CreateUserResponse{UserId: user.ID}, nil
}

func (server *Server) GetUserByID(ctx context.Context, request *userv1.GetUserByIDRequest) (*userv1.GetUserByIDResponse, error) {
	user, err := server.users.GetUserByID(ctx, request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &userv1.GetUserByIDResponse{User: publicUser(user)}, nil
}

func (server *Server) GetUserByAccount(ctx context.Context, request *userv1.GetUserByAccountRequest) (*userv1.GetUserByAccountResponse, error) {
	user, err := server.users.GetUserByAccount(ctx, request.GetAccount())
	if err != nil {
		return nil, grpcError(err)
	}
	return &userv1.GetUserByAccountResponse{
		Credential: &userv1.UserCredential{
			User:         publicUser(user),
			PasswordHash: user.PasswordHash,
		},
	}, nil
}

func publicUser(user domain.User) *userv1.UserInfo {
	return &userv1.UserInfo{
		Id:       user.ID,
		Account:  user.Account,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Role:     int32(user.Role),
		Status:   int32(user.Status),
	}
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "请求已取消")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "请求超时")
	}
	httpStatus, code, message := apperror.PublicFields(err)
	switch {
	case code == userservice.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, message)
	case code == userservice.CodeUserNotFound:
		return status.Error(codes.NotFound, message)
	case code == userservice.CodeDuplicateAccount:
		return status.Error(codes.AlreadyExists, message)
	case httpStatus == http.StatusInternalServerError:
		return status.Error(codes.Internal, message)
	default:
		return status.Error(codes.Unknown, message)
	}
}
