package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/file/domain"
	filerepo "github.com/kanhai447/GIM/server/internal/file/repository"
	fileservice "github.com/kanhai447/GIM/server/internal/file/service"
	filestorage "github.com/kanhai447/GIM/server/internal/file/storage"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

type captureFileAuditor struct{ events []audit.Event }

func (publisher *captureFileAuditor) Publish(_ context.Context, event audit.Event) error {
	publisher.events = append(publisher.events, event)
	return nil
}

func TestUploadListDetailDownloadDeleteAndOwnership(t *testing.T) {
	repository := newHTTPRepository()
	local, err := filestorage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := fileservice.New(repository, local, fileservice.Config{MaxUploadBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	auditor := &captureFileAuditor{}
	handler := &Handler{files: service, maxBytes: 1024, auditor: auditor}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", `../报告 "safe".txt`)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("hello file"))
	_ = writer.Close()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/file/file", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("User-ID", "11")
	request.Header.Set("Role", "2")
	recorder := httptest.NewRecorder()
	handler.UploadFile(recorder, request)
	if recorder.Code != stdhttp.StatusOK || strings.Contains(recorder.Body.String(), "storage_path") || strings.Contains(recorder.Body.String(), "objects/") {
		t.Fatalf("upload status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Code uint32 `json:"code"`
		Data struct {
			ID   uint64 `json:"id"`
			URL  string `json:"url"`
			MIME string `json:"mimeType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 || response.Data.ID == 0 || response.Data.URL == "" || response.Data.MIME != "text/plain; charset=utf-8" {
		t.Fatalf("upload response=%+v", response)
	}
	listRequest := trustedRequest(stdhttp.MethodGet, "/api/file/user-files", 11)
	listRecorder := httptest.NewRecorder()
	handler.List(listRecorder, listRequest)
	if !strings.Contains(listRecorder.Body.String(), "报告") {
		t.Fatalf("list=%s", listRecorder.Body.String())
	}
	detailRequest := pathvar.WithVars(trustedRequest(stdhttp.MethodGet, "/api/file/user-files/1", 11), map[string]string{"id": fmt.Sprint(response.Data.ID)})
	detailRecorder := httptest.NewRecorder()
	handler.Detail(detailRecorder, detailRequest)
	if detailRecorder.Code != 200 {
		t.Fatalf("detail=%s", detailRecorder.Body.String())
	}
	foreignRequest := pathvar.WithVars(trustedRequest(stdhttp.MethodGet, "/api/file/1", 22), map[string]string{"id": fmt.Sprint(response.Data.ID)})
	foreignRecorder := httptest.NewRecorder()
	handler.Download(foreignRecorder, foreignRequest)
	if !strings.Contains(foreignRecorder.Body.String(), `"code":2503`) {
		t.Fatalf("foreign download=%s", foreignRecorder.Body.String())
	}
	downloadRequest := pathvar.WithVars(trustedRequest(stdhttp.MethodGet, "/api/file/1", 11), map[string]string{"id": fmt.Sprint(response.Data.ID)})
	downloadRecorder := httptest.NewRecorder()
	handler.Download(downloadRecorder, downloadRequest)
	if downloadRecorder.Body.String() != "hello file" || !strings.HasPrefix(downloadRecorder.Header().Get("Content-Disposition"), "attachment") || downloadRecorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download headers=%v body=%q", downloadRecorder.Header(), downloadRecorder.Body.String())
	}
	deleteRequest := pathvar.WithVars(trustedRequest(stdhttp.MethodDelete, "/api/file/user-files/1", 11), map[string]string{"id": fmt.Sprint(response.Data.ID)})
	deleteRecorder := httptest.NewRecorder()
	handler.Delete(deleteRecorder, deleteRequest)
	if !strings.Contains(deleteRecorder.Body.String(), `"deleted":true`) {
		t.Fatalf("delete=%s", deleteRecorder.Body.String())
	}
	if len(auditor.events) != 2 || auditor.events[0].EventType != audit.EventFileUploaded || auditor.events[1].EventType != audit.EventFileDeleted {
		t.Fatalf("audit events = %#v", auditor.events)
	}
	payload, _ := json.Marshal(auditor.events)
	for _, forbidden := range []string{"hello file", "objects/", "storage_path"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("audit leaked %q: %s", forbidden, payload)
		}
	}
}

func TestUploadRejectsOversizedAndMissingTrustedIdentity(t *testing.T) {
	repository := newHTTPRepository()
	local, _ := filestorage.NewLocal(t.TempDir())
	service, _ := fileservice.New(repository, local, fileservice.Config{MaxUploadBytes: 4})
	handler := &Handler{files: service, maxBytes: 4}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "large.bin")
	_, _ = part.Write([]byte("12345"))
	_ = writer.Close()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/file/file", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("User-ID", "999")
	request.Header.Set("Role", "1")
	request.ContentLength = int64(body.Len())
	recorder := httptest.NewRecorder()
	handler.UploadFile(recorder, request)
	if !strings.Contains(recorder.Body.String(), `"code":2502`) {
		t.Fatalf("oversize=%s", recorder.Body.String())
	}
	request = httptest.NewRequest(stdhttp.MethodPost, "/api/file/file", bytes.NewReader(nil))
	recorder = httptest.NewRecorder()
	handler.UploadFile(recorder, request)
	if !strings.Contains(recorder.Body.String(), `"code":2501`) {
		t.Fatalf("identity=%s", recorder.Body.String())
	}
}

func trustedRequest(method, url string, userID uint64) *stdhttp.Request {
	request := httptest.NewRequest(method, url, nil)
	request.Header.Set("User-ID", fmt.Sprint(userID))
	request.Header.Set("Role", "2")
	return request
}

type httpRepository struct {
	mutex                sync.Mutex
	objects              map[string]domain.FileObject
	files                map[uint64]domain.UserFile
	nextObject, nextFile uint64
}

func newHTTPRepository() *httpRepository {
	return &httpRepository{objects: map[string]domain.FileObject{}, files: map[uint64]domain.UserFile{}}
}
func (repo *httpRepository) GetBySHA256(_ context.Context, hash string) (domain.FileObject, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	value, ok := repo.objects[hash]
	if !ok {
		return domain.FileObject{}, filerepo.ErrNotFound
	}
	return value, nil
}
func (repo *httpRepository) GetByID(_ context.Context, id uint64) (domain.FileObject, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	for _, value := range repo.objects {
		if value.ID == id {
			return value, nil
		}
	}
	return domain.FileObject{}, filerepo.ErrNotFound
}
func (repo *httpRepository) GetByUID(_ context.Context, uid string) (domain.FileObject, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	for _, value := range repo.objects {
		if value.UID == uid {
			return value, nil
		}
	}
	return domain.FileObject{}, filerepo.ErrNotFound
}
func (repo *httpRepository) Create(_ context.Context, value *domain.FileObject) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	repo.nextObject++
	value.ID = repo.nextObject
	repo.objects[value.SHA256] = *value
	return nil
}
func (repo *httpRepository) CountReferences(_ context.Context, id uint64) (uint64, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	var count uint64
	for _, value := range repo.files {
		if value.FileObjectID == id {
			count++
		}
	}
	return count, nil
}
func (repo *httpRepository) CreateUserFile(_ context.Context, value *domain.UserFile) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	repo.nextFile++
	value.ID = repo.nextFile
	repo.files[value.ID] = *value
	return nil
}
func (repo *httpRepository) Attach(_ context.Context, object domain.FileObject, file domain.UserFile) (domain.FileMetadata, bool, bool, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	stored, ok := repo.objects[object.SHA256]
	created := !ok
	if !ok {
		repo.nextObject++
		object.ID = repo.nextObject
		object.CreatedAt = time.Now()
		object.UpdatedAt = object.CreatedAt
		stored = object
		repo.objects[object.SHA256] = object
	}
	for _, value := range repo.files {
		if value.UserID == file.UserID && value.FileObjectID == stored.ID && value.OriginalName == file.OriginalName {
			return domain.FileMetadata{Object: stored, UserFile: value}, created, false, nil
		}
	}
	repo.nextFile++
	file.ID = repo.nextFile
	file.FileObjectID = stored.ID
	file.CreatedAt = time.Now()
	repo.files[file.ID] = file
	return domain.FileMetadata{Object: stored, UserFile: file}, created, true, nil
}
func (repo *httpRepository) GetOwnedByID(_ context.Context, userID, id uint64) (domain.FileMetadata, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	file, ok := repo.files[id]
	if !ok || file.UserID != userID {
		return domain.FileMetadata{}, filerepo.ErrNotFound
	}
	for _, object := range repo.objects {
		if object.ID == file.FileObjectID {
			return domain.FileMetadata{Object: object, UserFile: file}, nil
		}
	}
	return domain.FileMetadata{}, filerepo.ErrNotFound
}
func (repo *httpRepository) GetOwnedByUID(_ context.Context, userID uint64, uid string) (domain.FileMetadata, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	for _, object := range repo.objects {
		if object.UID == uid {
			for _, file := range repo.files {
				if file.UserID == userID && file.FileObjectID == object.ID {
					return domain.FileMetadata{Object: object, UserFile: file}, nil
				}
			}
		}
	}
	return domain.FileMetadata{}, filerepo.ErrNotFound
}
func (repo *httpRepository) ListByUser(_ context.Context, userID, before uint64, limit int) ([]domain.FileMetadata, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	result := []domain.FileMetadata{}
	for id, file := range repo.files {
		if file.UserID == userID && (before == 0 || id < before) {
			for _, object := range repo.objects {
				if object.ID == file.FileObjectID {
					result = append(result, domain.FileMetadata{Object: object, UserFile: file})
				}
			}
		}
	}
	return result, nil
}
func (repo *httpRepository) DeleteReference(_ context.Context, userID, id uint64) error {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	file, ok := repo.files[id]
	if !ok || file.UserID != userID {
		return filerepo.ErrNotFound
	}
	delete(repo.files, id)
	return nil
}
func (repo *httpRepository) Exists(_ context.Context, userID, id uint64) (bool, error) {
	repo.mutex.Lock()
	defer repo.mutex.Unlock()
	file, ok := repo.files[id]
	return ok && file.UserID == userID, nil
}
