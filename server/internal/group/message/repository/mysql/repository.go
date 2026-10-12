package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/group/message"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

var errDuplicateRace = errors.New("group message duplicate race")

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type record struct {
	ID            uint64          `gorm:"column:id;primaryKey"`
	GroupID       uint64          `gorm:"column:group_id"`
	SenderID      uint64          `gorm:"column:send_user_id"`
	GroupMemberID uint64          `gorm:"column:group_member_id"`
	ClientMsgID   string          `gorm:"column:client_msg_id"`
	Type          uint8           `gorm:"column:msg_type"`
	Preview       string          `gorm:"column:msg_preview"`
	Payload       json.RawMessage `gorm:"column:msg;type:json"`
	CreatedAt     time.Time       `gorm:"column:created_at"`
	UpdatedAt     time.Time       `gorm:"column:updated_at"`
}

func (record) TableName() string { return "group_messages" }

func (repository *Repository) PersistWithSessions(ctx context.Context, value message.Message) (message.PersistResult, error) {
	row, err := encodeRecord(value)
	if err != nil {
		return message.PersistResult{}, err
	}
	var result message.PersistResult
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeGroup struct {
			ID uint64 `gorm:"column:id"`
		}
		// Shared-lock the active group: concurrent sends may proceed, while a
		// dissolve/update cannot cross the persistence boundary mid-transaction.
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Table("`groups`").Select("id").Where("id = ? AND status = 1", value.GroupID).Take(&activeGroup).Error; err != nil {
			return fmt.Errorf("lock active group: %w", err)
		}
		var member struct {
			ID uint64 `gorm:"column:id"`
		}
		if err := tx.Table("group_members").Select("id").Where("group_id = ? AND user_id = ?", value.GroupID, value.SenderID).Take(&member).Error; err != nil {
			return fmt.Errorf("resolve sender group member: %w", err)
		}
		row.GroupMemberID = member.ID
		if err := tx.Create(&row).Error; err != nil {
			if !isDuplicate(err) {
				return fmt.Errorf("create group message: %w", err)
			}
			// End this transaction and read the winner in a fresh transaction.
			// A repeatable-read snapshot opened before the winning commit must not
			// be used to resolve the duplicate.
			return errDuplicateRace
		}
		var memberIDs []uint64
		if err := tx.Table("group_members").Where("group_id = ?", value.GroupID).Order("user_id ASC").Pluck("user_id", &memberIDs).Error; err != nil {
			return fmt.Errorf("snapshot group members: %w", err)
		}
		if len(memberIDs) == 0 {
			return errors.New("group has no effective members")
		}
		if err := upsertSessions(tx, row, memberIDs); err != nil {
			return err
		}
		domain, decodeErr := toDomain(row)
		if decodeErr != nil {
			return decodeErr
		}
		result = message.PersistResult{Message: domain, TargetUserIDs: memberIDs, CreatedNew: true}
		return nil
	})
	if errors.Is(err, errDuplicateRace) {
		existing, queryErr := repository.GetBySenderAndClientMsgID(ctx, value.SenderID, value.ClientMsgID)
		if queryErr != nil {
			return message.PersistResult{}, fmt.Errorf("query duplicate group message: %w", queryErr)
		}
		return message.PersistResult{Message: existing}, nil
	}
	if err != nil {
		return message.PersistResult{}, err
	}
	return result, nil
}

func upsertSessions(tx *gorm.DB, row record, userIDs []uint64) error {
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	values := make([]string, 0, len(userIDs))
	args := make([]any, 0, len(userIDs)*5)
	for _, userID := range userIDs {
		unread := 1
		if userID == row.SenderID {
			unread = 0
		}
		values = append(values, "(?, ?, ?, ?, ?, NULL)")
		args = append(args, userID, row.GroupID, row.ID, row.CreatedAt, unread)
	}
	query := `INSERT INTO group_sessions (user_id, group_id, last_message_id, last_message_at, unread_count, hidden_at) VALUES ` + strings.Join(values, ",") + `
		ON DUPLICATE KEY UPDATE
		last_message_at = IF(VALUES(last_message_id) > COALESCE(last_message_id,0), VALUES(last_message_at), last_message_at),
		last_message_id = GREATEST(COALESCE(last_message_id,0), VALUES(last_message_id)),
		unread_count = unread_count + VALUES(unread_count), hidden_at = NULL`
	if err := tx.Exec(query, args...).Error; err != nil {
		return fmt.Errorf("upsert group sessions: %w", err)
	}
	return nil
}

func (repository *Repository) GetBySenderAndClientMsgID(ctx context.Context, senderID uint64, clientMsgID string) (message.Message, error) {
	var row record
	err := repository.db.WithContext(ctx).Where("send_user_id = ? AND client_msg_id = ?", senderID, clientMsgID).Take(&row).Error
	return result(row, err)
}
func (repository *Repository) GetByID(ctx context.Context, id uint64) (message.Message, error) {
	var row record
	err := repository.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	return result(row, err)
}
func result(row record, err error) (message.Message, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return message.Message{}, message.ErrNotFound
	}
	if err != nil {
		return message.Message{}, fmt.Errorf("query group message: %w", err)
	}
	return toDomain(row)
}
func toDomain(row record) (message.Message, error) {
	var payload protocol.MessagePayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return message.Message{}, fmt.Errorf("decode group message: %w", err)
	}
	return message.Message{ID: row.ID, GroupID: row.GroupID, SenderID: row.SenderID, GroupMemberID: row.GroupMemberID, ClientMsgID: row.ClientMsgID, Type: row.Type, Preview: row.Preview, Payload: payload, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
func encodeRecord(value message.Message) (record, error) {
	payload, err := json.Marshal(value.Payload)
	if err != nil {
		return record{}, fmt.Errorf("encode group message: %w", err)
	}
	return record{GroupID: value.GroupID, SenderID: value.SenderID, GroupMemberID: value.GroupMemberID, ClientMsgID: value.ClientMsgID, Type: value.Type, Preview: value.Preview, Payload: payload}, nil
}
func isDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
