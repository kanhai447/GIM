package http

import (
	"context"
	"log"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	useradmin "github.com/kanhai447/GIM/server/internal/user/admin"
)

type fakeApplication struct{}

func (fakeApplication) List(context.Context, useradmin.Query) (useradmin.Page, error) {
	return useradmin.Page{}, nil
}
func (fakeApplication) UpdateRestrictions(context.Context, useradmin.Restrictions) error { return nil }
func (fakeApplication) Disable(context.Context, uint64, uint64) error                    { return nil }
func (fakeApplication) Enable(context.Context, uint64, uint64) error                     { return nil }

func TestAdminAuthorization(t *testing.T) {
	handler := &Handler{application: fakeApplication{}, logger: log.Default()}
	for _, test := range []struct {
		user, role string
		status     int
	}{
		{"7", "2", stdhttp.StatusForbidden},
		{"", "1", stdhttp.StatusForbidden},
		{"7", "1", stdhttp.StatusOK},
	} {
		request := httptest.NewRequest(stdhttp.MethodGet, "/api/user/users", nil)
		request.Header.Set("User-ID", test.user)
		request.Header.Set("Role", test.role)
		response := httptest.NewRecorder()
		handler.List(response, request)
		if response.Code != test.status {
			t.Fatalf("identity %q/%q status = %d, want %d", test.user, test.role, response.Code, test.status)
		}
	}
}
