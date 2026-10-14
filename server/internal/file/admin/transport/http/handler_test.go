package http

import (
	"context"
	fileadmin "github.com/kanhai447/GIM/server/internal/file/admin"
	"github.com/kanhai447/GIM/server/internal/file/domain"
	"github.com/kanhai447/GIM/server/internal/file/storage"
	"log"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

type fakeApplication struct{}

func (fakeApplication) List(context.Context, fileadmin.Query) (fileadmin.Page, error) {
	return fileadmin.Page{}, nil
}
func (fakeApplication) Get(context.Context, uint64) (fileadmin.File, error) {
	return fileadmin.File{}, nil
}
func (fakeApplication) Delete(context.Context, uint64) error { return nil }

type fakeOpener struct{}

func (fakeOpener) Open(context.Context, domain.FileMetadata) (storage.Object, error) { return nil, nil }
func TestFileAdminAuthorization(t *testing.T) {
	h := &Handler{application: fakeApplication{}, opener: fakeOpener{}, logger: log.Default()}
	r := httptest.NewRequest(stdhttp.MethodGet, "/api/file/files", nil)
	r.Header.Set("User-ID", "7")
	r.Header.Set("Role", "2")
	w := httptest.NewRecorder()
	h.List(w, r)
	if w.Code != stdhttp.StatusForbidden {
		t.Fatalf("member status = %d", w.Code)
	}
}
