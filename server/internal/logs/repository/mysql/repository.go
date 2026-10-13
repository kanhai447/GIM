package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/logs/domain"
	"github.com/kanhai447/GIM/server/internal/logs/repository"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type record struct {
	ID            uint64    `gorm:"column:id;primaryKey"`
	EventID       string    `gorm:"column:event_id"`
	SchemaVersion int       `gorm:"column:schema_version"`
	EventType     string    `gorm:"column:event_type"`
	Service       string    `gorm:"column:service"`
	Action        string    `gorm:"column:action"`
	UserID        *uint64   `gorm:"column:user_id"`
	Role          *int32    `gorm:"column:role"`
	ResourceType  string    `gorm:"column:resource_type"`
	ResourceID    string    `gorm:"column:resource_id"`
	RequestID     string    `gorm:"column:request_id"`
	TraceID       string    `gorm:"column:trace_id"`
	Status        string    `gorm:"column:status"`
	ErrorCode     uint32    `gorm:"column:error_code"`
	IP            string    `gorm:"column:ip"`
	Method        string    `gorm:"column:method"`
	Path          string    `gorm:"column:path"`
	OccurredAt    time.Time `gorm:"column:occurred_at"`
	Metadata      []byte    `gorm:"column:metadata;type:json"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (record) TableName() string { return "audit_logs" }

func (repo *Repository) Insert(ctx context.Context, event audit.Event) (bool, error) {
	if err := event.Validate(); err != nil {
		return false, err
	}
	metadata := event.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	value := record{
		EventID: event.EventID, SchemaVersion: event.SchemaVersion, EventType: event.EventType, Service: event.Service,
		Action: event.Action, UserID: event.UserID, Role: event.Role, ResourceType: event.ResourceType,
		ResourceID: event.ResourceID, RequestID: event.RequestID, TraceID: event.TraceID, Status: event.Status,
		ErrorCode: event.ErrorCode, IP: event.IP, Method: event.Method, Path: event.Path,
		OccurredAt: event.OccurredAt.UTC(), Metadata: []byte(metadata),
	}
	err := repo.db.WithContext(ctx).Create(&value).Error
	if err == nil {
		return true, nil
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return false, nil
	}
	return false, err
}

func (repo *Repository) List(ctx context.Context, query domain.Query) (domain.Page, error) {
	db := repo.db.WithContext(ctx).Model(&record{})
	if query.BeforeID > 0 {
		var cursor record
		if err := repo.db.WithContext(ctx).Select("id", "occurred_at").First(&cursor, "id = ?", query.BeforeID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Page{}, repository.ErrNotFound
			}
			return domain.Page{}, err
		}
		db = db.Where("occurred_at < ? OR (occurred_at = ? AND id < ?)", cursor.OccurredAt, cursor.OccurredAt, cursor.ID)
	}
	if query.From != nil {
		db = db.Where("occurred_at >= ?", query.From.UTC())
	}
	if query.To != nil {
		db = db.Where("occurred_at <= ?", query.To.UTC())
	}
	if query.Service != "" {
		db = db.Where("service = ?", query.Service)
	}
	if query.EventType != "" {
		db = db.Where("event_type = ?", query.EventType)
	}
	if query.UserID != nil {
		db = db.Where("user_id = ?", *query.UserID)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.RequestID != "" {
		db = db.Where("request_id = ?", query.RequestID)
	}
	var rows []record
	if err := db.Order("occurred_at DESC").Order("id DESC").Limit(query.Limit + 1).Find(&rows).Error; err != nil {
		return domain.Page{}, err
	}
	hasMore := len(rows) > query.Limit
	if hasMore {
		rows = rows[:query.Limit]
	}
	items := make([]domain.Log, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomain(row))
	}
	var next uint64
	if hasMore && len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return domain.Page{Items: items, NextBeforeID: next, HasMore: hasMore}, nil
}

func (repo *Repository) Get(ctx context.Context, id uint64) (domain.Log, error) {
	var value record
	if err := repo.db.WithContext(ctx).First(&value, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Log{}, repository.ErrNotFound
		}
		return domain.Log{}, err
	}
	return toDomain(value), nil
}

func toDomain(value record) domain.Log {
	return domain.Log{
		ID: value.ID, EventID: value.EventID, SchemaVersion: value.SchemaVersion, EventType: value.EventType,
		Service: value.Service, Action: value.Action, UserID: value.UserID, Role: value.Role,
		ResourceType: value.ResourceType, ResourceID: value.ResourceID, RequestID: value.RequestID,
		TraceID: value.TraceID, Status: value.Status, ErrorCode: value.ErrorCode, IP: value.IP,
		Method: value.Method, Path: value.Path, OccurredAt: value.OccurredAt,
		Metadata: json.RawMessage(value.Metadata), CreatedAt: value.CreatedAt,
	}
}
