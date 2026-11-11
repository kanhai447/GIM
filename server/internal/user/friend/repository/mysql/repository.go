// Package mysql implements friendship persistence for the User service.
package mysql

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/kanhai447/GIM/server/internal/user/friend"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type profileRecord struct {
	Nickname string `gorm:"column:nickname"`
	Avatar   string `gorm:"column:avatar"`
}

type friendRow struct {
	FriendID  uint64    `gorm:"column:friend_id"`
	Remark    string    `gorm:"column:remark"`
	Account   string    `gorm:"column:account"`
	Nickname  string    `gorm:"column:nickname"`
	Avatar    string    `gorm:"column:avatar"`
	Abstract  string    `gorm:"column:abstract"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

type verificationRecord struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	RequesterID uint64    `gorm:"column:requester_id"`
	ReceiverID  uint64    `gorm:"column:receiver_id"`
	Status      int       `gorm:"column:status"`
	Message     string    `gorm:"column:message"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

type verificationRow struct {
	ID                uint64    `gorm:"column:id"`
	RequesterID       uint64    `gorm:"column:requester_id"`
	ReceiverID        uint64    `gorm:"column:receiver_id"`
	Status            int       `gorm:"column:status"`
	Message           string    `gorm:"column:message"`
	RequesterNickname string    `gorm:"column:requester_nickname"`
	RequesterAvatar   string    `gorm:"column:requester_avatar"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

// SearchUsers matches active users by account, nickname or exact numeric ID.
// The caller's own row is excluded in SQL so the limit is not wasted on it.
func (repository *Repository) SearchUsers(ctx context.Context, userID uint64, keyword string, limit int) ([]domain.User, error) {
	like := "%" + escapeLike(keyword) + "%"
	exactID := uint64(0)
	if parsed, err := strconv.ParseUint(keyword, 10, 64); err == nil {
		exactID = parsed
	}
	var rows []struct {
		ID       uint64 `gorm:"column:id"`
		Account  string `gorm:"column:account"`
		Nickname string `gorm:"column:nickname"`
		Avatar   string `gorm:"column:avatar"`
		Abstract string `gorm:"column:abstract"`
	}
	err := repository.db.WithContext(ctx).Table("users").
		Select("id, account, nickname, avatar, abstract").
		Where("status = ? AND id <> ? AND (account LIKE ? ESCAPE '\\\\' OR nickname LIKE ? ESCAPE '\\\\' OR id = ?)",
			domain.StatusActive, userID, like, like, exactID).
		Order("id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, domain.User{ID: row.ID, Account: row.Account, Nickname: row.Nickname, Avatar: row.Avatar, Abstract: row.Abstract})
	}
	return users, nil
}

func (repository *Repository) ListFriends(ctx context.Context, userID uint64, limit int) ([]friend.FriendRow, error) {
	var rows []friendRow
	err := repository.db.WithContext(ctx).Table("friends AS f").
		Select(`f.friend_id, f.remark, f.created_at,
			u.account, u.nickname, u.avatar, u.abstract`).
		Joins("JOIN users AS u ON u.id = f.friend_id").
		Where("f.user_id = ?", userID).
		Order("f.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
	}
	items := make([]friend.FriendRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, toFriendRow(row))
	}
	return items, nil
}

func (repository *Repository) GetFriend(ctx context.Context, userID, friendID uint64) (friend.FriendRow, error) {
	var row friendRow
	err := repository.db.WithContext(ctx).Table("friends AS f").
		Select(`f.friend_id, f.remark, f.created_at,
			u.account, u.nickname, u.avatar, u.abstract`).
		Joins("JOIN users AS u ON u.id = f.friend_id").
		Where("f.user_id = ? AND f.friend_id = ?", userID, friendID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return friend.FriendRow{}, friend.ErrNotFriend
	}
	if err != nil {
		return friend.FriendRow{}, fmt.Errorf("get friend: %w", err)
	}
	return toFriendRow(row), nil
}

func (repository *Repository) FriendshipFlags(ctx context.Context, userID uint64, otherIDs []uint64) (map[uint64]bool, error) {
	flags := make(map[uint64]bool, len(otherIDs))
	if len(otherIDs) == 0 {
		return flags, nil
	}
	var ids []uint64
	err := repository.db.WithContext(ctx).Table("friends").
		Where("user_id = ? AND friend_id IN ?", userID, otherIDs).Pluck("friend_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("query friendship flags: %w", err)
	}
	for _, id := range ids {
		flags[id] = true
	}
	return flags, nil
}

// CreateRequest re-checks the friendship and lets the pending-pair unique key
// arbitrate concurrent requests in either direction.
func (repository *Repository) CreateRequest(ctx context.Context, requesterID, receiverID uint64, message string) (friend.VerificationRow, error) {
	var result friend.VerificationRow
	err := repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var count int64
		if err := transaction.Table("friends").
			Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", requesterID, receiverID, receiverID, requesterID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check existing friendship: %w", err)
		}
		if count > 0 {
			return friend.ErrAlreadyFriend
		}
		now := time.Now().UTC()
		row := verificationRecord{
			RequesterID: requesterID, ReceiverID: receiverID,
			Status: friend.StatusPending, Message: message, CreatedAt: now, UpdatedAt: now,
		}
		if err := transaction.Table("friend_verifies").Create(&row).Error; err != nil {
			if isDuplicate(err) {
				return friend.ErrDuplicateRequest
			}
			return fmt.Errorf("create friend request: %w", err)
		}
		profile, err := loadProfile(transaction, requesterID)
		if err != nil {
			return err
		}
		result = toVerificationRow(row, profile)
		return nil
	})
	return result, err
}

func (repository *Repository) ListRequests(ctx context.Context, receiverID uint64, status int, limit int) ([]friend.VerificationRow, error) {
	query := repository.db.WithContext(ctx).Table("friend_verifies AS v").
		Select(`v.id, v.requester_id, v.receiver_id, v.status, v.message, v.created_at, v.updated_at,
			u.nickname AS requester_nickname, u.avatar AS requester_avatar`).
		Joins("JOIN users AS u ON u.id = v.requester_id").
		Where("v.receiver_id = ?", receiverID)
	if status >= 0 {
		query = query.Where("v.status = ?", status)
	}
	var rows []verificationRow
	if err := query.Order("v.id DESC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list friend requests: %w", err)
	}
	items := make([]friend.VerificationRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, friend.VerificationRow{
			ID: row.ID, RequesterID: row.RequesterID, ReceiverID: row.ReceiverID, Status: row.Status,
			Message: row.Message, RequesterNickname: row.RequesterNickname, RequesterAvatar: row.RequesterAvatar,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

// Respond locks the request row, validates the reviewer and pending state, and
// writes both friendship rows plus the status change in one transaction. A
// concurrent reviewer blocks on the lock and then sees a handled request.
func (repository *Repository) Respond(ctx context.Context, requestID, reviewerID uint64, accept bool) (friend.VerificationRow, error) {
	var result friend.VerificationRow
	err := repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var row verificationRecord
		err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Table("friend_verifies").
			Where("id = ?", requestID).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return friend.ErrRequestNotFound
		}
		if err != nil {
			return fmt.Errorf("lock friend request: %w", err)
		}
		if row.ReceiverID != reviewerID {
			return friend.ErrNotReceiver
		}
		if row.Status != friend.StatusPending {
			return friend.ErrRequestHandled
		}
		status := friend.StatusRejected
		if accept {
			status = friend.StatusAccepted
			if err := transaction.Exec(`INSERT INTO friends (user_id, friend_id) VALUES (?, ?), (?, ?)
				ON DUPLICATE KEY UPDATE remark = remark`,
				row.RequesterID, row.ReceiverID, row.ReceiverID, row.RequesterID).Error; err != nil {
				return fmt.Errorf("create friendship: %w", err)
			}
		}
		if err := transaction.Table("friend_verifies").Where("id = ?", requestID).Update("status", status).Error; err != nil {
			return fmt.Errorf("update friend request: %w", err)
		}
		profile, err := loadProfile(transaction, row.RequesterID)
		if err != nil {
			return err
		}
		row.Status = status
		row.UpdatedAt = time.Now().UTC()
		result = toVerificationRow(row, profile)
		return nil
	})
	return result, err
}

func (repository *Repository) UpdateRemark(ctx context.Context, userID, friendID uint64, remark string) error {
	result := repository.db.WithContext(ctx).Table("friends").
		Where("user_id = ? AND friend_id = ?", userID, friendID).Update("remark", remark)
	if result.Error != nil {
		return fmt.Errorf("update friend remark: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// MySQL reports zero affected rows when the value is unchanged.
		var count int64
		if err := repository.db.WithContext(ctx).Table("friends").
			Where("user_id = ? AND friend_id = ?", userID, friendID).Count(&count).Error; err != nil {
			return fmt.Errorf("check friend after remark update: %w", err)
		}
		if count == 0 {
			return friend.ErrNotFriend
		}
	}
	return nil
}

// DeleteFriend removes both directions; a missing pair is reported as not
// friends instead of silently succeeding.
func (repository *Repository) DeleteFriend(ctx context.Context, userID, friendID uint64) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Exec(`DELETE FROM friends
			WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
			userID, friendID, friendID, userID)
		if result.Error != nil {
			return fmt.Errorf("delete friendship: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return friend.ErrNotFriend
		}
		return nil
	})
}

func loadProfile(transaction *gorm.DB, userID uint64) (profileRecord, error) {
	var profile profileRecord
	if err := transaction.Table("users").Select("nickname, avatar").Where("id = ?", userID).Take(&profile).Error; err != nil {
		return profileRecord{}, fmt.Errorf("load friend request profile: %w", err)
	}
	return profile, nil
}

func toFriendRow(row friendRow) friend.FriendRow {
	return friend.FriendRow{
		FriendID: row.FriendID, Remark: row.Remark, Account: row.Account,
		Nickname: row.Nickname, Avatar: row.Avatar, Abstract: row.Abstract, CreatedAt: row.CreatedAt,
	}
}

func toVerificationRow(row verificationRecord, profile profileRecord) friend.VerificationRow {
	return friend.VerificationRow{
		ID: row.ID, RequesterID: row.RequesterID, ReceiverID: row.ReceiverID, Status: row.Status,
		Message: row.Message, RequesterNickname: profile.Nickname, RequesterAvatar: profile.Avatar,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func isDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func escapeLike(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}
