package http

import (
	"context"
	groupadmin "github.com/kanhai447/GIM/server/internal/group/admin"
	"log"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

type fakeApplication struct{}

func (fakeApplication) List(context.Context, groupadmin.Query) (groupadmin.Page, error) {
	return groupadmin.Page{}, nil
}
func (fakeApplication) ListMessages(context.Context, groupadmin.MessageQuery) (groupadmin.MessagePage, error) {
	return groupadmin.MessagePage{}, nil
}
func (fakeApplication) Dissolve(context.Context, uint64) error      { return nil }
func (fakeApplication) DeleteMessage(context.Context, uint64) error { return nil }
func TestGroupAdminAuthorization(t *testing.T) {
	h := &Handler{application: fakeApplication{}, logger: log.Default()}
	r := httptest.NewRequest(stdhttp.MethodGet, "/api/group/groups", nil)
	r.Header.Set("User-ID", "7")
	r.Header.Set("Role", "2")
	w := httptest.NewRecorder()
	h.List(w, r)
	if w.Code != stdhttp.StatusForbidden {
		t.Fatalf("member status = %d", w.Code)
	}
	admin := httptest.NewRequest(stdhttp.MethodGet, "/api/group/groups", nil)
	admin.Header.Set("User-ID", "7")
	admin.Header.Set("Role", "1")
	w = httptest.NewRecorder()
	h.List(w, admin)
	if w.Code != stdhttp.StatusOK {
		t.Fatalf("admin status = %d", w.Code)
	}
}
