package http

import (
	"context"
	"github.com/kanhai447/GIM/server/internal/settings/domain"
	"log"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

type fakeApplication struct{}

func (fakeApplication) Get(context.Context) (domain.Info, error)                     { return domain.Defaults(), nil }
func (fakeApplication) Update(_ context.Context, v domain.Info) (domain.Info, error) { return v, nil }
func TestSettingsAdminAuthorization(t *testing.T) {
	h := &Handler{application: fakeApplication{}, logger: log.Default()}
	r := httptest.NewRequest(stdhttp.MethodPut, "/api/settings/admin/info", nil)
	r.Header.Set("User-ID", "7")
	r.Header.Set("Role", "2")
	w := httptest.NewRecorder()
	h.Update(w, r)
	if w.Code != stdhttp.StatusForbidden {
		t.Fatalf("member status=%d", w.Code)
	}
}
