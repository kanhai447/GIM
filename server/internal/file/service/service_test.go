package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/kanhai447/GIM/server/internal/file/domain"
	filerepo "github.com/kanhai447/GIM/server/internal/file/repository"
	filestorage "github.com/kanhai447/GIM/server/internal/file/storage"
)

func TestSanitizeFilenameAndDisposition(t *testing.T) {
	name, err := SanitizeFilename(`../../目录/report.txt`)
	if err != nil || name != "report.txt" {
		t.Fatalf("sanitize=%q err=%v", name, err)
	}
	for _, bad := range []string{"", "/", "bad\r\nname.txt", string([]byte{0xff})} {
		if _, err := SanitizeFilename(bad); err == nil {
			t.Fatalf("SanitizeFilename(%q) accepted", bad)
		}
	}
	if got := ContentDisposition("报告 2026.txt"); got == "" || got == "attachment" {
		t.Fatalf("ContentDisposition=%q", got)
	}
}

func TestUploadConcurrentDedupAndOwnershipReferences(t *testing.T) {
	repository := newMemoryRepository()
	objects := newMemoryStorage()
	service, _ := New(repository, objects, Config{MaxUploadBytes: 1 << 20})
	payload := []byte("same payload")
	const workers = 50
	results := make(chan domain.UploadResult, workers)
	failures := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := service.Upload(context.Background(), 7, "same.txt", bytes.NewReader(payload))
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatalf("concurrent Upload error=%v", err)
	}
	var firstID uint64
	for result := range results {
		if firstID == 0 {
			firstID = result.Metadata.UserFile.ID
		}
		if result.Metadata.UserFile.ID != firstID {
			t.Fatalf("same logical upload returned ids %d/%d", firstID, result.Metadata.UserFile.ID)
		}
	}
	if repository.objectCount() != 1 || repository.referenceCount() != 1 || objects.commitCount != 1 {
		t.Fatalf("objects=%d refs=%d commits=%d", repository.objectCount(), repository.referenceCount(), objects.commitCount)
	}
	if _, err := service.Upload(context.Background(), 7, "renamed.txt", bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Upload(context.Background(), 8, "same.txt", bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	if repository.objectCount() != 1 || repository.referenceCount() != 3 || objects.commitCount != 1 {
		t.Fatalf("objects=%d refs=%d commits=%d", repository.objectCount(), repository.referenceCount(), objects.commitCount)
	}
}

func TestUploadLimitsCleanupAndCompensation(t *testing.T) {
	repository := newMemoryRepository()
	objects := newMemoryStorage()
	service, _ := New(repository, objects, Config{MaxUploadBytes: 4})
	if _, err := service.Upload(context.Background(), 1, "large.bin", bytes.NewReader([]byte("12345"))); err == nil {
		t.Fatal("oversized upload succeeded")
	}
	if objects.abortCount != 1 || objects.commitCount != 0 {
		t.Fatalf("abort=%d commit=%d", objects.abortCount, objects.commitCount)
	}
	repository.failAttach = true
	if _, err := service.Upload(context.Background(), 1, "small.bin", bytes.NewReader([]byte("1234"))); err == nil {
		t.Fatal("injected attach failure succeeded")
	}
	if objects.deleteCount != 1 || len(objects.objects) != 0 {
		t.Fatalf("delete=%d objects=%d", objects.deleteCount, len(objects.objects))
	}
	objects.failWrite = true
	if _, err := service.Upload(context.Background(), 1, "write.bin", bytes.NewReader([]byte("12"))); err == nil {
		t.Fatal("injected temp write failure succeeded")
	}
	objects.failWrite = false
	objects.failCommit = true
	if _, err := service.Upload(context.Background(), 1, "move.bin", bytes.NewReader([]byte("34"))); err == nil {
		t.Fatal("injected final move failure succeeded")
	}
}

func TestExistingSharedObjectSurvivesReferenceFailure(t *testing.T) {
	repository := newMemoryRepository()
	objects := newMemoryStorage()
	service, _ := New(repository, objects, Config{MaxUploadBytes: 64})
	if _, err := service.Upload(context.Background(), 1, "shared.bin", bytes.NewReader([]byte("shared"))); err != nil {
		t.Fatal(err)
	}
	repository.failAttach = true
	if _, err := service.Upload(context.Background(), 2, "shared.bin", bytes.NewReader([]byte("shared"))); err == nil {
		t.Fatal("injected reference failure succeeded")
	}
	if objects.deleteCount != 0 || len(objects.objects) != 1 {
		t.Fatalf("delete=%d objects=%d", objects.deleteCount, len(objects.objects))
	}
}

type memoryTemp struct {
	bytes.Reader
	buffer bytes.Buffer
	owner  *memoryStorage
}

func newMemoryTemp(owner *memoryStorage) *memoryTemp { return &memoryTemp{owner: owner} }
func (temp *memoryTemp) Write(p []byte) (int, error) {
	if temp.owner.failWrite {
		return 0, errors.New("injected temp write failure")
	}
	return temp.buffer.Write(p)
}
func (temp *memoryTemp) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (temp *memoryTemp) Close() error                                 { return nil }

type memoryStorage struct {
	mutex                                sync.Mutex
	objects                              map[string][]byte
	commitCount, abortCount, deleteCount int
	failWrite, failCommit                bool
}

func newMemoryStorage() *memoryStorage { return &memoryStorage{objects: map[string][]byte{}} }
func (storage *memoryStorage) CreateTemp(context.Context) (filestorage.TempObject, error) {
	return newMemoryTemp(storage), nil
}
func (storage *memoryStorage) Commit(_ context.Context, object filestorage.TempObject, path string) (bool, error) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	if storage.failCommit {
		return false, errors.New("injected final move failure")
	}
	temp := object.(*memoryTemp)
	if _, ok := storage.objects[path]; ok {
		return false, nil
	}
	storage.objects[path] = bytes.Clone(temp.buffer.Bytes())
	storage.commitCount++
	return true, nil
}
func (storage *memoryStorage) Abort(filestorage.TempObject) error {
	storage.mutex.Lock()
	storage.abortCount++
	storage.mutex.Unlock()
	return nil
}
func (storage *memoryStorage) Open(_ context.Context, path string) (filestorage.Object, error) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	payload, ok := storage.objects[path]
	if !ok {
		return nil, errors.New("missing")
	}
	return &memoryObject{Reader: *bytes.NewReader(bytes.Clone(payload))}, nil
}
func (storage *memoryStorage) Delete(_ context.Context, path string) error {
	storage.mutex.Lock()
	delete(storage.objects, path)
	storage.deleteCount++
	storage.mutex.Unlock()
	return nil
}
func (storage *memoryStorage) Exists(_ context.Context, path string) (bool, error) {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	_, ok := storage.objects[path]
	return ok, nil
}

type memoryObject struct{ bytes.Reader }

func (object *memoryObject) Close() error { return nil }

type memoryRepository struct {
	mutex                sync.Mutex
	objects              map[string]domain.FileObject
	files                map[uint64]domain.UserFile
	nextObject, nextFile uint64
	failAttach           bool
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{objects: map[string]domain.FileObject{}, files: map[uint64]domain.UserFile{}}
}
func (repo *memoryRepository) GetBySHA256(_ context.Context, hash string) (domain.FileObject, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	object, ok := repo.objects[hash]
	if !ok {
		return domain.FileObject{}, filerepo.ErrNotFound
	}
	return object, nil
}
func (repo *memoryRepository) GetByID(_ context.Context, id uint64) (domain.FileObject, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	for _, o := range repo.objects {
		if o.ID == id {
			return o, nil
		}
	}
	return domain.FileObject{}, filerepo.ErrNotFound
}
func (repo *memoryRepository) GetByUID(_ context.Context, uid string) (domain.FileObject, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	for _, o := range repo.objects {
		if o.UID == uid {
			return o, nil
		}
	}
	return domain.FileObject{}, filerepo.ErrNotFound
}
func (repo *memoryRepository) Create(_ context.Context, object *domain.FileObject) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	repo.nextObject++
	object.ID = repo.nextObject
	repo.objects[object.SHA256] = *object
	return nil
}
func (repo *memoryRepository) CountReferences(_ context.Context, id uint64) (uint64, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	var count uint64
	for _, f := range repo.files {
		if f.FileObjectID == id {
			count++
		}
	}
	return count, nil
}
func (repo *memoryRepository) CreateUserFile(_ context.Context, file *domain.UserFile) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	repo.nextFile++
	file.ID = repo.nextFile
	repo.files[file.ID] = *file
	return nil
}
func (repo *memoryRepository) Attach(_ context.Context, object domain.FileObject, file domain.UserFile) (domain.FileMetadata, bool, bool, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	if repo.failAttach {
		return domain.FileMetadata{}, false, false, errors.New("injected attach failure")
	}
	stored, ok := repo.objects[object.SHA256]
	createdObject := !ok
	if !ok {
		repo.nextObject++
		object.ID = repo.nextObject
		stored = object
		repo.objects[object.SHA256] = stored
	}
	for _, current := range repo.files {
		if current.UserID == file.UserID && current.FileObjectID == stored.ID && current.OriginalName == file.OriginalName {
			return domain.FileMetadata{Object: stored, UserFile: current}, createdObject, false, nil
		}
	}
	repo.nextFile++
	file.ID = repo.nextFile
	file.FileObjectID = stored.ID
	repo.files[file.ID] = file
	return domain.FileMetadata{Object: stored, UserFile: file}, createdObject, true, nil
}
func (repo *memoryRepository) GetOwnedByID(_ context.Context, userID, id uint64) (domain.FileMetadata, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	file, ok := repo.files[id]
	if !ok || file.UserID != userID {
		return domain.FileMetadata{}, filerepo.ErrNotFound
	}
	for _, o := range repo.objects {
		if o.ID == file.FileObjectID {
			return domain.FileMetadata{Object: o, UserFile: file}, nil
		}
	}
	return domain.FileMetadata{}, filerepo.ErrNotFound
}
func (repo *memoryRepository) GetOwnedByUID(_ context.Context, userID uint64, uid string) (domain.FileMetadata, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	var object domain.FileObject
	for _, o := range repo.objects {
		if o.UID == uid {
			object = o
		}
	}
	if object.ID == 0 {
		return domain.FileMetadata{}, filerepo.ErrNotFound
	}
	for _, f := range repo.files {
		if f.UserID == userID && f.FileObjectID == object.ID {
			return domain.FileMetadata{Object: object, UserFile: f}, nil
		}
	}
	return domain.FileMetadata{}, filerepo.ErrNotFound
}
func (repo *memoryRepository) ListByUser(_ context.Context, userID, before uint64, limit int) ([]domain.FileMetadata, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	var result []domain.FileMetadata
	for id, f := range repo.files {
		if f.UserID == userID && (before == 0 || id < before) {
			for _, o := range repo.objects {
				if o.ID == f.FileObjectID {
					result = append(result, domain.FileMetadata{Object: o, UserFile: f})
				}
			}
		}
	}
	return result, nil
}
func (repo *memoryRepository) DeleteReference(_ context.Context, userID, id uint64) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	file, ok := repo.files[id]
	if !ok || file.UserID != userID {
		return filerepo.ErrNotFound
	}
	delete(repo.files, id)
	return nil
}
func (repo *memoryRepository) Exists(_ context.Context, userID, id uint64) (bool, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	f, ok := repo.files[id]
	return ok && f.UserID == userID, nil
}
func (repo *memoryRepository) objectCount() int {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	return len(repo.objects)
}
func (repo *memoryRepository) referenceCount() int {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	return len(repo.files)
}

var _ io.ReadWriteSeeker = (*memoryTemp)(nil)
