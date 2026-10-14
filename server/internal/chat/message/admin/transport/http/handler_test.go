package http

import (
	"context"
	"log"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	chatadmin "github.com/kanhai447/GIM/server/internal/chat/message/admin"
)

type fakeApplication struct{}

func (fakeApplication) List(context.Context, chatadmin.Query) (chatadmin.Page, error) {
	return chatadmin.Page{}, nil
}
func (fakeApplication) Delete(context.Context, uint64) error { return nil }

func TestChatAdminAuthorization(t *testing.T) {
	handler := &Handler{application: fakeApplication{}, logger: log.Default()}
	member := httptest.NewRequest(stdhttp.MethodGet, "/api/chat/admin/history", nil)
	member.Header.Set("User-ID", "7")
	member.Header.Set("Role", "2")
	response := httptest.NewRecorder()
	handler.List(response, member)
	if response.Code != stdhttp.StatusForbidden {
		t.Fatalf("member status = %d", response.Code)
	}

	admin := httptest.NewRequest(stdhttp.MethodGet, "/api/chat/admin/history", nil)
	admin.Header.Set("User-ID", "7")
	admin.Header.Set("Role", "1")
	response = httptest.NewRecorder()
	handler.List(response, admin)
	if response.Code != stdhttp.StatusOK {
		t.Fatalf("admin status = %d", response.Code)
	}
}
