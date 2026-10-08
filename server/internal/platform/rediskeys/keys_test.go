package rediskeys

import (
	"strings"
	"testing"
)

func TestAuthLogoutDoesNotContainRawToken(t *testing.T) {
	rawToken := "header.payload.signature"
	fingerprint := strings.Repeat("a", 64)
	key := AuthLogout(fingerprint)
	if key != "gim:auth:logout:"+fingerprint {
		t.Fatalf("AuthLogout() = %q", key)
	}
	if strings.Contains(key, rawToken) {
		t.Fatal("blacklist key contains the raw token")
	}
}
