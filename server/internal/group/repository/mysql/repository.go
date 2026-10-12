// Package mysql implements Group persistence using the existing versioned schema.
package mysql

import (
	"context"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/group/domain"
	grouprepo "github.com/kanhai447/GIM/server/internal/group/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (repo *Repository) CreateGroup(ctx context.Context, group *domain.Group, owner domain.GroupMember, invited []domain.GroupMember) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record := groupToRecord(*group)
		if err := tx.Create(&record).Error; err != nil {
			return mapError("create group", err)
		}
		owner.GroupID = record.ID
		owner.Role = domain.GroupRoleOwner
		members := make([]memberRecord, 0, 1+len(invited))
		members = append(members, memberToRecord(owner))
		seen := map[uint64]struct{}{owner.UserID: {}}
		for _, member := range invited {
			if member.UserID == 0 {
				return errors.New("group repository create group: invited user id is required")
			}
			if _, exists := seen[member.UserID]; exists {
				continue
			}
			seen[member.UserID] = struct{}{}
			member.GroupID = record.ID
			member.Role = domain.GroupRoleMember
			members = append(members, memberToRecord(member))
		}
		if uint32(len(members)) > record.MemberLimit {
			return grouprepo.ErrMemberLimit
		}
		if err := tx.Create(&members).Error; err != nil {
			return mapError("create owner members", err)
		}
		*group = record.toDomain()
		return nil
	})
}

func (repo *Repository) GetGroupByID(ctx context.Context, groupID uint64) (domain.Group, error) {
	var record groupRecord
	if err := repo.db.WithContext(ctx).Take(&record, groupID).Error; err != nil {
		return domain.Group{}, mapError("get group", err)
	}
	return record.toDomain(), nil
}

func (repo *Repository) GetMember(ctx context.Context, groupID, userID uint64) (domain.GroupMember, error) {
	var record memberRecord
	if err := repo.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).Take(&record).Error; err != nil {
		return domain.GroupMember{}, mapMemberError("get member", err)
	}
	return record.toDomain(), nil
}

func (repo *Repository) IsMember(ctx context.Context, groupID, userID uint64) (bool, error) {
	var count int64
	if err := repo.db.WithContext(ctx).Model(&memberRecord{}).Where("group_id = ? AND user_id = ?", groupID, userID).Limit(1).Count(&count).Error; err != nil {
		return false, mapError("check member", err)
	}
	return count == 1, nil
}

func (repo *Repository) AddMember(ctx context.Context, member *domain.GroupMember) error {
	if err := repo.AddMembers(ctx, []domain.GroupMember{*member}); err != nil {
		return err
	}
	created, err := repo.GetMember(ctx, member.GroupID, member.UserID)
	if err != nil {
		return err
	}
	*member = created
	return nil
}

func (repo *Repository) AddMembers(ctx context.Context, members []domain.GroupMember) error {
	if len(members) == 0 {
		return nil
	}
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		groupID := members[0].GroupID
		var group groupRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&group, groupID).Error; err != nil {
			return mapError("lock group for add members", err)
		}
		if domain.GroupStatus(group.Status) != domain.GroupStatusActive {
			return grouprepo.ErrGroupNotFound
		}
		var count int64
		if err := tx.Model(&memberRecord{}).Where("group_id = ?", groupID).Count(&count).Error; err != nil {
			return mapError("count members", err)
		}
		if count+int64(len(members)) > int64(group.MemberLimit) {
			return grouprepo.ErrMemberLimit
		}
		records := make([]memberRecord, 0, len(members))
		for _, member := range members {
			if member.GroupID != groupID {
				return errors.New("group repository add members: mixed group ids")
			}
			records = append(records, memberToRecord(member))
		}
		if err := tx.Create(&records).Error; err != nil {
			return mapMemberError("add members", err)
		}
		return nil
	})
}

func (repo *Repository) RemoveMember(ctx context.Context, groupID, userID uint64) error {
	result := repo.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&memberRecord{})
	if result.Error != nil {
		return mapError("remove member", result.Error)
	}
	if result.RowsAffected != 1 {
		return grouprepo.ErrMemberNotFound
	}
	return nil
}

func (repo *Repository) ListMembers(ctx context.Context, groupID uint64, limit int) ([]domain.GroupMember, uint64, error) {
	query := repo.db.WithContext(ctx).Model(&memberRecord{}).Where("group_id = ?", groupID)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, mapError("count member list", err)
	}
	var records []memberRecord
	if err := query.Order("role ASC, joined_at ASC, id ASC").Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, mapError("list members", err)
	}
	members := make([]domain.GroupMember, 0, len(records))
	for _, record := range records {
		members = append(members, record.toDomain())
	}
	return members, uint64(count), nil
}

func (repo *Repository) CountMembers(ctx context.Context, groupID uint64) (uint64, error) {
	var count int64
	if err := repo.db.WithContext(ctx).Model(&memberRecord{}).Where("group_id = ?", groupID).Count(&count).Error; err != nil {
		return 0, mapError("count members", err)
	}
	return uint64(count), nil
}

func (repo *Repository) UpdateRole(ctx context.Context, groupID, userID uint64, role domain.GroupRole) error {
	result := repo.db.WithContext(ctx).Model(&memberRecord{}).Where("group_id = ? AND user_id = ?", groupID, userID).Update("role", uint8(role))
	if result.Error != nil {
		return mapError("update role", result.Error)
	}
	if result.RowsAffected != 1 {
		return grouprepo.ErrMemberNotFound
	}
	return nil
}

func (repo *Repository) ListGroupsForUser(ctx context.Context, userID uint64, limit int) ([]domain.GroupSummary, error) {
	type row struct {
		Group       groupRecord `gorm:"embedded"`
		CurrentRole uint8       `gorm:"column:current_role"`
		MemberCount uint64      `gorm:"column:member_count"`
	}
	counts := repo.db.Table("group_members").Select("group_id, COUNT(*) AS member_count").Group("group_id")
	var rows []row
	err := repo.db.WithContext(ctx).Table("group_members AS mine").
		Select("`groups`.*, mine.role AS current_role, counts.member_count").
		Joins("JOIN `groups` ON groups.id = mine.group_id AND groups.status = ?", uint8(domain.GroupStatusActive)).
		Joins("JOIN (?) AS counts ON counts.group_id = groups.id", counts).
		Where("mine.user_id = ?", userID).
		Order("groups.updated_at DESC, groups.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, mapError("list user groups", err)
	}
	result := make([]domain.GroupSummary, 0, len(rows))
	for _, item := range rows {
		result = append(result, domain.GroupSummary{Group: item.Group.toDomain(), Role: domain.GroupRole(item.CurrentRole), MemberCount: item.MemberCount})
	}
	return result, nil
}

func (repo *Repository) CreateJoinRequest(ctx context.Context, request *domain.JoinRequest) error {
	record := requestToRecord(*request)
	if err := repo.db.WithContext(ctx).Create(&record).Error; err != nil {
		return mapRequestError("create join request", err)
	}
	*request = record.toDomain()
	return nil
}

func (repo *Repository) JoinOpenGroup(ctx context.Context, request *domain.JoinRequest, member *domain.GroupMember) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var group groupRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&group, request.GroupID).Error; err != nil {
			return mapError("lock open group", err)
		}
		if domain.GroupStatus(group.Status) != domain.GroupStatusActive {
			return grouprepo.ErrGroupNotFound
		}
		var count int64
		if err := tx.Model(&memberRecord{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
			return mapError("count open group members", err)
		}
		if count >= int64(group.MemberLimit) {
			return grouprepo.ErrMemberLimit
		}
		member.GroupID, member.Role = group.ID, domain.GroupRoleMember
		memberRow := memberToRecord(*member)
		if err := tx.Create(&memberRow).Error; err != nil {
			return mapMemberError("join open group", err)
		}
		request.Status = domain.JoinRequestAccepted
		requestRow := requestToRecord(*request)
		if err := tx.Create(&requestRow).Error; err != nil {
			return mapRequestError("record open group join", err)
		}
		*member, *request = memberRow.toDomain(), requestRow.toDomain()
		return nil
	})
}

func (repo *Repository) GetJoinRequest(ctx context.Context, requestID uint64) (domain.JoinRequest, error) {
	var record requestRecord
	if err := repo.db.WithContext(ctx).Take(&record, requestID).Error; err != nil {
		return domain.JoinRequest{}, mapRequestError("get join request", err)
	}
	return record.toDomain(), nil
}

func (repo *Repository) ApproveJoinRequest(ctx context.Context, requestID, reviewerID uint64) (domain.JoinRequest, error) {
	var result domain.JoinRequest
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request requestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&request, requestID).Error; err != nil {
			return mapRequestError("lock join request", err)
		}
		if domain.JoinRequestStatus(request.Status) != domain.JoinRequestPending {
			return grouprepo.ErrInvalidRequestState
		}
		var group groupRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&group, request.GroupID).Error; err != nil {
			return mapError("lock group for approval", err)
		}
		if domain.GroupStatus(group.Status) != domain.GroupStatusActive {
			return grouprepo.ErrGroupNotFound
		}
		var count int64
		if err := tx.Model(&memberRecord{}).Where("group_id = ?", request.GroupID).Count(&count).Error; err != nil {
			return mapError("count approval members", err)
		}
		if count >= int64(group.MemberLimit) {
			return grouprepo.ErrMemberLimit
		}
		member := memberToRecord(domain.GroupMember{GroupID: request.GroupID, UserID: request.UserID, Role: domain.GroupRoleMember})
		if err := tx.Create(&member).Error; err != nil {
			return mapMemberError("insert approved member", err)
		}
		request.Status, request.ReviewerID = uint8(domain.JoinRequestAccepted), &reviewerID
		if err := tx.Model(&request).Updates(map[string]any{"status": request.Status, "reviewer_id": reviewerID}).Error; err != nil {
			return mapError("accept join request", err)
		}
		result = request.toDomain()
		return nil
	})
	return result, err
}

func (repo *Repository) RejectJoinRequest(ctx context.Context, requestID, reviewerID uint64) (domain.JoinRequest, error) {
	var result domain.JoinRequest
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request requestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&request, requestID).Error; err != nil {
			return mapRequestError("lock join request", err)
		}
		if domain.JoinRequestStatus(request.Status) != domain.JoinRequestPending {
			return grouprepo.ErrInvalidRequestState
		}
		request.Status, request.ReviewerID = uint8(domain.JoinRequestRejected), &reviewerID
		if err := tx.Model(&request).Updates(map[string]any{"status": request.Status, "reviewer_id": reviewerID}).Error; err != nil {
			return mapError("reject join request", err)
		}
		result = request.toDomain()
		return nil
	})
	return result, err
}

func (repo *Repository) DissolveGroup(ctx context.Context, groupID uint64) error {
	return repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&groupRecord{}).Where("id = ? AND status = ?", groupID, uint8(domain.GroupStatusActive)).Update("status", uint8(domain.GroupStatusDissolved))
		if result.Error != nil {
			return mapError("dissolve group", result.Error)
		}
		if result.RowsAffected != 1 {
			return grouprepo.ErrGroupNotFound
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&memberRecord{}).Error; err != nil {
			return mapError("remove dissolved memberships", err)
		}
		return nil
	})
}

func mapError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return grouprepo.ErrGroupNotFound
	}
	return fmt.Errorf("group repository %s: %w", operation, err)
}

func mapMemberError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return grouprepo.ErrMemberNotFound
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return grouprepo.ErrAlreadyMember
	}
	return fmt.Errorf("group repository %s: %w", operation, err)
}

func mapRequestError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return grouprepo.ErrRequestNotFound
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return grouprepo.ErrDuplicateRequest
	}
	return fmt.Errorf("group repository %s: %w", operation, err)
}
