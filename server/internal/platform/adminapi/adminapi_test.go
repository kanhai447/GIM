package adminapi

import (
	"net/http/httptest"
	"testing"
)

func TestIdentityRejectsMissingOrForgedAdmin(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/user/users", nil)
	request.Header.Set("Role", "1")
	if _, ok := Identity(request); ok {
		t.Fatal("role without trusted user identity was accepted")
	}
	request.Header.Set("User-ID", "7")
	if id, ok := Identity(request); !ok || id != 7 {
		t.Fatalf("admin identity = %d, %t", id, ok)
	}
	request.Header.Set("Role", "2")
	if _, ok := Identity(request); ok {
		t.Fatal("member identity was accepted")
	}
	request.Header.Set("Role", "admin")
	if _, ok := Identity(request); ok {
		t.Fatal("string admin role was accepted")
	}
}

func TestCursorLimitBounds(t *testing.T) {
	request := httptest.NewRequest("GET", "/?beforeId=9&limit=100", nil)
	before, limit, ok := CursorLimit(request.URL.Query(), 20, 100)
	if !ok || before != 9 || limit != 100 {
		t.Fatalf("cursor = %d limit = %d ok = %t", before, limit, ok)
	}
	request = httptest.NewRequest("GET", "/?limit=101", nil)
	if _, _, ok := CursorLimit(request.URL.Query(), 20, 100); ok {
		t.Fatal("unbounded limit was accepted")
	}
}
