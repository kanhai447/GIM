// Package service contains transport-independent Group business policy.
package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/group/domain"
	"github.com/kanhai447/GIM/server/internal/group/mute"
	"github.com/kanhai447/GIM/server/internal/group/repository"
	"github.com/kanhai447/GIM/server/internal/group/userclient"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

const (
	CodeInvalidArgument     uint32 = 2301
	CodeGroupNotFound       uint32 = 2302
	CodeAlreadyMember       uint32 = 2303
	CodeNotMember           uint32 = 2304
	CodePermissionDenied    uint32 = 2305
	CodeDuplicateRequest    uint32 = 2306
	CodeRequestNotFound     uint32 = 2307
	CodeInvalidRequestState uint32 = 2308
	CodeTargetUserNotFound  uint32 = 2309
	CodeOwnerCannotLeave    uint32 = 2310
	CodeMuteStateError      uint32 = 2311
	CodeJoinClosed          uint32 = 2312
	CodeMemberLimit         uint32 = 2313
)

const (
	defaultMemberLimit = 100
	defaultListLimit   = 100
	maxListLimit       = 500
	maxMuteDuration    = 30 * 24 * time.Hour
)

type Users interface {
	GetByID(ctx context.Context, userID uint64) (domain.UserProfile, error)
	BatchGet(ctx context.Context, userIDs []uint64) (map[uint64]domain.UserProfile, error)
}

type Config struct{ DependencyTimeout time.Duration }

func DefaultConfig() Config { return Config{DependencyTimeout: 2 * time.Second} }

type Service struct {
	repository repository.Repository
	users      Users
	mutes      mute.Store
	config     Config
}

type CreateGroupInput struct {
	Name           string
	Avatar         string
	Description    string
	IsSearch       bool
	JoinPolicy     domain.JoinPolicy
	MemberLimit    uint32
	InvitedUserIDs []uint64
}

type GroupInfo struct {
	Group       domain.Group
	CurrentRole domain.GroupRole
	MemberCount uint64
}

func New(repository repository.Repository, users Users, mutes mute.Store, config Config) (*Service, error) {
	if repository == nil || users == nil || mutes == nil || config.DependencyTimeout <= 0 {
		return nil, errors.New("group service dependencies are invalid")
	}
	return &Service{repository: repository, users: users, mutes: mutes, config: config}, nil
}

func (service *Service) CreateGroup(ctx context.Context, operatorID uint64, input CreateGroupInput) (domain.Group, error) {
	if err := ctx.Err(); err != nil {
		return domain.Group{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if operatorID == 0 || utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 64 || !input.JoinPolicy.Valid() {
		return domain.Group{}, business(CodeInvalidArgument, "群参数无效")
	}
	if input.MemberLimit == 0 {
		input.MemberLimit = defaultMemberLimit
	}
	if input.MemberLimit > 10_000 {
		return domain.Group{}, business(CodeInvalidArgument, "群人数上限无效")
	}
	uniqueInvites := uniqueUserIDs(input.InvitedUserIDs, operatorID)
	if uint32(1+len(uniqueInvites)) > input.MemberLimit {
		return domain.Group{}, business(CodeMemberLimit, "群成员已达上限")
	}
	profiles, err := service.lookupUsers(ctx, append([]uint64{operatorID}, uniqueInvites...))
	if err != nil {
		return domain.Group{}, err
	}
	if len(profiles) != 1+len(uniqueInvites) {
		return domain.Group{}, business(CodeTargetUserNotFound, "目标用户不存在")
	}
	for _, profile := range profiles {
		if !profile.Active {
			return domain.Group{}, business(CodeTargetUserNotFound, "目标用户不存在")
		}
	}
	group := domain.Group{
		Name: input.Name, OwnerID: operatorID, Avatar: strings.TrimSpace(input.Avatar), Description: strings.TrimSpace(input.Description),
		Status: domain.GroupStatusActive, IsSearch: input.IsSearch, JoinPolicy: input.JoinPolicy,
		AllowMemberInvite: true, AllowTemporarySession: true, MemberLimit: input.MemberLimit,
	}
	invited := make([]domain.GroupMember, 0, len(uniqueInvites))
	for _, userID := range uniqueInvites {
		invited = append(invited, domain.GroupMember{UserID: userID, Role: domain.GroupRoleMember})
	}
	if err := service.repository.CreateGroup(ctx, &group, domain.GroupMember{UserID: operatorID, Role: domain.GroupRoleOwner}, invited); err != nil {
		return domain.Group{}, serviceError(err)
	}
	return group, nil
}

func (service *Service) GetGroupByID(ctx context.Context, groupID uint64) (domain.Group, error) {
	group, err := service.repository.GetGroupByID(ctx, groupID)
	if err != nil {
		return domain.Group{}, serviceError(err)
	}
	if group.Status != domain.GroupStatusActive {
		return domain.Group{}, business(CodeGroupNotFound, "群不存在")
	}
	return group, nil
}

func (service *Service) GroupInfo(ctx context.Context, operatorID, groupID uint64) (GroupInfo, error) {
	group, err := service.GetGroupByID(ctx, groupID)
	if err != nil {
		return GroupInfo{}, err
	}
	member, err := service.repository.GetMember(ctx, groupID, operatorID)
	if err != nil {
		return GroupInfo{}, serviceError(err)
	}
	count, err := service.repository.CountMembers(ctx, groupID)
	if err != nil {
		return GroupInfo{}, serviceError(err)
	}
	return GroupInfo{Group: group, CurrentRole: member.Role, MemberCount: count}, nil
}

func (service *Service) MyGroups(ctx context.Context, operatorID uint64, limit int) ([]domain.GroupSummary, error) {
	if operatorID == 0 {
		return nil, business(CodeInvalidArgument, "身份信息无效")
	}
	limit, err := normalizeLimit(limit)
	if err != nil {
		return nil, err
	}
	groups, err := service.repository.ListGroupsForUser(ctx, operatorID, limit)
	if err != nil {
		return nil, serviceError(err)
	}
	return groups, nil
}

func (service *Service) ListMembers(ctx context.Context, operatorID, groupID uint64, limit int) ([]domain.MemberView, uint64, error) {
	if _, err := service.requireMember(ctx, groupID, operatorID); err != nil {
		return nil, 0, err
	}
	limit, err := normalizeLimit(limit)
	if err != nil {
		return nil, 0, err
	}
	members, count, err := service.repository.ListMembers(ctx, groupID, limit)
	if err != nil {
		return nil, 0, serviceError(err)
	}
	ids := make([]uint64, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.UserID)
	}
	profiles, err := service.lookupUsers(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	views := make([]domain.MemberView, 0, len(members))
	for _, member := range members {
		profile, exists := profiles[member.UserID]
		if !exists {
			return nil, 0, internal(errors.New("user batch response omitted a group member"))
		}
		views = append(views, domain.MemberView{Member: member, User: profile})
	}
	return views, count, nil
}

func (service *Service) ApplyToJoin(ctx context.Context, operatorID, groupID uint64, message string) (domain.JoinRequest, error) {
	group, err := service.GetGroupByID(ctx, groupID)
	if err != nil {
		return domain.JoinRequest{}, err
	}
	if len(message) > 128 {
		return domain.JoinRequest{}, business(CodeInvalidArgument, "加群申请参数无效")
	}
	if member, lookupErr := service.repository.IsMember(ctx, groupID, operatorID); lookupErr != nil {
		return domain.JoinRequest{}, serviceError(lookupErr)
	} else if member {
		return domain.JoinRequest{}, business(CodeAlreadyMember, "已经是群成员")
	}
	profile, err := service.lookupUser(ctx, operatorID)
	if err != nil || !profile.Active {
		if err != nil {
			return domain.JoinRequest{}, err
		}
		return domain.JoinRequest{}, business(CodeTargetUserNotFound, "用户不存在")
	}
	request := domain.JoinRequest{GroupID: groupID, UserID: operatorID, Status: domain.JoinRequestPending, RequestType: domain.JoinRequestApply, Message: strings.TrimSpace(message)}
	switch group.JoinPolicy {
	case domain.JoinPolicyClosed:
		return domain.JoinRequest{}, business(CodeJoinClosed, "该群不允许申请加入")
	case domain.JoinPolicyOpen:
		member := domain.GroupMember{GroupID: groupID, UserID: operatorID, Role: domain.GroupRoleMember}
		if err := service.repository.JoinOpenGroup(ctx, &request, &member); err != nil {
			return domain.JoinRequest{}, serviceError(err)
		}
	default:
		if err := service.repository.CreateJoinRequest(ctx, &request); err != nil {
			return domain.JoinRequest{}, serviceError(err)
		}
	}
	return request, nil
}

func (service *Service) ReviewJoinRequest(ctx context.Context, operatorID, requestID uint64, accept bool) (domain.JoinRequest, error) {
	request, err := service.repository.GetJoinRequest(ctx, requestID)
	if err != nil {
		return domain.JoinRequest{}, serviceError(err)
	}
	operator, err := service.requireMember(ctx, request.GroupID, operatorID)
	if err != nil {
		return domain.JoinRequest{}, err
	}
	if !canManageJoin(operator.Role) {
		return domain.JoinRequest{}, business(CodePermissionDenied, "无权处理加群申请")
	}
	if accept {
		request, err = service.repository.ApproveJoinRequest(ctx, requestID, operatorID)
	} else {
		request, err = service.repository.RejectJoinRequest(ctx, requestID, operatorID)
	}
	if err != nil {
		return domain.JoinRequest{}, serviceError(err)
	}
	return request, nil
}

func (service *Service) InviteMembers(ctx context.Context, operatorID, groupID uint64, targetIDs []uint64) error {
	group, err := service.GetGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	operator, err := service.requireMember(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if !canInvite(group, operator.Role) {
		return business(CodePermissionDenied, "无权邀请群成员")
	}
	targetIDs = uniqueUserIDs(targetIDs, operatorID)
	if len(targetIDs) == 0 {
		return business(CodeInvalidArgument, "邀请成员参数无效")
	}
	profiles, err := service.lookupUsers(ctx, targetIDs)
	if err != nil {
		return err
	}
	if len(profiles) != len(targetIDs) {
		return business(CodeTargetUserNotFound, "目标用户不存在")
	}
	members := make([]domain.GroupMember, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		profile := profiles[targetID]
		if !profile.Active {
			return business(CodeTargetUserNotFound, "目标用户不存在")
		}
		if exists, lookupErr := service.repository.IsMember(ctx, groupID, targetID); lookupErr != nil {
			return serviceError(lookupErr)
		} else if exists {
			return business(CodeAlreadyMember, "目标用户已经是群成员")
		}
		members = append(members, domain.GroupMember{GroupID: groupID, UserID: targetID, Role: domain.GroupRoleMember})
	}
	if err := service.repository.AddMembers(ctx, members); err != nil {
		return serviceError(err)
	}
	return nil
}

func (service *Service) LeaveGroup(ctx context.Context, operatorID, groupID uint64) error {
	member, err := service.requireMember(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if member.Role == domain.GroupRoleOwner {
		return business(CodeOwnerCannotLeave, "群主不能直接退出群聊")
	}
	if err := service.repository.RemoveMember(ctx, groupID, operatorID); err != nil {
		return serviceError(err)
	}
	return nil
}

func (service *Service) KickMember(ctx context.Context, operatorID, groupID, targetID uint64) error {
	if operatorID == targetID {
		return service.LeaveGroup(ctx, operatorID, groupID)
	}
	operator, err := service.requireMember(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	target, err := service.requireMember(ctx, groupID, targetID)
	if err != nil {
		return err
	}
	if !canActOn(operator.Role, target.Role) {
		return business(CodePermissionDenied, "无权移除该群成员")
	}
	if err := service.repository.RemoveMember(ctx, groupID, targetID); err != nil {
		return serviceError(err)
	}
	return nil
}

func (service *Service) UpdateMemberRole(ctx context.Context, operatorID, groupID, targetID uint64, role domain.GroupRole) error {
	if role != domain.GroupRoleAdmin && role != domain.GroupRoleMember {
		return business(CodeInvalidArgument, "群角色参数无效")
	}
	operator, err := service.requireMember(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if operator.Role != domain.GroupRoleOwner {
		return business(CodePermissionDenied, "只有群主可以调整管理员")
	}
	target, err := service.requireMember(ctx, groupID, targetID)
	if err != nil {
		return err
	}
	if target.Role == domain.GroupRoleOwner {
		return business(CodePermissionDenied, "不能修改群主角色")
	}
	if err := service.repository.UpdateRole(ctx, groupID, targetID, role); err != nil {
		return serviceError(err)
	}
	return nil
}

func (service *Service) MuteMember(ctx context.Context, operatorID, groupID, targetID uint64, duration time.Duration) error {
	if duration <= 0 || duration > maxMuteDuration {
		return business(CodeInvalidArgument, "禁言时长无效")
	}
	operator, target, err := service.membersForModeration(ctx, operatorID, groupID, targetID)
	if err != nil {
		return err
	}
	if !canActOn(operator.Role, target.Role) {
		return business(CodePermissionDenied, "无权禁言该群成员")
	}
	if err := service.mutes.Mute(ctx, target.ID, duration); err != nil {
		return muteError(err)
	}
	return nil
}

func (service *Service) UnmuteMember(ctx context.Context, operatorID, groupID, targetID uint64) error {
	operator, target, err := service.membersForModeration(ctx, operatorID, groupID, targetID)
	if err != nil {
		return err
	}
	if !canActOn(operator.Role, target.Role) {
		return business(CodePermissionDenied, "无权解除该群成员禁言")
	}
	if err := service.mutes.Unmute(ctx, target.ID); err != nil {
		return muteError(err)
	}
	return nil
}

func (service *Service) IsMuted(ctx context.Context, groupID, userID uint64) (bool, error) {
	member, err := service.requireMember(ctx, groupID, userID)
	if err != nil {
		return false, err
	}
	muted, err := service.mutes.IsMuted(ctx, member.ID)
	if err != nil {
		return false, muteError(err)
	}
	return muted, nil
}

func (service *Service) IsMember(ctx context.Context, groupID, userID uint64) (bool, error) {
	group, err := service.GetGroupByID(ctx, groupID)
	if err != nil {
		return false, err
	}
	_ = group
	member, err := service.repository.IsMember(ctx, groupID, userID)
	if err != nil {
		return false, serviceError(err)
	}
	return member, nil
}

func (service *Service) GetMemberRole(ctx context.Context, groupID, userID uint64) (domain.GroupRole, error) {
	member, err := service.requireMember(ctx, groupID, userID)
	if err != nil {
		return 0, err
	}
	return member.Role, nil
}

func (service *Service) DissolveGroup(ctx context.Context, operatorID, groupID uint64) error {
	member, err := service.requireMember(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if member.Role != domain.GroupRoleOwner {
		return business(CodePermissionDenied, "只有群主可以解散群聊")
	}
	if err := service.repository.DissolveGroup(ctx, groupID); err != nil {
		return serviceError(err)
	}
	return nil
}

func (service *Service) membersForModeration(ctx context.Context, operatorID, groupID, targetID uint64) (domain.GroupMember, domain.GroupMember, error) {
	operator, err := service.requireMember(ctx, groupID, operatorID)
	if err != nil {
		return domain.GroupMember{}, domain.GroupMember{}, err
	}
	target, err := service.requireMember(ctx, groupID, targetID)
	if err != nil {
		return domain.GroupMember{}, domain.GroupMember{}, err
	}
	return operator, target, nil
}

func (service *Service) requireMember(ctx context.Context, groupID, userID uint64) (domain.GroupMember, error) {
	if groupID == 0 || userID == 0 {
		return domain.GroupMember{}, business(CodeInvalidArgument, "参数错误")
	}
	if _, err := service.GetGroupByID(ctx, groupID); err != nil {
		return domain.GroupMember{}, err
	}
	member, err := service.repository.GetMember(ctx, groupID, userID)
	if err != nil {
		return domain.GroupMember{}, serviceError(err)
	}
	return member, nil
}

func (service *Service) lookupUser(ctx context.Context, userID uint64) (domain.UserProfile, error) {
	lookupContext, cancel := context.WithTimeout(ctx, service.config.DependencyTimeout)
	defer cancel()
	profile, err := service.users.GetByID(lookupContext, userID)
	if err != nil {
		if errors.Is(err, userclient.ErrUserNotFound) {
			return domain.UserProfile{}, business(CodeTargetUserNotFound, "用户不存在")
		}
		return domain.UserProfile{}, internal(err)
	}
	return profile, nil
}

func (service *Service) lookupUsers(ctx context.Context, userIDs []uint64) (map[uint64]domain.UserProfile, error) {
	if len(userIDs) == 0 {
		return map[uint64]domain.UserProfile{}, nil
	}
	lookupContext, cancel := context.WithTimeout(ctx, service.config.DependencyTimeout)
	defer cancel()
	profiles, err := service.users.BatchGet(lookupContext, userIDs)
	if err != nil {
		return nil, internal(err)
	}
	return profiles, nil
}

func normalizeLimit(limit int) (int, error) {
	if limit == 0 || limit == -1 {
		return defaultListLimit, nil
	}
	if limit < 1 || limit > maxListLimit {
		return 0, business(CodeInvalidArgument, "列表参数无效")
	}
	return limit, nil
}

func uniqueUserIDs(values []uint64, excluded uint64) []uint64 {
	result := make([]uint64, 0, len(values))
	seen := map[uint64]struct{}{excluded: {}}
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func serviceError(err error) error {
	switch {
	case errors.Is(err, repository.ErrGroupNotFound):
		return business(CodeGroupNotFound, "群不存在")
	case errors.Is(err, repository.ErrMemberNotFound):
		return business(CodeNotMember, "不是群成员")
	case errors.Is(err, repository.ErrAlreadyMember):
		return business(CodeAlreadyMember, "已经是群成员")
	case errors.Is(err, repository.ErrRequestNotFound):
		return business(CodeRequestNotFound, "加群申请不存在")
	case errors.Is(err, repository.ErrDuplicateRequest):
		return business(CodeDuplicateRequest, "已有待处理的加群申请")
	case errors.Is(err, repository.ErrInvalidRequestState):
		return business(CodeInvalidRequestState, "加群申请状态无效")
	case errors.Is(err, repository.ErrMemberLimit):
		return business(CodeMemberLimit, "群成员已达上限")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return internal(err)
	}
}

func business(code uint32, message string) error { return apperror.Business(code, message) }

func internal(err error) error {
	return apperror.Wrap(err, http.StatusInternalServerError, apperror.CodeInternal, "服务器错误")
}

func muteError(err error) error {
	return apperror.Wrap(err, http.StatusServiceUnavailable, CodeMuteStateError, "群禁言状态暂不可用")
}
