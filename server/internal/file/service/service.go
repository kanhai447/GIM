// Package service orchestrates streaming uploads, deduplication, ownership, and compensation.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/file/domain"
	filerepo "github.com/kanhai447/GIM/server/internal/file/repository"
	"github.com/kanhai447/GIM/server/internal/file/storage"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

const (
	CodeInvalid     uint32 = 2501
	CodeTooLarge    uint32 = 2502
	CodeNotFound    uint32 = 2503
	CodeUnavailable uint32 = 2504
)

var ErrTooLarge = errors.New("file upload exceeds configured limit")

type Config struct{ MaxUploadBytes int64 }

type Service struct {
	repository filerepo.Repository
	storage    storage.Storage
	maxBytes   int64
	locks      hashLocks
}

func New(repository filerepo.Repository, storage storage.Storage, config Config) (*Service, error) {
	if repository == nil || storage == nil || config.MaxUploadBytes <= 0 {
		return nil, errors.New("invalid file service dependencies")
	}
	return &Service{repository: repository, storage: storage, maxBytes: config.MaxUploadBytes, locks: hashLocks{items: make(map[string]*hashLock)}}, nil
}

func (service *Service) Upload(ctx context.Context, userID uint64, originalName string, reader io.Reader) (domain.UploadResult, error) {
	if userID == 0 || reader == nil {
		return domain.UploadResult{}, invalid("上传参数无效")
	}
	name, err := SanitizeFilename(originalName)
	if err != nil {
		return domain.UploadResult{}, invalid("文件名无效")
	}
	temp, err := service.storage.CreateTemp(ctx)
	if err != nil {
		return domain.UploadResult{}, unavailable(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = service.storage.Abort(temp)
		}
	}()
	hasher := sha256.New()
	prefix := &prefixWriter{limit: 512}
	count, copyErr := io.CopyBuffer(io.MultiWriter(temp, hasher, prefix), &contextReader{ctx: ctx, reader: io.LimitReader(reader, service.maxBytes+1)}, make([]byte, 32*1024))
	if copyErr != nil {
		return domain.UploadResult{}, unavailable(copyErr)
	}
	if count > service.maxBytes {
		return domain.UploadResult{}, apperror.Business(CodeTooLarge, "文件超过大小限制")
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	mimeType := http.DetectContentType(prefix.bytes)
	objectPath := path.Join("objects", hash[:2], hash[2:4], hash)
	unlock := service.locks.lock(hash)
	defer unlock()
	existing, lookupErr := service.repository.GetBySHA256(ctx, hash)
	physicalCreated := false
	if lookupErr == nil {
		if existing.Size != count || existing.StoragePath != objectPath {
			return domain.UploadResult{}, unavailable(errors.New("stored content metadata conflict"))
		}
		exists, existsErr := service.storage.Exists(ctx, objectPath)
		if existsErr != nil {
			return domain.UploadResult{}, unavailable(existsErr)
		}
		if exists {
			if err := service.storage.Abort(temp); err != nil {
				return domain.UploadResult{}, unavailable(err)
			}
		} else {
			physicalCreated, err = service.storage.Commit(ctx, temp, objectPath)
			if err != nil {
				return domain.UploadResult{}, unavailable(err)
			}
		}
		committed = true
	} else if !errors.Is(lookupErr, filerepo.ErrNotFound) {
		return domain.UploadResult{}, unavailable(lookupErr)
	} else {
		physicalCreated, err = service.storage.Commit(ctx, temp, objectPath)
		if err != nil {
			return domain.UploadResult{}, unavailable(err)
		}
		committed = true
	}
	uid, err := newUUID()
	if err != nil {
		if physicalCreated {
			_ = service.storage.Delete(context.WithoutCancel(ctx), objectPath)
		}
		return domain.UploadResult{}, unavailable(err)
	}
	object := domain.FileObject{UID: uid, SHA256: hash, Size: count, StoragePath: objectPath, MIMEType: mimeType}
	metadata, objectCreated, referenceCreated, err := service.repository.Attach(ctx, object, domain.UserFile{UserID: userID, OriginalName: name})
	if err != nil {
		if physicalCreated {
			if _, checkErr := service.repository.GetBySHA256(ctx, hash); errors.Is(checkErr, filerepo.ErrNotFound) {
				_ = service.storage.Delete(context.WithoutCancel(ctx), objectPath)
			}
		}
		return domain.UploadResult{}, unavailable(err)
	}
	return domain.UploadResult{Metadata: metadata, ObjectCreated: objectCreated, ReferenceNew: referenceCreated}, nil
}

func (service *Service) Get(ctx context.Context, userID uint64, identifier string) (domain.FileMetadata, error) {
	if userID == 0 {
		return domain.FileMetadata{}, invalid("身份信息无效")
	}
	identifier = strings.TrimSpace(identifier)
	if id, err := strconv.ParseUint(identifier, 10, 64); err == nil && id != 0 {
		return service.mapOwned(service.repository.GetOwnedByID(ctx, userID, id))
	}
	if !validUUID(identifier) {
		return domain.FileMetadata{}, invalid("文件标识无效")
	}
	return service.mapOwned(service.repository.GetOwnedByUID(ctx, userID, identifier))
}

func (service *Service) List(ctx context.Context, userID, beforeID uint64, limit int) ([]domain.FileMetadata, error) {
	if userID == 0 || limit < 0 || limit > 100 {
		return nil, invalid("文件列表参数无效")
	}
	if limit == 0 {
		limit = 20
	}
	items, err := service.repository.ListByUser(ctx, userID, beforeID, limit)
	if err != nil {
		return nil, unavailable(err)
	}
	return items, nil
}

func (service *Service) Delete(ctx context.Context, userID, id uint64) error {
	if userID == 0 || id == 0 {
		return invalid("文件参数无效")
	}
	if err := service.repository.DeleteReference(ctx, userID, id); err != nil {
		if errors.Is(err, filerepo.ErrNotFound) {
			return apperror.Business(CodeNotFound, "文件不存在")
		}
		return unavailable(err)
	}
	return nil
}

func (service *Service) Open(ctx context.Context, metadata domain.FileMetadata) (storage.Object, error) {
	object, err := service.storage.Open(ctx, metadata.Object.StoragePath)
	if err != nil {
		return nil, apperror.Business(CodeNotFound, "文件不存在")
	}
	return object, nil
}

func (service *Service) mapOwned(metadata domain.FileMetadata, err error) (domain.FileMetadata, error) {
	if errors.Is(err, filerepo.ErrNotFound) {
		return domain.FileMetadata{}, apperror.Business(CodeNotFound, "文件不存在")
	}
	if err != nil {
		return domain.FileMetadata{}, unavailable(err)
	}
	return metadata, nil
}

func SanitizeFilename(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = path.Base(value)
	if value == "." || value == "" || value == ".." || value == "/" || !utf8.ValidString(value) {
		return "", errors.New("invalid filename")
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", errors.New("filename contains control characters")
		}
	}
	if len([]byte(value)) > 255 {
		return "", errors.New("filename is too long")
	}
	return value, nil
}

func ContentDisposition(filename string) string {
	value := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	if value == "" {
		return "attachment"
	}
	return value
}

func invalid(message string) error { return apperror.Business(CodeInvalid, message) }
func unavailable(cause error) error {
	return apperror.Wrap(cause, http.StatusInternalServerError, CodeUnavailable, "文件服务暂不可用")
}

type prefixWriter struct {
	bytes []byte
	limit int
}

func (writer *prefixWriter) Write(payload []byte) (int, error) {
	if len(writer.bytes) < writer.limit {
		remaining := writer.limit - len(writer.bytes)
		if remaining > len(payload) {
			remaining = len(payload)
		}
		writer.bytes = append(writer.bytes, payload[:remaining]...)
	}
	return len(payload), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(payload []byte) (int, error) {
	select {
	case <-reader.ctx.Done():
		return 0, reader.ctx.Err()
	default:
		return reader.reader.Read(payload)
	}
}

type hashLock struct {
	mutex sync.Mutex
	refs  int
}
type hashLocks struct {
	mutex sync.Mutex
	items map[string]*hashLock
}

func (locks *hashLocks) lock(hash string) func() {
	locks.mutex.Lock()
	item := locks.items[hash]
	if item == nil {
		item = &hashLock{}
		locks.items[hash] = item
	}
	item.refs++
	locks.mutex.Unlock()
	item.mutex.Lock()
	return func() {
		item.mutex.Unlock()
		locks.mutex.Lock()
		item.refs--
		if item.refs == 0 {
			delete(locks.items, hash)
		}
		locks.mutex.Unlock()
	}
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(value[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[:8], raw[8:12], raw[12:16], raw[16:20], raw[20:]), nil
}
func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
