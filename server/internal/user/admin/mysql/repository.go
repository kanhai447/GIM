package mysql

import (
	"context"
	"fmt"
	"strings"

	useradmin "github.com/kanhai447/GIM/server/internal/user/admin"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (repository *Repository) List(ctx context.Context, query useradmin.Query) ([]useradmin.User, error) {
	database := repository.db.WithContext(ctx).Table("users AS users").
		Select(`users.id, users.account, users.nickname, users.avatar, users.ip, users.addr AS address,
			users.role, users.status, users.created_at,
			COALESCE(confs.curtail_chat, 0) AS curtail_chat,
			COALESCE(confs.add_user, 0) AS curtail_add_user,
			COALESCE(confs.create_group, 0) AS curtail_create_group,
			COALESCE(confs.in_group_chat, 0) AS curtail_in_group_chat,
			(SELECT COUNT(*) FROM chat_messages WHERE send_user_id = users.id) AS send_message_count,
			(SELECT COUNT(*) FROM group_members WHERE user_id = users.id AND role IN (1,2)) AS group_admin_count,
			(SELECT COUNT(*) FROM group_members WHERE user_id = users.id) AS group_count`).
		Joins("LEFT JOIN user_confs AS confs ON confs.user_id = users.id")
	if query.BeforeID != 0 {
		database = database.Where("users.id < ?", query.BeforeID)
	}
	if query.Search != "" {
		like := "%" + escapeLike(query.Search) + "%"
		database = database.Where("users.account LIKE ? ESCAPE '\\\\' OR users.nickname LIKE ? ESCAPE '\\\\'", like, like)
	}
	if query.Status != 0 {
		database = database.Where("users.status = ?", query.Status)
	}
	var rows []useradmin.User
	if err := database.Order("users.id DESC").Limit(query.Limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}
	return rows, nil
}

func (repository *Repository) UpdateRestrictions(ctx context.Context, input useradmin.Restrictions) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var count int64
		if err := transaction.Table("users").Where("id = ?", input.UserID).Count(&count).Error; err != nil {
			return fmt.Errorf("check restriction user: %w", err)
		}
		if count != 1 {
			return useradmin.ErrNotFound
		}
		result := transaction.Exec(`INSERT INTO user_confs
			(user_id, curtail_chat, add_user, create_group, in_group_chat)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE curtail_chat=VALUES(curtail_chat), add_user=VALUES(add_user),
			create_group=VALUES(create_group), in_group_chat=VALUES(in_group_chat)`,
			input.UserID, input.CurtailChat, input.CurtailAddUser, input.CurtailCreateGroup, input.CurtailInGroupChat)
		if result.Error != nil {
			return fmt.Errorf("update user restrictions: %w", result.Error)
		}
		return nil
	})
}

func (repository *Repository) UpdateStatus(ctx context.Context, userID uint64, status domain.Status) error {
	if !status.Valid() {
		return useradmin.ErrInvalid
	}
	result := repository.db.WithContext(ctx).Table("users").Where("id = ?", userID).Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("update user status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := repository.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&count).Error; err != nil {
			return fmt.Errorf("check user status target: %w", err)
		}
		if count == 0 {
			return useradmin.ErrNotFound
		}
	}
	return nil
}

func escapeLike(value string) string {
	value = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
	return value
}
