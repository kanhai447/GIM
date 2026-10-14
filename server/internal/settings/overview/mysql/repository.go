package mysql

import (
	"context"
	"fmt"

	"github.com/kanhai447/GIM/server/internal/settings/overview"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (repository *Repository) Summary(ctx context.Context) (overview.Summary, error) {
	var summary overview.Summary
	err := repository.db.WithContext(ctx).Raw(`SELECT
		(SELECT COUNT(*) FROM users) AS users_total,
		(SELECT COUNT(*) FROM users WHERE status = 1) AS users_active,
		(SELECT COUNT(*) FROM users WHERE status = 2) AS users_disabled,
		(SELECT COUNT(*) FROM chat_messages WHERE admin_deleted_at IS NULL) AS private_messages,
		(SELECT COUNT(*) FROM ` + "`groups`" + ` WHERE status = 1) AS groups_active,
		(SELECT COUNT(*) FROM group_messages WHERE admin_deleted_at IS NULL) AS group_messages,
		(SELECT COUNT(*) FROM user_files) AS user_files,
		(SELECT COUNT(*) FROM audit_logs) AS audit_events`).Scan(&summary).Error
	if err != nil {
		return overview.Summary{}, fmt.Errorf("load admin overview: %w", err)
	}
	return summary, nil
}
