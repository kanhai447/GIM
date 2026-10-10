package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type FriendshipRepository struct{ db *gorm.DB }

func NewFriendshipRepository(db *gorm.DB) *FriendshipRepository {
	return &FriendshipRepository{db: db}
}

func (repository *FriendshipRepository) AreFriends(ctx context.Context, first, second uint64) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Table("friends").
		Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", first, second, second, first).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("query friendship: %w", err)
	}
	return count > 0, nil
}

func (repository *FriendshipRepository) CanChat(ctx context.Context, userID uint64) (bool, error) {
	var restricted int
	err := repository.db.WithContext(ctx).Table("user_confs").
		Select("COALESCE(MAX(curtail_chat), 0)").Where("user_id = ?", userID).Scan(&restricted).Error
	if err != nil {
		return false, fmt.Errorf("query private chat policy: %w", err)
	}
	return restricted == 0, nil
}
