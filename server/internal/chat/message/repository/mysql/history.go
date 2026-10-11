package mysql

import (
	"context"
	"fmt"

	"github.com/kanhai447/GIM/server/internal/chat/message"
)

func (repository *Repository) ListHistory(
	ctx context.Context,
	userID, peerID, beforeMessageID uint64,
	limit int,
) ([]message.Message, error) {
	lowID, highID := userID, peerID
	if lowID > highID {
		lowID, highID = highID, lowID
	}
	query := repository.db.WithContext(ctx).Model(&record{}).
		Where("conversation_low_id = ? AND conversation_high_id = ?", lowID, highID).
		Where("NOT EXISTS (SELECT 1 FROM chat_message_deletions AS deletions WHERE deletions.user_id = ? AND deletions.message_id = chat_messages.id)", userID)
	if beforeMessageID != 0 {
		query = query.Where("id < ?", beforeMessageID)
	}
	var rows []record
	if err := query.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list chat history: %w", err)
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
