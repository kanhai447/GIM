package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type record struct {
	ID          uint64          `gorm:"column:id;primaryKey"`
	SenderID    uint64          `gorm:"column:send_user_id"`
	ReceiverID  uint64          `gorm:"column:rev_user_id"`
	ClientMsgID string          `gorm:"column:client_msg_id"`
	Type        uint8           `gorm:"column:msg_type"`
	Preview     string          `gorm:"column:msg_preview"`
	Payload     json.RawMessage `gorm:"column:msg;type:json"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

func (record) TableName() string { return "chat_messages" }

func (repository *Repository) Create(ctx context.Context, value message.Message) (message.Message, error) {
	row, err := encodeRecord(value)
	if err != nil {
		return message.Message{}, err
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicate(err) {
			return message.Message{}, message.ErrDuplicate
		}
		return message.Message{}, fmt.Errorf("create chat message: %w", err)
	}
	return toDomain(row)
}

func (repository *Repository) PersistWithSessions(ctx context.Context, value message.Message) (message.PersistResult, error) {
	row, err := encodeRecord(value)
	if err != nil {
		return message.PersistResult{}, err
	}
	var result message.PersistResult
	err = repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if createErr := transaction.Create(&row).Error; createErr != nil {
			if !isDuplicate(createErr) {
				return fmt.Errorf("create chat message: %w", createErr)
			}
			var existing record
			queryErr := transaction.Where("send_user_id = ? AND client_msg_id = ?", value.SenderID, value.ClientMsgID).First(&existing).Error
			if queryErr != nil {
				return fmt.Errorf("query duplicate chat message: %w", queryErr)
			}
			domain, decodeErr := toDomain(existing)
			if decodeErr != nil {
				return decodeErr
			}
			result = message.PersistResult{Message: domain}
			return nil
		}

		operations := []sessionUpdate{
			{UserID: row.SenderID, PeerID: row.ReceiverID},
			{UserID: row.ReceiverID, PeerID: row.SenderID, IncrementUnread: true},
		}
		if operations[0].UserID > operations[1].UserID {
			operations[0], operations[1] = operations[1], operations[0]
		}
		for _, operation := range operations {
			if updateErr := upsertSession(transaction, row, operation); updateErr != nil {
				return updateErr
			}
		}
		domain, decodeErr := toDomain(row)
		if decodeErr != nil {
			return decodeErr
		}
		result = message.PersistResult{Message: domain, CreatedNew: true}
		return nil
	})
	if err != nil {
		return message.PersistResult{}, err
	}
	return result, nil
}

func (repository *Repository) GetBySenderAndClientMsgID(ctx context.Context, senderID uint64, clientMsgID string) (message.Message, error) {
	var row record
	err := repository.db.WithContext(ctx).Where("send_user_id = ? AND client_msg_id = ?", senderID, clientMsgID).First(&row).Error
	return repository.result(row, err)
}

func (repository *Repository) GetByID(ctx context.Context, messageID uint64) (message.Message, error) {
	var row record
	err := repository.db.WithContext(ctx).Where("id = ?", messageID).First(&row).Error
	return repository.result(row, err)
}

func (repository *Repository) result(row record, err error) (message.Message, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return message.Message{}, message.ErrNotFound
	}
	if err != nil {
		return message.Message{}, fmt.Errorf("query chat message: %w", err)
	}
	return toDomain(row)
}

func toDomain(row record) (message.Message, error) {
	var payload protocol.MessagePayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return message.Message{}, fmt.Errorf("decode chat message: %w", err)
	}
	return message.Message{
		ID: row.ID, SenderID: row.SenderID, ReceiverID: row.ReceiverID, ClientMsgID: row.ClientMsgID,
		Type: row.Type, Preview: row.Preview, Payload: payload, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func encodeRecord(value message.Message) (record, error) {
	payload, err := json.Marshal(value.Payload)
	if err != nil {
		return record{}, fmt.Errorf("encode chat message: %w", err)
	}
	return record{
		SenderID: value.SenderID, ReceiverID: value.ReceiverID, ClientMsgID: value.ClientMsgID,
		Type: value.Type, Preview: value.Preview, Payload: payload,
	}, nil
}

func isDuplicate(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
