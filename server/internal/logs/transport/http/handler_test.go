package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
)

type fakeApplication struct{}

func (fakeApplication) List(context.Context, domain.Query) (domain.Page, error) {
	return domain.Page{}, nil
}
func (fakeApplication) Get(context.Context, uint64) (domain.Log, error) {
	return domain.Log{ID: 1}, nil
}

func TestAdminAuthorization(t *testing.T) {
	handler := &Handler{logs: fakeApplication{}}
	for _, test := range []struct {
		name, user, role string
		want             int
	}{
		{name: "member", user: "8", role: "2", want: http.StatusForbidden},
		{name: "forged role without identity", role: "1", want: http.StatusForbidden},
		{name: "admin", user: "7", role: "1", want: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/logs/logs", nil)
			request.Header.Set("User-ID", test.user)
			request.Header.Set("Role", test.role)
			recorder := httptest.NewRecorder()
			handler.List(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
