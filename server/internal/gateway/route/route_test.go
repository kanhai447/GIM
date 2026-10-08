package route

import (
	"errors"
	"testing"
)

func TestParseSupportedServices(t *testing.T) {
	for public, internal := range map[string]string{
		"auth": "auth_api", "user": "user_api", "chat": "chat_api", "group": "group_api",
		"file": "file_api", "settings": "settings_api", "logs": "logs_api",
	} {
		target, err := Parse("/api/" + public + "/resource")
		if err != nil || target.DiscoveryService != internal {
			t.Fatalf("Parse(%q) = %#v, %v", public, target, err)
		}
	}
}

func TestParseRejectsInvalidAndUnsupportedPaths(t *testing.T) {
	for _, path := range []string{"/", "/api", "/api/", "/api/user", "/other/user/info"} {
		if _, err := Parse(path); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("Parse(%q) error = %v", path, err)
		}
	}
	if _, err := Parse("/api/admin/info"); !errors.Is(err, ErrUnsupportedService) {
		t.Fatalf("unsupported Parse() error = %v", err)
	}
}
