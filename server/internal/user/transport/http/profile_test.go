package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanhai447/GIM/server/internal/user/domain"
)

type fakeUserReader struct {
	user domain.User
	err  error
}

func (reader fakeUserReader) GetUserByID(context.Context, uint64) (domain.User, error) {
	return reader.user, reader.err
}

func TestProfileHandlerReturnsPublicFieldsOnly(t *testing.T) {
	secretHash := "private-password-hash-value"
	handler := NewProfileHandler(fakeUserReader{user: domain.User{
		ID:           8,
		Account:      "gim-user",
		PasswordHash: secretHash,
		Nickname:     "GIM User",
		Avatar:       "/avatar.png",
		Role:         domain.RoleMember,
		Status:       domain.StatusActive,
	}})
	request := httptest.NewRequest(http.MethodGet, "/api/user/user_info", nil)
	request.Header.Set("User-ID", "8")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"account":"gim-user"`) {
		t.Fatalf("profile response status=%d body=%s", recorder.Code, body)
	}
	if strings.Contains(body, secretHash) || strings.Contains(strings.ToLower(body), "password") {
		t.Fatal("profile response exposed password material")
	}
}

func TestProfileHandlerRejectsMissingUserID(t *testing.T) {
	handler := NewProfileHandler(fakeUserReader{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/user_info", nil))
	if !strings.Contains(recorder.Body.String(), `"code":1001`) {
		t.Fatalf("profile response = %s", recorder.Body.String())
	}
}
