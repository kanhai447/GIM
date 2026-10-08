// Package revocation stores logout state without persisting raw JWTs.
package revocation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
)

type redisCommands interface {
	Set(context.Context, string, any, time.Duration) *redisv9.StatusCmd
	Exists(context.Context, ...string) *redisv9.IntCmd
}

type RedisStore struct{ client redisCommands }

func NewRedisStore(client redisCommands) *RedisStore { return &RedisStore{client: client} }

func Fingerprint(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}

func (store *RedisStore) Revoke(ctx context.Context, tokenFingerprint string, ttl time.Duration) error {
	if store == nil || store.client == nil {
		return errors.New("redis revocation store is not initialized")
	}
	if ttl <= 0 {
		return errors.New("revocation ttl must be positive")
	}
	return store.client.Set(ctx, rediskeys.AuthLogout(tokenFingerprint), "1", ttl).Err()
}

func (store *RedisStore) IsRevoked(ctx context.Context, tokenFingerprint string) (bool, error) {
	if store == nil || store.client == nil {
		return false, errors.New("redis revocation store is not initialized")
	}
	count, err := store.client.Exists(ctx, rediskeys.AuthLogout(tokenFingerprint)).Result()
	return count > 0, err
}
