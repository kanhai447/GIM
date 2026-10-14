package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	chatadmin "github.com/kanhai447/GIM/server/internal/chat/message/admin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (repository *Repository) List(ctx context.Context, query chatadmin.Query) ([]chatadmin.Message, error) {
	database := repository.db.WithContext(ctx).Table("chat_messages AS messages").
		Select(`messages.id, messages.client_msg_id, messages.msg_type AS message_type,
			messages.msg_preview AS preview, messages.msg AS payload, messages.created_at,
			sender.id AS sender__id, sender.nickname AS sender__nickname, sender.avatar AS sender__avatar,
			receiver.id AS receiver__id, receiver.nickname AS receiver__nickname, receiver.avatar AS receiver__avatar`).
		Joins("JOIN users AS sender ON sender.id = messages.send_user_id").
		Joins("JOIN users AS receiver ON receiver.id = messages.rev_user_id").
		Where("messages.admin_deleted_at IS NULL")
	if query.BeforeID != 0 {
		database = database.Where("messages.id < ?", query.BeforeID)
	}
	if query.UserID != 0 {
		database = database.Where("messages.send_user_id = ? OR messages.rev_user_id = ?", query.UserID, query.UserID)
	}
	if query.SenderID != 0 {
		database = database.Where("messages.send_user_id = ?", query.SenderID)
	}
	if query.ReceiverID != 0 {
		database = database.Where("messages.rev_user_id = ?", query.ReceiverID)
	}
	type row struct {
		ID               uint64    `gorm:"column:id"`
		ClientMsgID      string    `gorm:"column:client_msg_id"`
		MessageType      uint8     `gorm:"column:message_type"`
		Preview          string    `gorm:"column:preview"`
		Payload          []byte    `gorm:"column:payload"`
		CreatedAt        time.Time `gorm:"column:created_at"`
		SenderID         uint64    `gorm:"column:sender__id"`
		SenderNickname   string    `gorm:"column:sender__nickname"`
		SenderAvatar     string    `gorm:"column:sender__avatar"`
		ReceiverID       uint64    `gorm:"column:receiver__id"`
		ReceiverNickname string    `gorm:"column:receiver__nickname"`
		ReceiverAvatar   string    `gorm:"column:receiver__avatar"`
	}
	var rows []row
	if err := database.Order("messages.id DESC").Limit(query.Limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list admin chat messages: %w", err)
	}
	items := make([]chatadmin.Message, 0, len(rows))
	for _, value := range rows {
		items = append(items, chatadmin.Message{ID: value.ID, ClientMsgID: value.ClientMsgID, MessageType: value.MessageType, Preview: value.Preview, Payload: value.Payload, CreatedAt: value.CreatedAt,
			Sender:   chatadmin.User{ID: value.SenderID, Nickname: value.SenderNickname, Avatar: value.SenderAvatar},
			Receiver: chatadmin.User{ID: value.ReceiverID, Nickname: value.ReceiverNickname, Avatar: value.ReceiverAvatar}})
	}
	return items, nil
}

func (repository *Repository) Delete(ctx context.Context, messageID uint64) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var message struct {
			ID         uint64 `gorm:"column:id"`
			SenderID   uint64 `gorm:"column:send_user_id"`
			ReceiverID uint64 `gorm:"column:rev_user_id"`
		}
		err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Table("chat_messages").
			Select("id, send_user_id, rev_user_id").Where("id = ? AND admin_deleted_at IS NULL", messageID).Take(&message).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatadmin.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock admin chat message: %w", err)
		}
		if err := transaction.Table("chat_messages").Where("id = ?", messageID).Update("admin_deleted_at", time.Now().UTC()).Error; err != nil {
			return fmt.Errorf("delete admin chat message: %w", err)
		}
		for _, pair := range [][2]uint64{{message.SenderID, message.ReceiverID}, {message.ReceiverID, message.SenderID}} {
			if err := repairSession(transaction, pair[0], pair[1], messageID); err != nil {
				return err
			}
		}
		return nil
	})
}

func repairSession(transaction *gorm.DB, userID, peerID, deletedID uint64) error {
	var session struct {
		ID                uint64  `gorm:"column:id"`
		LastMessageID     *uint64 `gorm:"column:last_message_id"`
		LastReadMessageID *uint64 `gorm:"column:last_read_message_id"`
	}
	err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Table("chat_sessions").
		Select("id, last_message_id, last_read_message_id").Where("user_id = ? AND peer_user_id = ?", userID, peerID).Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || session.LastMessageID == nil || *session.LastMessageID != deletedID {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock chat session after admin delete: %w", err)
	}
	low, high := userID, peerID
	if low > high {
		low, high = high, low
	}
	var latest struct {
		ID        uint64    `gorm:"column:id"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	err = transaction.Table("chat_messages AS messages").Select("messages.id, messages.created_at").
		Where("messages.conversation_low_id = ? AND messages.conversation_high_id = ? AND messages.admin_deleted_at IS NULL", low, high).
		Where("NOT EXISTS (SELECT 1 FROM chat_message_deletions AS d WHERE d.user_id = ? AND d.message_id = messages.id)", userID).
		Order("messages.id DESC").Take(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return transaction.Table("chat_sessions").Where("id = ?", session.ID).Updates(map[string]any{
			"last_message_id": nil, "last_message_at": nil, "unread_count": 0, "hidden_at": time.Now().UTC(),
		}).Error
	}
	if err != nil {
		return fmt.Errorf("find previous chat message after admin delete: %w", err)
	}
	query := transaction.Table("chat_messages AS messages").Where("messages.send_user_id = ? AND messages.rev_user_id = ? AND messages.admin_deleted_at IS NULL", peerID, userID).
		Where("NOT EXISTS (SELECT 1 FROM chat_message_deletions AS d WHERE d.user_id = ? AND d.message_id = messages.id)", userID)
	if session.LastReadMessageID != nil {
		query = query.Where("messages.id > ?", *session.LastReadMessageID)
	}
	var unread int64
	if err := query.Count(&unread).Error; err != nil {
		return fmt.Errorf("recount unread after admin chat delete: %w", err)
	}
	return transaction.Table("chat_sessions").Where("id = ?", session.ID).Updates(map[string]any{
		"last_message_id": latest.ID, "last_message_at": latest.CreatedAt, "unread_count": unread,
	}).Error
}
