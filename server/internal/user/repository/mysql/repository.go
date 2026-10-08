// Package mysql implements the user repository with GORM and MySQL.
package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/kanhai447/GIM/server/internal/user/repository"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (repo *Repository) Create(ctx context.Context, user *domain.User) error {
	record := recordFromDomain(*user)
	if err := repo.db.WithContext(ctx).Create(&record).Error; err != nil {
		return mapError("create", err)
	}
	*user = record.toDomain()
	return nil
}

func (repo *Repository) GetByID(ctx context.Context, userID uint64) (domain.User, error) {
	var record userRecord
	if err := repo.db.WithContext(ctx).First(&record, userID).Error; err != nil {
		return domain.User{}, mapError("get by id", err)
	}
	return record.toDomain(), nil
}

func (repo *Repository) GetByAccount(ctx context.Context, account string) (domain.User, error) {
	var record userRecord
	if err := repo.db.WithContext(ctx).Where("account = ?", account).Take(&record).Error; err != nil {
		return domain.User{}, mapError("get by account", err)
	}
	return record.toDomain(), nil
}

type userRecord struct {
	ID             uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement"`
	Account        string    `gorm:"column:account;type:varchar(64);not null;uniqueIndex:ux_users_account"`
	PasswordHash   string    `gorm:"column:pwd_hash;type:varchar(255);not null"`
	Nickname       string    `gorm:"column:nickname;type:varchar(32);not null"`
	Abstract       string    `gorm:"column:abstract;type:varchar(128);not null;default:''"`
	Avatar         string    `gorm:"column:avatar;type:varchar(256);not null;default:''"`
	IP             string    `gorm:"column:ip;type:varchar(45);not null;default:''"`
	Address        string    `gorm:"column:addr;type:varchar(128);not null;default:''"`
	OpenID         *string   `gorm:"column:open_id;type:varchar(128);uniqueIndex:ux_users_open_id"`
	RegisterSource string    `gorm:"column:register_source;type:varchar(32);not null;default:account"`
	Role           int32     `gorm:"column:role;type:tinyint unsigned;not null;default:2"`
	Status         int32     `gorm:"column:status;type:tinyint unsigned;not null;default:1"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime(3);not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:datetime(3);not null"`
}

func (userRecord) TableName() string { return "users" }

func recordFromDomain(user domain.User) userRecord {
	return userRecord{
		ID:             user.ID,
		Account:        user.Account,
		PasswordHash:   user.PasswordHash,
		Nickname:       user.Nickname,
		Abstract:       user.Abstract,
		Avatar:         user.Avatar,
		IP:             user.IP,
		Address:        user.Address,
		OpenID:         optionalString(user.OpenID),
		RegisterSource: user.RegisterSource,
		Role:           int32(user.Role),
		Status:         int32(user.Status),
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}
}

func (record userRecord) toDomain() domain.User {
	return domain.User{
		ID:             record.ID,
		Account:        record.Account,
		PasswordHash:   record.PasswordHash,
		Nickname:       record.Nickname,
		Abstract:       record.Abstract,
		Avatar:         record.Avatar,
		IP:             record.IP,
		Address:        record.Address,
		OpenID:         valueOrEmpty(record.OpenID),
		RegisterSource: record.RegisterSource,
		Role:           domain.Role(record.Role),
		Status:         domain.Status(record.Status),
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func mapError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return repository.ErrDuplicateAccount
	}
	return fmt.Errorf("user repository %s: %w", operation, err)
}
