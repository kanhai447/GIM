package mysql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kanhai447/GIM/server/internal/settings/repository"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) ListPublic(ctx context.Context, keys []string) (map[string]json.RawMessage, error) {
	type row struct {
		Key   string `gorm:"column:setting_key"`
		Value []byte `gorm:"column:setting_value"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("settings").Select("setting_key,setting_value").Where("is_public = 1 AND setting_key IN ?", keys).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list public settings: %w", err)
	}
	result := make(map[string]json.RawMessage, len(rows))
	for _, item := range rows {
		result[item.Key] = append(json.RawMessage(nil), item.Value...)
	}
	return result, nil
}
func (r *Repository) UpsertPublic(ctx context.Context, values []repository.Value) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, value := range values {
			result := tx.Exec(`INSERT INTO settings(setting_key,setting_value,is_public,description) VALUES(?,?,1,'GIM public business setting') ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),is_public=1`, value.Key, value.JSON)
			if result.Error != nil {
				return fmt.Errorf("upsert public setting: %w", result.Error)
			}
		}
		return nil
	})
}
