package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/message"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sessionRecord struct {
	ID                uint64     `gorm:"column:id;primaryKey"`
	UserID            uint64     `gorm:"column:user_id"`
	GroupID           uint64     `gorm:"column:group_id"`
	LastMessageID     uint64     `gorm:"column:last_message_id"`
	LastMessageAt     time.Time  `gorm:"column:last_message_at"`
	LastReadMessageID *uint64    `gorm:"column:last_read_message_id"`
	UnreadCount       uint32     `gorm:"column:unread_count"`
	IsTop             bool       `gorm:"column:is_top"`
	HiddenAt          *time.Time `gorm:"column:hidden_at"`
}

func (sessionRecord) TableName() string { return "group_sessions" }

type sessionListRecord struct {
	Session          sessionRecord   `gorm:"embedded"`
	GroupName        string          `gorm:"column:group_name"`
	GroupAvatar      string          `gorm:"column:group_avatar"`
	MessageSenderID  uint64          `gorm:"column:message_sender_id"`
	MessageMemberID  uint64          `gorm:"column:message_member_id"`
	MessageClientID  string          `gorm:"column:message_client_id"`
	MessageType      uint8           `gorm:"column:message_type"`
	MessagePreview   string          `gorm:"column:message_preview"`
	MessagePayload   json.RawMessage `gorm:"column:message_payload"`
	MessageCreatedAt time.Time       `gorm:"column:message_created_at"`
	MessageUpdatedAt time.Time       `gorm:"column:message_updated_at"`
}

func (repository *Repository) ListSessions(ctx context.Context, userID uint64, limit int) ([]message.Session, int64, error) {
	db := repository.db.WithContext(ctx)
	var count int64
	if err := db.Table("group_sessions AS sessions").Joins("JOIN group_members AS membership ON membership.group_id = sessions.group_id AND membership.user_id = sessions.user_id").Joins("JOIN `groups` ON groups.id = sessions.group_id AND groups.status = 1").Where("sessions.user_id = ? AND sessions.hidden_at IS NULL", userID).Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("count group sessions: %w", err)
	}
	var rows []sessionListRecord
	err := db.Table("group_sessions AS sessions").Select(`sessions.id,sessions.user_id,sessions.group_id,sessions.last_message_id,sessions.last_message_at,sessions.last_read_message_id,sessions.unread_count,sessions.is_top,sessions.hidden_at,groups.name AS group_name,groups.avatar AS group_avatar,messages.send_user_id AS message_sender_id,messages.group_member_id AS message_member_id,messages.client_msg_id AS message_client_id,messages.msg_type AS message_type,messages.msg_preview AS message_preview,messages.msg AS message_payload,messages.created_at AS message_created_at,messages.updated_at AS message_updated_at`).Joins("JOIN group_members AS membership ON membership.group_id = sessions.group_id AND membership.user_id = sessions.user_id").Joins("JOIN `groups` ON groups.id = sessions.group_id AND groups.status = 1").Joins("JOIN group_messages AS messages ON messages.id = sessions.last_message_id AND messages.admin_deleted_at IS NULL").Where("sessions.user_id = ? AND sessions.hidden_at IS NULL", userID).Order("sessions.is_top DESC,sessions.last_message_at DESC,sessions.id DESC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list group sessions: %w", err)
	}
	items := make([]message.Session, 0, len(rows))
	for _, row := range rows {
		last, decodeErr := toDomain(record{ID: row.Session.LastMessageID, GroupID: row.Session.GroupID, SenderID: row.MessageSenderID, GroupMemberID: row.MessageMemberID, ClientMsgID: row.MessageClientID, Type: row.MessageType, Preview: row.MessagePreview, Payload: row.MessagePayload, CreatedAt: row.MessageCreatedAt, UpdatedAt: row.MessageUpdatedAt})
		if decodeErr != nil {
			return nil, 0, decodeErr
		}
		items = append(items, message.Session{ID: row.Session.ID, UserID: row.Session.UserID, Group: message.GroupInfo{ID: row.Session.GroupID, Name: row.GroupName, Avatar: row.GroupAvatar}, LastMessage: last, LastMessageID: row.Session.LastMessageID, LastMessageAt: row.Session.LastMessageAt, LastReadMessageID: row.Session.LastReadMessageID, UnreadCount: row.Session.UnreadCount, IsTop: row.Session.IsTop, HiddenAt: row.Session.HiddenAt})
	}
	return items, count, nil
}

func (repository *Repository) MarkRead(ctx context.Context, userID, groupID, readID uint64) (message.Session, error) {
	var output message.Session
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target record
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", readID).Take(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return message.ErrNotFound
			}
			return fmt.Errorf("query group read message: %w", err)
		}
		if target.GroupID != groupID {
			return message.ErrMessageNotInGroup
		}
		var session sessionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND group_id = ?", userID, groupID).Take(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return message.ErrSessionNotFound
			}
			return fmt.Errorf("lock group session: %w", err)
		}
		effective := readID
		if session.LastReadMessageID != nil && *session.LastReadMessageID > effective {
			effective = *session.LastReadMessageID
		}
		var unread int64
		if err := tx.Table("group_messages AS messages").Where("messages.group_id = ? AND messages.send_user_id <> ? AND messages.id > ? AND messages.admin_deleted_at IS NULL", groupID, userID, effective).Where("NOT EXISTS (SELECT 1 FROM group_message_deletions AS deletions WHERE deletions.user_id = ? AND deletions.message_id = messages.id)", userID).Count(&unread).Error; err != nil {
			return fmt.Errorf("count unread group messages: %w", err)
		}
		if err := tx.Model(&session).Updates(map[string]any{"last_read_message_id": effective, "unread_count": unread}).Error; err != nil {
			return fmt.Errorf("update group read state: %w", err)
		}
		session.LastReadMessageID = &effective
		session.UnreadCount = uint32(unread)
		output = message.Session{ID: session.ID, UserID: session.UserID, Group: message.GroupInfo{ID: session.GroupID}, LastMessageID: session.LastMessageID, LastMessageAt: session.LastMessageAt, LastReadMessageID: session.LastReadMessageID, UnreadCount: session.UnreadCount, IsTop: session.IsTop, HiddenAt: session.HiddenAt}
		return nil
	})
	if err != nil {
		return message.Session{}, err
	}
	return output, nil
}
