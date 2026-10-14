package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/kanhai447/GIM/server/internal/settings/overview"
)

type fakeApplication struct{}

func (fakeApplication) Get(context.Context) (overview.Summary, error) {
	return overview.Summary{UsersTotal: 3, GroupsActive: 1}, nil
}

func TestOverviewAuthorization(t *testing.T) {
	handler := &Handler{application: fakeApplication{}}
	for _, test := range []struct {
		name       string
		userID     string
		role       string
		wantStatus int
	}{
		{name: "missing token identity", wantStatus: stdhttp.StatusForbidden},
		{name: "normal user", userID: "7", role: "2", wantStatus: stdhttp.StatusForbidden},
		{name: "forged string role", userID: "7", role: "admin", wantStatus: stdhttp.StatusForbidden},
		{name: "valid admin", userID: "7", role: "1", wantStatus: stdhttp.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(stdhttp.MethodGet, "/api/settings/admin/overview", nil)
			request.Header.Set("User-ID", test.userID)
			request.Header.Set("Role", test.role)
			response := httptest.NewRecorder()
			handler.Get(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
