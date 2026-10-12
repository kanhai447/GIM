// Package grpc adapts Group membership services to the internal RPC contract.
package grpc

import (
	"context"
	"errors"
	"net/http"

	groupv1 "github.com/kanhai447/GIM/server/api/group/v1"
	"github.com/kanhai447/GIM/server/internal/group/domain"
	groupservice "github.com/kanhai447/GIM/server/internal/group/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Application interface {
	GetGroupByID(context.Context, uint64) (domain.Group, error)
	IsMember(context.Context, uint64, uint64) (bool, error)
	GetMemberRole(context.Context, uint64, uint64) (domain.GroupRole, error)
	IsMuted(context.Context, uint64, uint64) (bool, error)
}

type Server struct {
	groupv1.UnimplementedGroupServiceServer
	groups Application
}

func NewServer(groups Application) *Server { return &Server{groups: groups} }

func Register(groups Application) func(*googlegrpc.Server) {
	return func(server *googlegrpc.Server) { groupv1.RegisterGroupServiceServer(server, NewServer(groups)) }
}

func (server *Server) GetGroup(ctx context.Context, request *groupv1.GetGroupRequest) (*groupv1.GetGroupResponse, error) {
	group, err := server.groups.GetGroupByID(ctx, request.GetGroupId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &groupv1.GetGroupResponse{GroupId: group.ID, OwnerId: group.OwnerID, Status: int32(group.Status), IsAllMuted: group.IsAllMuted}, nil
}

func (server *Server) IsMember(ctx context.Context, request *groupv1.IsMemberRequest) (*groupv1.IsMemberResponse, error) {
	member, err := server.groups.IsMember(ctx, request.GetGroupId(), request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &groupv1.IsMemberResponse{IsMember: member}, nil
}

func (server *Server) GetMemberRole(ctx context.Context, request *groupv1.GetMemberRoleRequest) (*groupv1.GetMemberRoleResponse, error) {
	role, err := server.groups.GetMemberRole(ctx, request.GetGroupId(), request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &groupv1.GetMemberRoleResponse{Role: int32(role)}, nil
}

func (server *Server) IsMuted(ctx context.Context, request *groupv1.IsMutedRequest) (*groupv1.IsMutedResponse, error) {
	muted, err := server.groups.IsMuted(ctx, request.GetGroupId(), request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &groupv1.IsMutedResponse{IsMuted: muted}, nil
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "请求已取消")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "请求超时")
	}
	httpStatus, code, message := apperror.PublicFields(err)
	switch code {
	case groupservice.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, message)
	case groupservice.CodeGroupNotFound, groupservice.CodeNotMember, groupservice.CodeRequestNotFound:
		return status.Error(codes.NotFound, message)
	case groupservice.CodePermissionDenied:
		return status.Error(codes.PermissionDenied, message)
	case groupservice.CodeAlreadyMember, groupservice.CodeDuplicateRequest, groupservice.CodeInvalidRequestState:
		return status.Error(codes.FailedPrecondition, message)
	case groupservice.CodeMuteStateError:
		return status.Error(codes.Unavailable, message)
	case apperror.CodeInternal:
		return status.Error(codes.Internal, message)
	default:
		if httpStatus == http.StatusInternalServerError {
			return status.Error(codes.Internal, message)
		}
		return status.Error(codes.Unknown, message)
	}
}
