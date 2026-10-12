package mysql

import (
	"context"
	"fmt"
	"github.com/kanhai447/GIM/server/internal/group/message"
)

func (repository *Repository) ListHistory(ctx context.Context, userID, groupID, before uint64, limit int) ([]message.Message, error) {
	query := repository.db.WithContext(ctx).Model(&record{}).Where("group_id = ?", groupID).Where("NOT EXISTS (SELECT 1 FROM group_message_deletions AS deletions WHERE deletions.user_id = ? AND deletions.message_id = group_messages.id)", userID)
	if before != 0 {
		query = query.Where("id < ?", before)
	}
	var rows []record
	if err := query.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list group history: %w", err)
	}
	items := make([]message.Message, 0, len(rows))
	for _, row := range rows {
		item, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
