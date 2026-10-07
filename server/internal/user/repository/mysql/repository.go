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
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Account      string    `gorm:"column:account;type:varchar(64);not null;uniqueIndex:ux_users_account"`
	PasswordHash string    `gorm:"column:pwd_hash;type:varchar(255);not null"`
	Nickname     string    `gorm:"column:nickname;type:varchar(32);not null"`
	Abstract     string    `gorm:"column:abstract;type:varchar(128);not null;default:''"`
	Avatar       string    `gorm:"column:avatar;type:varchar(256);not null;default:''"`
	Role         int32     `gorm:"column:role;type:tinyint;not null"`
	Status       int32     `gorm:"column:status;type:tinyint;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (userRecord) TableName() string { return "users" }

func recordFromDomain(user domain.User) userRecord {
	return userRecord{
		ID:           user.ID,
		Account:      user.Account,
		PasswordHash: user.PasswordHash,
		Nickname:     user.Nickname,
		Abstract:     user.Abstract,
		Avatar:       user.Avatar,
		Role:         int32(user.Role),
		Status:       int32(user.Status),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func (record userRecord) toDomain() domain.User {
	return domain.User{
		ID:           record.ID,
		Account:      record.Account,
		PasswordHash: record.PasswordHash,
		Nickname:     record.Nickname,
		Abstract:     record.Abstract,
		Avatar:       record.Avatar,
		Role:         domain.Role(record.Role),
		Status:       domain.Status(record.Status),
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
	}
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
