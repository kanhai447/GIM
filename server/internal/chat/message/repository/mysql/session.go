package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/message"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sessionUpdate struct {
	UserID          uint64
	PeerID          uint64
	IncrementUnread bool
}

type sessionRecord struct {
	ID                uint64     `gorm:"column:id;primaryKey"`
	UserID            uint64     `gorm:"column:user_id"`
	PeerUserID        uint64     `gorm:"column:peer_user_id"`
	LastMessageID     uint64     `gorm:"column:last_message_id"`
	LastMessageAt     time.Time  `gorm:"column:last_message_at"`
	LastReadMessageID *uint64    `gorm:"column:last_read_message_id"`
	UnreadCount       uint32     `gorm:"column:unread_count"`
	IsTop             bool       `gorm:"column:is_top"`
	HiddenAt          *time.Time `gorm:"column:hidden_at"`
}

func (sessionRecord) TableName() string { return "chat_sessions" }

type sessionListRecord struct {
	Session           sessionRecord   `gorm:"embedded"`
	PeerNickname      string          `gorm:"column:peer_nickname"`
	PeerAvatar        string          `gorm:"column:peer_avatar"`
	MessageSenderID   uint64          `gorm:"column:message_sender_id"`
	MessageReceiverID uint64          `gorm:"column:message_receiver_id"`
	ClientMsgID       string          `gorm:"column:message_client_msg_id"`
	MessageType       uint8           `gorm:"column:message_type"`
	MessagePreview    string          `gorm:"column:message_preview"`
	MessagePayload    json.RawMessage `gorm:"column:message_payload"`
	MessageCreatedAt  time.Time       `gorm:"column:message_created_at"`
	MessageUpdatedAt  time.Time       `gorm:"column:message_updated_at"`
}

func upsertSession(transaction *gorm.DB, row record, operation sessionUpdate) error {
	unread := 0
	if operation.IncrementUnread {
		unread = 1
	}
	result := transaction.Exec(`
		INSERT INTO chat_sessions
			(user_id, peer_user_id, last_message_id, last_message_at, unread_count, hidden_at)
		VALUES (?, ?, ?, ?, ?, NULL)
		ON DUPLICATE KEY UPDATE
			last_message_at = IF(VALUES(last_message_id) > COALESCE(last_message_id, 0), VALUES(last_message_at), last_message_at),
			last_message_id = GREATEST(COALESCE(last_message_id, 0), VALUES(last_message_id)),
			unread_count = unread_count + VALUES(unread_count),
			hidden_at = NULL`,
		operation.UserID, operation.PeerID, row.ID, row.CreatedAt, unread,
	)
	if result.Error != nil {
		return fmt.Errorf("upsert chat session user %d: %w", operation.UserID, result.Error)
	}
	return nil
}

func (repository *Repository) GetSession(ctx context.Context, userID, peerID uint64) (message.Session, error) {
	var row sessionRecord
	err := repository.db.WithContext(ctx).Where("user_id = ? AND peer_user_id = ?", userID, peerID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return message.Session{}, message.ErrSessionNotFound
	}
	if err != nil {
		return message.Session{}, fmt.Errorf("query chat session: %w", err)
	}
	return sessionToDomain(row), nil
}

func (repository *Repository) ListSessions(ctx context.Context, userID uint64, limit int) ([]message.Session, int64, error) {
	database := repository.db.WithContext(ctx)
	var count int64
	if err := database.Table("chat_sessions").Where("user_id = ? AND hidden_at IS NULL", userID).Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("count chat sessions: %w", err)
	}
	var rows []sessionListRecord
	err := database.Table("chat_sessions AS sessions").
		Select(`sessions.id, sessions.user_id, sessions.peer_user_id, sessions.last_message_id, sessions.last_message_at,
			sessions.last_read_message_id, sessions.unread_count, sessions.is_top, sessions.hidden_at,
			users.nickname AS peer_nickname, users.avatar AS peer_avatar,
			messages.send_user_id AS message_sender_id, messages.rev_user_id AS message_receiver_id,
			messages.client_msg_id AS message_client_msg_id, messages.msg_type AS message_type,
			messages.msg_preview AS message_preview, messages.msg AS message_payload,
			messages.created_at AS message_created_at, messages.updated_at AS message_updated_at`).
		Joins("JOIN users ON users.id = sessions.peer_user_id").
		Joins("JOIN chat_messages AS messages ON messages.id = sessions.last_message_id AND messages.admin_deleted_at IS NULL").
		Where("sessions.user_id = ? AND sessions.hidden_at IS NULL", userID).
		Order("sessions.is_top DESC, sessions.last_message_at DESC, sessions.id DESC").
		Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list chat sessions: %w", err)
	}
	items := make([]message.Session, 0, len(rows))
	for _, row := range rows {
		item, decodeErr := sessionListToDomain(row)
		if decodeErr != nil {
			return nil, 0, decodeErr
		}
		items = append(items, item)
	}
	return items, count, nil
}

func (repository *Repository) MarkRead(ctx context.Context, userID, peerID, readThroughMessageID uint64) (message.Session, error) {
	var result message.Session
	err := repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var target record
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", readThroughMessageID).First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return message.ErrNotFound
			}
			return fmt.Errorf("query read-through message: %w", err)
		}
		if !sameConversation(target, userID, peerID) {
			return message.ErrMessageNotInConversation
		}

		var session sessionRecord
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND peer_user_id = ?", userID, peerID).First(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return message.ErrSessionNotFound
			}
			return fmt.Errorf("lock chat session: %w", err)
		}
		effectiveReadID := readThroughMessageID
		if session.LastReadMessageID != nil && *session.LastReadMessageID > effectiveReadID {
			effectiveReadID = *session.LastReadMessageID
		}
		var unread int64
		if err := transaction.Table("chat_messages AS messages").
			Where("messages.send_user_id = ? AND messages.rev_user_id = ? AND messages.id > ?", peerID, userID, effectiveReadID).
			Where("messages.admin_deleted_at IS NULL").
			Where("NOT EXISTS (SELECT 1 FROM chat_message_deletions AS deletions WHERE deletions.user_id = ? AND deletions.message_id = messages.id)", userID).
			Count(&unread).Error; err != nil {
			return fmt.Errorf("count unread chat messages: %w", err)
		}
		if err := transaction.Model(&session).Updates(map[string]any{
			"last_read_message_id": effectiveReadID,
			"unread_count":         unread,
		}).Error; err != nil {
			return fmt.Errorf("update chat read state: %w", err)
		}
		session.LastReadMessageID = &effectiveReadID
		session.UnreadCount = uint32(unread)
		result = sessionToDomain(session)
		return nil
	})
	if err != nil {
		return message.Session{}, err
	}
	return result, nil
}

func (repository *Repository) SetSessionTop(ctx context.Context, userID, peerID uint64, isTop bool) (message.Session, error) {
	result := repository.db.WithContext(ctx).Model(&sessionRecord{}).
		Where("user_id = ? AND peer_user_id = ?", userID, peerID).Update("is_top", isTop)
	if result.Error != nil {
		return message.Session{}, fmt.Errorf("update chat session top: %w", result.Error)
	}
	return repository.GetSession(ctx, userID, peerID)
}

func (repository *Repository) ToggleSessionTop(ctx context.Context, userID, peerID uint64) (message.Session, error) {
	result := repository.db.WithContext(ctx).Model(&sessionRecord{}).
		Where("user_id = ? AND peer_user_id = ?", userID, peerID).
		UpdateColumn("is_top", gorm.Expr("IF(is_top = 1, 0, 1)"))
	if result.Error != nil {
		return message.Session{}, fmt.Errorf("toggle chat session top: %w", result.Error)
	}
	return repository.GetSession(ctx, userID, peerID)
}

func (repository *Repository) HideSession(ctx context.Context, userID, peerID uint64) error {
	result := repository.db.WithContext(ctx).Model(&sessionRecord{}).
		Where("user_id = ? AND peer_user_id = ?", userID, peerID).Update("hidden_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("hide chat session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return message.ErrSessionNotFound
	}
	return nil
}

func sessionToDomain(row sessionRecord) message.Session {
	return message.Session{
		ID: row.ID, UserID: row.UserID, Peer: message.Peer{ID: row.PeerUserID},
		LastMessageID: row.LastMessageID, LastMessageAt: row.LastMessageAt,
		LastReadMessageID: row.LastReadMessageID, UnreadCount: row.UnreadCount,
		IsTop: row.IsTop, HiddenAt: row.HiddenAt,
	}
}

func sessionListToDomain(row sessionListRecord) (message.Session, error) {
	messageRow := record{
		ID: row.Session.LastMessageID, SenderID: row.MessageSenderID, ReceiverID: row.MessageReceiverID,
		ClientMsgID: row.ClientMsgID, Type: row.MessageType, Preview: row.MessagePreview,
		Payload: row.MessagePayload, CreatedAt: row.MessageCreatedAt, UpdatedAt: row.MessageUpdatedAt,
	}
	lastMessage, err := toDomain(messageRow)
	if err != nil {
		return message.Session{}, err
	}
	item := sessionToDomain(row.Session)
	item.Peer.Nickname = row.PeerNickname
	item.Peer.Avatar = row.PeerAvatar
	item.LastMessage = lastMessage
	return item, nil
}

func sameConversation(row record, first, second uint64) bool {
	return row.SenderID == first && row.ReceiverID == second || row.SenderID == second && row.ReceiverID == first
}
