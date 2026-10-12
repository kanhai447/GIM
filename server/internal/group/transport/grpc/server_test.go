package grpc

import (
	"context"
	"testing"

	groupv1 "github.com/kanhai447/GIM/server/api/group/v1"
	"github.com/kanhai447/GIM/server/internal/group/domain"
)

type fakeApplication struct{}

func (fakeApplication) GetGroupByID(context.Context, uint64) (domain.Group, error) {
	return domain.Group{ID: 9, OwnerID: 1, Status: domain.GroupStatusActive, IsAllMuted: true}, nil
}

func (fakeApplication) IsMember(context.Context, uint64, uint64) (bool, error) { return true, nil }

func (fakeApplication) GetMemberRole(context.Context, uint64, uint64) (domain.GroupRole, error) {
	return domain.GroupRoleAdmin, nil
}

func (fakeApplication) IsMuted(context.Context, uint64, uint64) (bool, error) { return true, nil }

func TestMembershipRPCFoundation(t *testing.T) {
	server := NewServer(fakeApplication{})
	group, err := server.GetGroup(context.Background(), &groupv1.GetGroupRequest{GroupId: 9})
	if err != nil || group.GetOwnerId() != 1 || !group.GetIsAllMuted() {
		t.Fatalf("GetGroup() = %#v, %v", group, err)
	}
	member, err := server.IsMember(context.Background(), &groupv1.IsMemberRequest{GroupId: 9, UserId: 2})
	if err != nil || !member.GetIsMember() {
		t.Fatalf("IsMember() = %#v, %v", member, err)
	}
	role, err := server.GetMemberRole(context.Background(), &groupv1.GetMemberRoleRequest{GroupId: 9, UserId: 2})
	if err != nil || role.GetRole() != int32(domain.GroupRoleAdmin) {
		t.Fatalf("GetMemberRole() = %#v, %v", role, err)
	}
	muted, err := server.IsMuted(context.Background(), &groupv1.IsMutedRequest{GroupId: 9, UserId: 2})
	if err != nil || !muted.GetIsMuted() {
		t.Fatalf("IsMuted() = %#v, %v", muted, err)
	}
}
