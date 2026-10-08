package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/auth/revocation"
	"github.com/kanhai447/GIM/server/internal/platform/config"
	redisclient "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
)

func TestAuthRedisBlacklistTTL(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Auth Redis integration tests")
	}
	values, err := config.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	cfg, err := redisclient.FromValues(values)
	if err != nil {
		t.Fatalf("build Redis configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := redisclient.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open Redis client: %s", redisErrorCategory(err))
	}
	defer client.Close()

	rawToken := "integration-token-material-" + time.Now().UTC().Format("20060102150405.000000000")
	fingerprint := revocation.Fingerprint(rawToken)
	key := rediskeys.AuthLogout(fingerprint)
	t.Cleanup(func() { _ = client.Raw().Del(context.Background(), key).Err() })

	store := revocation.NewRedisStore(client.Raw())
	requestedTTL := 30 * time.Second
	if err := store.Revoke(ctx, fingerprint, requestedTTL); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	revoked, err := store.IsRevoked(ctx, fingerprint)
	if err != nil || !revoked {
		t.Fatalf("IsRevoked() = %t, %v", revoked, err)
	}
	remaining, err := client.Raw().TTL(ctx, key).Result()
	if err != nil || remaining <= 0 || remaining > requestedTTL {
		t.Fatalf("blacklist TTL = %v, %v", remaining, err)
	}
	if key == rawToken || len(key) <= len("gim:auth:logout:") {
		t.Fatal("blacklist key did not use the token fingerprint")
	}
}
