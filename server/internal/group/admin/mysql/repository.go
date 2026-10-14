package mysql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	groupadmin "github.com/kanhai447/GIM/server/internal/group/admin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type groupMemberRecord struct {
	ID uint64 `gorm:"column:id;primaryKey"`
}

func (groupMemberRecord) TableName() string { return "group_members" }

func (repository *Repository) List(ctx context.Context, query groupadmin.Query) ([]groupadmin.Group, error) {
	database := repository.db.WithContext(ctx).Table("`groups` AS g").Select(`g.id, g.name, g.avatar, g.description,
		g.owner_id, owner.nickname AS owner_nickname, g.status, g.created_at,
		(SELECT COUNT(*) FROM group_members WHERE group_id = g.id) AS member_count,
		(SELECT COUNT(*) FROM group_messages WHERE group_id = g.id AND admin_deleted_at IS NULL) AS message_count`).
		Joins("JOIN users AS owner ON owner.id = g.owner_id")
	if query.BeforeID != 0 {
		database = database.Where("g.id < ?", query.BeforeID)
	}
	if query.Search != "" {
		database = database.Where("g.name LIKE ? ESCAPE '\\\\'", "%"+escapeLike(query.Search)+"%")
	}
	if query.Status != 0 {
		database = database.Where("g.status = ?", query.Status)
	}
	var groups []groupadmin.Group
	if err := database.Order("g.id DESC").Limit(query.Limit).Scan(&groups).Error; err != nil {
		return nil, fmt.Errorf("list admin groups: %w", err)
	}
	if len(groups) == 0 {
		return groups, nil
	}
	ids := make([]uint64, 0, len(groups))
	byID := make(map[uint64]int, len(groups))
	for index := range groups {
		ids = append(ids, groups[index].ID)
		byID[groups[index].ID] = index
	}
	type member struct {
		GroupID uint64 `gorm:"column:group_id"`
		UserID  uint64 `gorm:"column:user_id"`
	}
	var members []member
	if err := repository.db.WithContext(ctx).Table("group_members").Select("group_id, user_id").Where("group_id IN ?", ids).Scan(&members).Error; err != nil {
		return nil, fmt.Errorf("list admin group members: %w", err)
	}
	for _, value := range members {
		index := byID[value.GroupID]
		groups[index].MemberIDs = append(groups[index].MemberIDs, value.UserID)
	}
	return groups, nil
}

func (repository *Repository) ListMessages(ctx context.Context, query groupadmin.MessageQuery) ([]groupadmin.Message, error) {
	type row struct {
		ID             uint64    `gorm:"column:id"`
		GroupID        uint64    `gorm:"column:group_id"`
		SenderID       uint64    `gorm:"column:sender_id"`
		SenderNickname string    `gorm:"column:sender_nickname"`
		SenderAvatar   string    `gorm:"column:sender_avatar"`
		MemberNickname string    `gorm:"column:member_nickname"`
		MessageType    uint8     `gorm:"column:message_type"`
		Preview        string    `gorm:"column:preview"`
		Payload        []byte    `gorm:"column:payload"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	database := repository.db.WithContext(ctx).Table("group_messages AS messages").Select(`messages.id, messages.group_id, messages.send_user_id AS sender_id,
		users.nickname AS sender_nickname, users.avatar AS sender_avatar, members.member_nickname,
		messages.msg_type AS message_type, messages.msg_preview AS preview, messages.msg AS payload, messages.created_at`).
		Joins("JOIN users ON users.id = messages.send_user_id").
		Joins("LEFT JOIN group_members AS members ON members.id = messages.group_member_id").
		Where("messages.group_id = ? AND messages.admin_deleted_at IS NULL", query.GroupID)
	if query.BeforeID != 0 {
		database = database.Where("messages.id < ?", query.BeforeID)
	}
	var rows []row
	if err := database.Order("messages.id DESC").Limit(query.Limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list admin group messages: %w", err)
	}
	items := make([]groupadmin.Message, 0, len(rows))
	for _, value := range rows {
		items = append(items, groupadmin.Message{ID: value.ID, GroupID: value.GroupID, SenderID: value.SenderID, SenderNickname: value.SenderNickname, SenderAvatar: value.SenderAvatar, MemberNickname: value.MemberNickname, MessageType: value.MessageType, Preview: value.Preview, Payload: value.Payload, CreatedAt: value.CreatedAt})
	}
	return items, nil
}

func (repository *Repository) Dissolve(ctx context.Context, groupID uint64) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Table("`groups`").Where("id = ? AND status = 1", groupID).Update("status", 2)
		if result.Error != nil {
			return fmt.Errorf("admin dissolve group: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return groupadmin.ErrNotFound
		}
		if err := transaction.Where("group_id = ?", groupID).Delete(&groupMemberRecord{}).Error; err != nil {
			return fmt.Errorf("admin remove group members: %w", err)
		}
		if err := transaction.Table("group_sessions").Where("group_id = ?", groupID).Update("hidden_at", time.Now().UTC()).Error; err != nil {
			return fmt.Errorf("admin hide dissolved group sessions: %w", err)
		}
		return nil
	})
}

func (repository *Repository) DeleteMessage(ctx context.Context, messageID uint64) error {
	return repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var value struct {
			ID      uint64 `gorm:"column:id"`
			GroupID uint64 `gorm:"column:group_id"`
		}
		err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Table("group_messages").Select("id, group_id").Where("id = ? AND admin_deleted_at IS NULL", messageID).Take(&value).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return groupadmin.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock admin group message: %w", err)
		}
		if err := transaction.Table("group_messages").Where("id = ?", messageID).Update("admin_deleted_at", time.Now().UTC()).Error; err != nil {
			return fmt.Errorf("delete admin group message: %w", err)
		}
		var sessionIDs []uint64
		if err := transaction.Table("group_sessions").Where("group_id = ? AND last_message_id = ?", value.GroupID, messageID).Pluck("id", &sessionIDs).Error; err != nil {
			return fmt.Errorf("find group sessions after admin delete: %w", err)
		}
		for _, sessionID := range sessionIDs {
			if err := repairGroupSession(transaction, sessionID, value.GroupID); err != nil {
				return err
			}
		}
		return nil
	})
}

func repairGroupSession(transaction *gorm.DB, sessionID, groupID uint64) error {
	var session struct {
		ID                uint64  `gorm:"column:id"`
		UserID            uint64  `gorm:"column:user_id"`
		LastReadMessageID *uint64 `gorm:"column:last_read_message_id"`
	}
	if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Table("group_sessions").Select("id, user_id, last_read_message_id").Where("id = ?", sessionID).Take(&session).Error; err != nil {
		return fmt.Errorf("lock group session after admin delete: %w", err)
	}
	var latest struct {
		ID        uint64    `gorm:"column:id"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	err := transaction.Table("group_messages AS messages").Select("messages.id, messages.created_at").Where("messages.group_id = ? AND messages.admin_deleted_at IS NULL", groupID).
		Where("NOT EXISTS (SELECT 1 FROM group_message_deletions AS d WHERE d.user_id = ? AND d.message_id = messages.id)", session.UserID).Order("messages.id DESC").Take(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return transaction.Table("group_sessions").Where("id = ?", session.ID).Updates(map[string]any{"last_message_id": nil, "last_message_at": nil, "unread_count": 0, "hidden_at": time.Now().UTC()}).Error
	}
	if err != nil {
		return fmt.Errorf("find previous group message after admin delete: %w", err)
	}
	query := transaction.Table("group_messages AS messages").Where("messages.group_id = ? AND messages.send_user_id <> ? AND messages.admin_deleted_at IS NULL", groupID, session.UserID).
		Where("NOT EXISTS (SELECT 1 FROM group_message_deletions AS d WHERE d.user_id = ? AND d.message_id = messages.id)", session.UserID)
	if session.LastReadMessageID != nil {
		query = query.Where("messages.id > ?", *session.LastReadMessageID)
	}
	var unread int64
	if err := query.Count(&unread).Error; err != nil {
		return fmt.Errorf("recount group unread after admin delete: %w", err)
	}
	return transaction.Table("group_sessions").Where("id = ?", session.ID).Updates(map[string]any{"last_message_id": latest.ID, "last_message_at": latest.CreatedAt, "unread_count": unread}).Error
}

func escapeLike(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}
