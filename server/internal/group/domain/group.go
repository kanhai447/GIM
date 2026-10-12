// Package domain defines Group aggregates and enums without transport or persistence dependencies.
package domain

import "time"

type GroupStatus uint8

const (
	GroupStatusActive    GroupStatus = 1
	GroupStatusDissolved GroupStatus = 2
)

func (status GroupStatus) Valid() bool {
	return status == GroupStatusActive || status == GroupStatusDissolved
}

type GroupRole uint8

const (
	GroupRoleOwner  GroupRole = 1
	GroupRoleAdmin  GroupRole = 2
	GroupRoleMember GroupRole = 3
)

func (role GroupRole) Valid() bool {
	return role == GroupRoleOwner || role == GroupRoleAdmin || role == GroupRoleMember
}

func (role GroupRole) IsManager() bool {
	return role == GroupRoleOwner || role == GroupRoleAdmin
}

type JoinPolicy uint8

const (
	JoinPolicyClosed        JoinPolicy = 0
	JoinPolicyOpen          JoinPolicy = 1
	JoinPolicyApproval      JoinPolicy = 2
	JoinPolicyQuestion      JoinPolicy = 3
	JoinPolicyCorrectAnswer JoinPolicy = 4
)

func (policy JoinPolicy) Valid() bool { return policy <= JoinPolicyCorrectAnswer }

type Group struct {
	ID                    uint64
	Name                  string
	OwnerID               uint64
	Avatar                string
	Description           string
	Notice                string
	Status                GroupStatus
	IsSearch              bool
	JoinPolicy            JoinPolicy
	VerificationQuestion  []byte
	AllowMemberInvite     bool
	AllowTemporarySession bool
	IsAllMuted            bool
	MemberLimit           uint32
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type GroupMember struct {
	ID             uint64
	GroupID        uint64
	UserID         uint64
	MemberNickname string
	Role           GroupRole
	JoinedAt       time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type JoinRequestStatus uint8

const (
	JoinRequestPending  JoinRequestStatus = 0
	JoinRequestAccepted JoinRequestStatus = 1
	JoinRequestRejected JoinRequestStatus = 2
)

func (status JoinRequestStatus) Valid() bool { return status <= JoinRequestRejected }

type JoinRequestType uint8

const (
	JoinRequestApply  JoinRequestType = 1
	JoinRequestInvite JoinRequestType = 2
)

type JoinRequest struct {
	ID                 uint64
	GroupID            uint64
	UserID             uint64
	ReviewerID         *uint64
	Status             JoinRequestStatus
	RequestType        JoinRequestType
	Message            string
	VerificationAnswer []byte
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type GroupSummary struct {
	Group       Group
	Role        GroupRole
	MemberCount uint64
}

type UserProfile struct {
	ID       uint64
	Account  string
	Nickname string
	Avatar   string
	Active   bool
}

type MemberView struct {
	Member GroupMember
	User   UserProfile
}
