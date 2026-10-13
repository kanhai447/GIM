// Package mysql implements File repositories on the versioned 005_file schema.
package mysql

import (
	"context"
	"errors"
	"fmt"

	"github.com/kanhai447/GIM/server/internal/file/domain"
	filerepo "github.com/kanhai447/GIM/server/internal/file/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (repo *Repository) GetBySHA256(ctx context.Context, hash string) (domain.FileObject, error) {
	var record objectRecord
	if err := repo.db.WithContext(ctx).Where("sha256 = ?", hash).Take(&record).Error; err != nil {
		return domain.FileObject{}, mapError("get object by hash", err)
	}
	return record.domain(), nil
}

func (repo *Repository) GetByID(ctx context.Context, id uint64) (domain.FileObject, error) {
	var record objectRecord
	if err := repo.db.WithContext(ctx).Take(&record, id).Error; err != nil {
		return domain.FileObject{}, mapError("get object by id", err)
	}
	return record.domain(), nil
}

func (repo *Repository) GetByUID(ctx context.Context, uid string) (domain.FileObject, error) {
	var record objectRecord
	if err := repo.db.WithContext(ctx).Where("uid = ?", uid).Take(&record).Error; err != nil {
		return domain.FileObject{}, mapError("get object by uid", err)
	}
	return record.domain(), nil
}

func (repo *Repository) Create(ctx context.Context, object *domain.FileObject) error {
	record := objectRecord{UID: object.UID, SHA256: object.SHA256, Size: uint64(object.Size), StoragePath: object.StoragePath, MIMEType: object.MIMEType}
	if err := repo.db.WithContext(ctx).Create(&record).Error; err != nil {
		return mapError("create object", err)
	}
	*object = record.domain()
	return nil
}

func (repo *Repository) CountReferences(ctx context.Context, objectID uint64) (uint64, error) {
	var count int64
	if err := repo.db.WithContext(ctx).Model(&userFileRecord{}).Where("file_object_id = ?", objectID).Count(&count).Error; err != nil {
		return 0, mapError("count references", err)
	}
	return uint64(count), nil
}

func (repo *Repository) CreateUserFile(ctx context.Context, file *domain.UserFile) error {
	record := userFileRecord{UserID: file.UserID, FileObjectID: file.FileObjectID, OriginalName: file.OriginalName}
	if err := repo.db.WithContext(ctx).Create(&record).Error; err != nil {
		return mapError("create user file", err)
	}
	*file = record.domain()
	return nil
}

func (repo *Repository) Attach(ctx context.Context, object domain.FileObject, file domain.UserFile) (domain.FileMetadata, bool, bool, error) {
	var metadata domain.FileMetadata
	var objectCreated, referenceCreated bool
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		objectRow := objectRecord{UID: object.UID, SHA256: object.SHA256, Size: uint64(object.Size), StoragePath: object.StoragePath, MIMEType: object.MIMEType}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "sha256"}}, DoNothing: true}).Create(&objectRow)
		if result.Error != nil {
			return mapError("attach object", result.Error)
		}
		objectCreated = result.RowsAffected == 1
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sha256 = ?", object.SHA256).Take(&objectRow).Error; err != nil {
			return mapError("resolve attached object", err)
		}
		if objectRow.Size != uint64(object.Size) || objectRow.StoragePath != object.StoragePath {
			return errors.New("file repository: content identity conflict")
		}
		fileRow := userFileRecord{UserID: file.UserID, FileObjectID: objectRow.ID, OriginalName: file.OriginalName}
		result = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "file_object_id"}, {Name: "original_name"}}, DoNothing: true}).Create(&fileRow)
		if result.Error != nil {
			return mapError("attach user file", result.Error)
		}
		referenceCreated = result.RowsAffected == 1
		if err := tx.Where("user_id = ? AND file_object_id = ? AND original_name = ?", file.UserID, objectRow.ID, file.OriginalName).Take(&fileRow).Error; err != nil {
			return mapError("resolve attached user file", err)
		}
		metadata = domain.FileMetadata{Object: objectRow.domain(), UserFile: fileRow.domain()}
		return nil
	})
	return metadata, objectCreated, referenceCreated, err
}

func (repo *Repository) GetOwnedByID(ctx context.Context, userID, id uint64) (domain.FileMetadata, error) {
	return repo.getOwned(ctx, "uf.id = ?", userID, id)
}

func (repo *Repository) GetOwnedByUID(ctx context.Context, userID uint64, uid string) (domain.FileMetadata, error) {
	return repo.getOwned(ctx, "fo.uid = ?", userID, uid)
}

func (repo *Repository) getOwned(ctx context.Context, predicate string, userID uint64, value any) (domain.FileMetadata, error) {
	var uf userFileRecord
	var fo objectRecord
	err := repo.db.WithContext(ctx).Table("user_files AS uf").Select("uf.*").Joins("JOIN file_objects AS fo ON fo.id = uf.file_object_id").Where("uf.user_id = ?", userID).Where(predicate, value).Take(&uf).Error
	if err != nil {
		return domain.FileMetadata{}, mapError("get owned file", err)
	}
	if err := repo.db.WithContext(ctx).Take(&fo, uf.FileObjectID).Error; err != nil {
		return domain.FileMetadata{}, mapError("get owned object", err)
	}
	return domain.FileMetadata{UserFile: uf.domain(), Object: fo.domain()}, nil
}

func (repo *Repository) ListByUser(ctx context.Context, userID, beforeID uint64, limit int) ([]domain.FileMetadata, error) {
	query := repo.db.WithContext(ctx).Where("user_id = ?", userID)
	if beforeID != 0 {
		query = query.Where("id < ?", beforeID)
	}
	var files []userFileRecord
	if err := query.Order("id DESC").Limit(limit).Find(&files).Error; err != nil {
		return nil, mapError("list user files", err)
	}
	if len(files) == 0 {
		return []domain.FileMetadata{}, nil
	}
	ids := make([]uint64, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.FileObjectID)
	}
	var objects []objectRecord
	if err := repo.db.WithContext(ctx).Where("id IN ?", ids).Find(&objects).Error; err != nil {
		return nil, mapError("list file objects", err)
	}
	byID := make(map[uint64]domain.FileObject, len(objects))
	for _, o := range objects {
		byID[o.ID] = o.domain()
	}
	result := make([]domain.FileMetadata, 0, len(files))
	for _, f := range files {
		if o, ok := byID[f.FileObjectID]; ok {
			result = append(result, domain.FileMetadata{UserFile: f.domain(), Object: o})
		}
	}
	return result, nil
}

func (repo *Repository) DeleteReference(ctx context.Context, userID, id uint64) error {
	result := repo.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&userFileRecord{})
	if result.Error != nil {
		return mapError("delete reference", result.Error)
	}
	if result.RowsAffected != 1 {
		return filerepo.ErrNotFound
	}
	return nil
}

func (repo *Repository) Exists(ctx context.Context, userID, id uint64) (bool, error) {
	var count int64
	if err := repo.db.WithContext(ctx).Model(&userFileRecord{}).Where("id = ? AND user_id = ?", id, userID).Count(&count).Error; err != nil {
		return false, mapError("check reference", err)
	}
	return count == 1, nil
}

func mapError(operation string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filerepo.ErrNotFound
	}
	return fmt.Errorf("file repository %s: %w", operation, err)
}
