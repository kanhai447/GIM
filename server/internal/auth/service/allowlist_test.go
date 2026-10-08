package service

import "testing"

func TestAllowlistMatchesExactPathOnly(t *testing.T) {
	allowlist := DefaultAllowlist()
	for _, path := range []string{"/api/auth/login", "/api/auth/register?source=web", "/api/settings/info"} {
		if !allowlist.IsPublic(path) {
			t.Fatalf("expected public path %q", path)
		}
	}
	for _, path := range []string{"/api/auth/login/extra", "/prefix/api/auth/login", "/api/auth/log", "https://example.com/api/auth/login", "/api/auth/%6cogin"} {
		if allowlist.IsPublic(path) {
			t.Fatalf("unexpected public path %q", path)
		}
	}
}
