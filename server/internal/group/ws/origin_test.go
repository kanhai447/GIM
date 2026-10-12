package ws

import (
	"net/http/httptest"
	"testing"
)

func TestOriginPolicy(t *testing.T) {
	policy, err := NewOriginPolicy([]string{"http://localhost:5173", "https://group.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	allowed := httptest.NewRequest("GET", "http://internal/api/group/ws/chat", nil)
	allowed.Header.Set("Origin", "HTTP://LOCALHOST:5173/")
	if !policy.Allows(allowed) {
		t.Fatal("allowed origin rejected")
	}
	for _, origin := range []string{"", "http://evil.example", "http://localhost:5173/path", "file://localhost"} {
		request := httptest.NewRequest("GET", "http://internal/api/group/ws/chat", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		if policy.Allows(request) {
			t.Fatalf("origin %q unexpectedly allowed", origin)
		}
	}
}
