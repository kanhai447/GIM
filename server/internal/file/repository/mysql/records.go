package mysql

import (
	"time"

	"github.com/kanhai447/GIM/server/internal/file/domain"
)

type objectRecord struct {
	ID          uint64    `gorm:"column:id;primaryKey"`
	UID         string    `gorm:"column:uid"`
	SHA256      string    `gorm:"column:sha256"`
	Size        uint64    `gorm:"column:size"`
	StoragePath string    `gorm:"column:storage_path"`
	MIMEType    string    `gorm:"column:mime_type"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (objectRecord) TableName() string { return "file_objects" }

func (record objectRecord) domain() domain.FileObject {
	return domain.FileObject{ID: record.ID, UID: record.UID, SHA256: record.SHA256, Size: int64(record.Size), StoragePath: record.StoragePath, MIMEType: record.MIMEType, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

type userFileRecord struct {
	ID           uint64    `gorm:"column:id;primaryKey"`
	UserID       uint64    `gorm:"column:user_id"`
	FileObjectID uint64    `gorm:"column:file_object_id"`
	OriginalName string    `gorm:"column:original_name"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (userFileRecord) TableName() string { return "user_files" }

func (record userFileRecord) domain() domain.UserFile {
	return domain.UserFile{ID: record.ID, UserID: record.UserID, FileObjectID: record.FileObjectID, OriginalName: record.OriginalName, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
