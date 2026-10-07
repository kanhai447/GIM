package grpc

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeApplication struct {
	create       func(context.Context, userservice.CreateUserInput) (domain.User, error)
	getByID      func(context.Context, uint64) (domain.User, error)
	getByAccount func(context.Context, string) (domain.User, error)
}

func (app fakeApplication) CreateUser(ctx context.Context, input userservice.CreateUserInput) (domain.User, error) {
	return app.create(ctx, input)
}

func (app fakeApplication) GetUserByID(ctx context.Context, userID uint64) (domain.User, error) {
	return app.getByID(ctx, userID)
}

func (app fakeApplication) GetUserByAccount(ctx context.Context, account string) (domain.User, error) {
	return app.getByAccount(ctx, account)
}

func TestCreateUserMapsInternalContract(t *testing.T) {
	app := fakeApplication{create: func(_ context.Context, input userservice.CreateUserInput) (domain.User, error) {
		if input.PasswordHash != "private-hash-value-long-enough" || input.Role != domain.RoleMember || input.Status != domain.StatusActive {
			t.Fatalf("CreateUser input = %#v", input)
		}
		return domain.User{ID: 44}, nil
	}}
	response, err := NewServer(app).CreateUser(context.Background(), &userv1.CreateUserRequest{
		Account: "gim-user", Nickname: "GIM", PasswordHash: "private-hash-value-long-enough", Role: 2, Status: 1,
	})
	if err != nil || response.GetUserId() != 44 {
		t.Fatalf("CreateUser() = %#v, %v", response, err)
	}
}

func TestGetUserByIDReturnsPublicInfo(t *testing.T) {
	app := fakeApplication{getByID: func(context.Context, uint64) (domain.User, error) {
		return domain.User{ID: 44, Account: "gim-user", PasswordHash: "private-hash", Role: domain.RoleMember, Status: domain.StatusActive}, nil
	}}
	response, err := NewServer(app).GetUserByID(context.Background(), &userv1.GetUserByIDRequest{UserId: 44})
	if err != nil || response.GetUser().GetAccount() != "gim-user" {
		t.Fatalf("GetUserByID() = %#v, %v", response, err)
	}
}

func TestGetUserByAccountReturnsAuthCredential(t *testing.T) {
	app := fakeApplication{getByAccount: func(_ context.Context, account string) (domain.User, error) {
		return domain.User{ID: 44, Account: account, PasswordHash: "private-hash", Role: domain.RoleMember, Status: domain.StatusActive}, nil
	}}
	response, err := NewServer(app).GetUserByAccount(context.Background(), &userv1.GetUserByAccountRequest{Account: "gim-user"})
	if err != nil || response.GetCredential().GetPasswordHash() != "private-hash" {
		t.Fatalf("GetUserByAccount() = %#v, %v", response, err)
	}
}

func TestRPCErrorDoesNotLeakRepositoryDetails(t *testing.T) {
	privateDetail := "database password=private-test-value"
	app := fakeApplication{getByID: func(context.Context, uint64) (domain.User, error) {
		return domain.User{}, errors.New(privateDetail)
	}}
	_, err := NewServer(app).GetUserByID(context.Background(), &userv1.GetUserByIDRequest{UserId: 44})
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), privateDetail) {
		t.Fatalf("GetUserByID() leaked error: %v", err)
	}
}

func TestRegisteredRPCServesGeneratedClient(t *testing.T) {
	app := fakeApplication{getByID: func(_ context.Context, userID uint64) (domain.User, error) {
		return domain.User{ID: userID, Account: "gim-user", Nickname: "GIM", Role: domain.RoleMember, Status: domain.StatusActive}, nil
	}}
	listener := bufconn.Listen(1024 * 1024)
	rpcServer := googlegrpc.NewServer()
	Register(app)(rpcServer)
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- rpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		rpcServer.Stop()
		_ = listener.Close()
		<-serveDone
	})

	connection, err := googlegrpc.NewClient(
		"passthrough:///bufnet",
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
		googlegrpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	response, err := userv1.NewUserServiceClient(connection).GetUserByID(
		context.Background(),
		&userv1.GetUserByIDRequest{UserId: 44},
	)
	if err != nil || response.GetUser().GetId() != 44 {
		t.Fatalf("GetUserByID RPC = %#v, %v", response, err)
	}
}
