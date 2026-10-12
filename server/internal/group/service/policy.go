package service

import "github.com/kanhai447/GIM/server/internal/group/domain"

func canManageJoin(role domain.GroupRole) bool { return role.IsManager() }

func canInvite(group domain.Group, role domain.GroupRole) bool {
	return role.IsManager() || role == domain.GroupRoleMember && group.AllowMemberInvite
}

func canActOn(operator, target domain.GroupRole) bool {
	if target == domain.GroupRoleOwner {
		return false
	}
	return operator == domain.GroupRoleOwner || operator == domain.GroupRoleAdmin && target == domain.GroupRoleMember
}
