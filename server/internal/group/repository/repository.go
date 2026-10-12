// Package repository defines persistence boundaries for the Group domain.
package repository

import (
	"context"
	"errors"

	"github.com/kanhai447/GIM/server/internal/group/domain"
)

var (
	ErrGroupNotFound       = errors.New("group repository: group not found")
	ErrMemberNotFound      = errors.New("group repository: member not found")
	ErrAlreadyMember       = errors.New("group repository: already member")
	ErrRequestNotFound     = errors.New("group repository: request not found")
	ErrDuplicateRequest    = errors.New("group repository: duplicate pending request")
	ErrInvalidRequestState = errors.New("group repository: invalid request state")
	ErrMemberLimit         = errors.New("group repository: member limit reached")
)

type Repository interface {
	CreateGroup(ctx context.Context, group *domain.Group, owner domain.GroupMember, invited []domain.GroupMember) error
	GetGroupByID(ctx context.Context, groupID uint64) (domain.Group, error)
	GetMember(ctx context.Context, groupID, userID uint64) (domain.GroupMember, error)
	IsMember(ctx context.Context, groupID, userID uint64) (bool, error)
	AddMember(ctx context.Context, member *domain.GroupMember) error
	AddMembers(ctx context.Context, members []domain.GroupMember) error
	RemoveMember(ctx context.Context, groupID, userID uint64) error
	ListMembers(ctx context.Context, groupID uint64, limit int) ([]domain.GroupMember, uint64, error)
	CountMembers(ctx context.Context, groupID uint64) (uint64, error)
	UpdateRole(ctx context.Context, groupID, userID uint64, role domain.GroupRole) error
	ListGroupsForUser(ctx context.Context, userID uint64, limit int) ([]domain.GroupSummary, error)
	CreateJoinRequest(ctx context.Context, request *domain.JoinRequest) error
	JoinOpenGroup(ctx context.Context, request *domain.JoinRequest, member *domain.GroupMember) error
	GetJoinRequest(ctx context.Context, requestID uint64) (domain.JoinRequest, error)
	ApproveJoinRequest(ctx context.Context, requestID, reviewerID uint64) (domain.JoinRequest, error)
	RejectJoinRequest(ctx context.Context, requestID, reviewerID uint64) (domain.JoinRequest, error)
	DissolveGroup(ctx context.Context, groupID uint64) error
}
