package mysql

import (
	"time"

	"github.com/kanhai447/GIM/server/internal/group/domain"
)

type groupRecord struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	Name                  string    `gorm:"column:name"`
	OwnerID               uint64    `gorm:"column:owner_id"`
	Avatar                string    `gorm:"column:avatar"`
	Description           string    `gorm:"column:description"`
	Notice                string    `gorm:"column:notice"`
	Status                uint8     `gorm:"column:status"`
	IsSearch              bool      `gorm:"column:is_search"`
	JoinPolicy            uint8     `gorm:"column:join_policy"`
	VerificationQuestion  []byte    `gorm:"column:verification_question"`
	AllowMemberInvite     bool      `gorm:"column:allow_member_invite"`
	AllowTemporarySession bool      `gorm:"column:allow_temporary_session"`
	IsAllMuted            bool      `gorm:"column:is_all_muted"`
	MemberLimit           uint32    `gorm:"column:member_limit"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (groupRecord) TableName() string { return "groups" }

type memberRecord struct {
	ID             uint64    `gorm:"column:id;primaryKey"`
	GroupID        uint64    `gorm:"column:group_id"`
	UserID         uint64    `gorm:"column:user_id"`
	MemberNickname string    `gorm:"column:member_nickname"`
	Role           uint8     `gorm:"column:role"`
	JoinedAt       time.Time `gorm:"column:joined_at"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (memberRecord) TableName() string { return "group_members" }

type requestRecord struct {
	ID                 uint64    `gorm:"column:id;primaryKey"`
	GroupID            uint64    `gorm:"column:group_id"`
	UserID             uint64    `gorm:"column:user_id"`
	ReviewerID         *uint64   `gorm:"column:reviewer_id"`
	Status             uint8     `gorm:"column:status"`
	RequestType        uint8     `gorm:"column:request_type"`
	Message            string    `gorm:"column:message"`
	VerificationAnswer []byte    `gorm:"column:verification_answer"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (requestRecord) TableName() string { return "group_verifies" }

func groupToRecord(value domain.Group) groupRecord {
	return groupRecord{
		ID: value.ID, Name: value.Name, OwnerID: value.OwnerID, Avatar: value.Avatar,
		Description: value.Description, Notice: value.Notice, Status: uint8(value.Status),
		IsSearch: value.IsSearch, JoinPolicy: uint8(value.JoinPolicy), VerificationQuestion: value.VerificationQuestion,
		AllowMemberInvite: value.AllowMemberInvite, AllowTemporarySession: value.AllowTemporarySession,
		IsAllMuted: value.IsAllMuted, MemberLimit: value.MemberLimit, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func (record groupRecord) toDomain() domain.Group {
	return domain.Group{
		ID: record.ID, Name: record.Name, OwnerID: record.OwnerID, Avatar: record.Avatar,
		Description: record.Description, Notice: record.Notice, Status: domain.GroupStatus(record.Status),
		IsSearch: record.IsSearch, JoinPolicy: domain.JoinPolicy(record.JoinPolicy), VerificationQuestion: record.VerificationQuestion,
		AllowMemberInvite: record.AllowMemberInvite, AllowTemporarySession: record.AllowTemporarySession,
		IsAllMuted: record.IsAllMuted, MemberLimit: record.MemberLimit, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}

func memberToRecord(value domain.GroupMember) memberRecord {
	if value.JoinedAt.IsZero() {
		value.JoinedAt = time.Now()
	}
	return memberRecord{ID: value.ID, GroupID: value.GroupID, UserID: value.UserID, MemberNickname: value.MemberNickname, Role: uint8(value.Role), JoinedAt: value.JoinedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func (record memberRecord) toDomain() domain.GroupMember {
	return domain.GroupMember{ID: record.ID, GroupID: record.GroupID, UserID: record.UserID, MemberNickname: record.MemberNickname, Role: domain.GroupRole(record.Role), JoinedAt: record.JoinedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func requestToRecord(value domain.JoinRequest) requestRecord {
	return requestRecord{ID: value.ID, GroupID: value.GroupID, UserID: value.UserID, ReviewerID: value.ReviewerID, Status: uint8(value.Status), RequestType: uint8(value.RequestType), Message: value.Message, VerificationAnswer: value.VerificationAnswer, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func (record requestRecord) toDomain() domain.JoinRequest {
	return domain.JoinRequest{ID: record.ID, GroupID: record.GroupID, UserID: record.UserID, ReviewerID: record.ReviewerID, Status: domain.JoinRequestStatus(record.Status), RequestType: domain.JoinRequestType(record.RequestType), Message: record.Message, VerificationAnswer: record.VerificationAnswer, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
