package mysql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	fileadmin "github.com/kanhai447/GIM/server/internal/file/admin"
	"github.com/kanhai447/GIM/server/internal/file/domain"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type row struct {
	ID           uint64    `gorm:"column:id"`
	UserID       uint64    `gorm:"column:user_id"`
	UserNickname string    `gorm:"column:user_nickname"`
	ObjectID     uint64    `gorm:"column:object_id"`
	UID          string    `gorm:"column:uid"`
	FileName     string    `gorm:"column:file_name"`
	Size         uint64    `gorm:"column:size"`
	MIMEType     string    `gorm:"column:mime_type"`
	SHA256       string    `gorm:"column:sha256"`
	StoragePath  string    `gorm:"column:storage_path"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func base(db *gorm.DB) *gorm.DB {
	return db.Table("user_files AS files").Select(`files.id, files.user_id, users.nickname AS user_nickname, objects.id AS object_id,
	objects.uid, files.original_name AS file_name, objects.size, objects.mime_type, objects.sha256, objects.storage_path, files.created_at, objects.updated_at`).
		Joins("JOIN file_objects AS objects ON objects.id = files.file_object_id").Joins("JOIN users ON users.id = files.user_id")
}

func (repository *Repository) List(ctx context.Context, query fileadmin.Query) ([]fileadmin.File, error) {
	database := base(repository.db.WithContext(ctx))
	if query.BeforeID != 0 {
		database = database.Where("files.id < ?", query.BeforeID)
	}
	if query.UserID != 0 {
		database = database.Where("files.user_id = ?", query.UserID)
	}
	if query.Search != "" {
		database = database.Where("files.original_name LIKE ? ESCAPE '\\\\'", "%"+escapeLike(query.Search)+"%")
	}
	var rows []row
	if err := database.Order("files.id DESC").Limit(query.Limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list admin files: %w", err)
	}
	items := make([]fileadmin.File, 0, len(rows))
	for _, value := range rows {
		items = append(items, toFile(value))
	}
	return items, nil
}
func (repository *Repository) Get(ctx context.Context, id uint64) (fileadmin.File, error) {
	var value row
	err := base(repository.db.WithContext(ctx)).Where("files.id = ?", id).Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fileadmin.File{}, fileadmin.ErrNotFound
	}
	if err != nil {
		return fileadmin.File{}, fmt.Errorf("get admin file: %w", err)
	}
	return toFile(value), nil
}
func (repository *Repository) Delete(ctx context.Context, id uint64) error {
	result := repository.db.WithContext(ctx).Exec("DELETE FROM user_files WHERE id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete admin file reference: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fileadmin.ErrNotFound
	}
	return nil
}
func toFile(value row) fileadmin.File {
	metadata := domain.FileMetadata{UserFile: domain.UserFile{ID: value.ID, UserID: value.UserID, FileObjectID: value.ObjectID, OriginalName: value.FileName, CreatedAt: value.CreatedAt}, Object: domain.FileObject{ID: value.ObjectID, UID: value.UID, SHA256: value.SHA256, Size: int64(value.Size), StoragePath: value.StoragePath, MIMEType: value.MIMEType, UpdatedAt: value.UpdatedAt}}
	return fileadmin.File{ID: value.ID, UserID: value.UserID, UserNickname: value.UserNickname, UID: value.UID, FileName: value.FileName, Size: int64(value.Size), MIMEType: value.MIMEType, SHA256: value.SHA256, DownloadURL: fmt.Sprintf("/api/file/files/%d", value.ID), CreatedAt: value.CreatedAt, Metadata: metadata}
}
func escapeLike(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}
