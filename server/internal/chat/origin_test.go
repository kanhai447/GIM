package chat

import (
	"net/http/httptest"
	"testing"
)

func TestOriginPolicyAllowsOnlyConfiguredOrigins(t *testing.T) {
	policy, err := NewOriginPolicy([]string{"http://localhost:5173", "https://chat.example.test"})
	if err != nil {
		t.Fatalf("NewOriginPolicy() error = %v", err)
	}
	allowed := httptest.NewRequest("GET", "http://internal/api/chat/ws/chat", nil)
	allowed.Header.Set("Origin", "http://localhost:5173")
	if !policy.Allows(allowed) {
		t.Fatal("configured origin was rejected")
	}
	for _, origin := range []string{"", "http://evil.example", "http://localhost:5173/path", "null"} {
		request := httptest.NewRequest("GET", "http://internal/api/chat/ws/chat", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		if policy.Allows(request) {
			t.Fatalf("origin %q was accepted", origin)
		}
	}
}
