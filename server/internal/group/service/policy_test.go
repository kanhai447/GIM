package service

import (
	"testing"

	"github.com/kanhai447/GIM/server/internal/group/domain"
)

func TestRolePolicy(t *testing.T) {
	group := domain.Group{AllowMemberInvite: false}
	if !canManageJoin(domain.GroupRoleOwner) || !canManageJoin(domain.GroupRoleAdmin) || canManageJoin(domain.GroupRoleMember) {
		t.Fatal("join review policy mismatch")
	}
	if !canActOn(domain.GroupRoleOwner, domain.GroupRoleAdmin) || !canActOn(domain.GroupRoleAdmin, domain.GroupRoleMember) {
		t.Fatal("manager action policy rejected an allowed transition")
	}
	if canActOn(domain.GroupRoleAdmin, domain.GroupRoleAdmin) || canActOn(domain.GroupRoleAdmin, domain.GroupRoleOwner) || canActOn(domain.GroupRoleMember, domain.GroupRoleMember) {
		t.Fatal("manager action policy allowed privilege escalation")
	}
	if canInvite(group, domain.GroupRoleMember) {
		t.Fatal("member invited while invitations were disabled")
	}
	group.AllowMemberInvite = true
	if !canInvite(group, domain.GroupRoleMember) {
		t.Fatal("member invitation policy ignored group setting")
	}
}
