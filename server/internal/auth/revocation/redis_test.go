package revocation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
)

type fakeRedis struct {
	setKey string
	setTTL time.Duration
	exists bool
}

func (redis *fakeRedis) Set(_ context.Context, key string, _ any, ttl time.Duration) *redisv9.StatusCmd {
	redis.setKey, redis.setTTL = key, ttl
	command := redisv9.NewStatusCmd(context.Background())
	command.SetVal("OK")
	return command
}

func (redis *fakeRedis) Exists(_ context.Context, _ ...string) *redisv9.IntCmd {
	command := redisv9.NewIntCmd(context.Background())
	if redis.exists {
		command.SetVal(1)
	}
	return command
}

func TestFingerprintIsStableAndDoesNotExposeToken(t *testing.T) {
	raw := "header.payload.signature"
	first := Fingerprint(raw)
	second := Fingerprint(raw)
	if first != second || len(first) != 64 {
		t.Fatalf("Fingerprint() = %q, %q", first, second)
	}
	if strings.Contains(first, raw) {
		t.Fatal("fingerprint exposed the raw token")
	}
}

func TestRedisStoreUsesFingerprintKeyAndTTL(t *testing.T) {
	client := &fakeRedis{exists: true}
	store := NewRedisStore(client)
	fingerprint := strings.Repeat("a", 64)
	if err := store.Revoke(context.Background(), fingerprint, 45*time.Minute); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if client.setKey != rediskeys.AuthLogout(fingerprint) || client.setTTL != 45*time.Minute {
		t.Fatalf("Redis SET key/TTL = %q/%v", client.setKey, client.setTTL)
	}
	revoked, err := store.IsRevoked(context.Background(), fingerprint)
	if err != nil || !revoked {
		t.Fatalf("IsRevoked() = %t, %v", revoked, err)
	}
}

func TestRedisStoreRejectsInvalidState(t *testing.T) {
	if err := NewRedisStore(nil).Revoke(context.Background(), "fingerprint", time.Minute); err == nil {
		t.Fatal("Revoke() error = nil for missing client")
	}
	if err := NewRedisStore(&fakeRedis{}).Revoke(context.Background(), "fingerprint", 0); err == nil {
		t.Fatal("Revoke() error = nil for invalid TTL")
	}
}
