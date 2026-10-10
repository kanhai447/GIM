package presence

import (
	"context"
	"errors"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
)

const (
	markOfflineScript = `
redis.call('ZREM', KEYS[1], ARGV[1])
if redis.call('ZCARD', KEYS[1]) == 0 then
  redis.call('DEL', KEYS[1])
end
return 1`
	isOnlineScript = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
return redis.call('ZCARD', KEYS[1])`
)

type RedisStore struct{ client *redisv9.Client }

func NewRedisStore(client *redisv9.Client) *RedisStore { return &RedisStore{client: client} }

func (store *RedisStore) MarkOnline(ctx context.Context, userID uint64, instanceID string, expiresAt time.Time, ttl time.Duration) error {
	if err := store.validate(userID, instanceID); err != nil {
		return err
	}
	if ttl <= 0 || expiresAt.IsZero() {
		return errors.New("invalid presence expiration")
	}
	key := rediskeys.PresenceUser(userID)
	_, err := store.client.TxPipelined(ctx, func(pipe redisv9.Pipeliner) error {
		pipe.ZAdd(ctx, key, redisv9.Z{Score: float64(expiresAt.UnixMilli()), Member: instanceID})
		pipe.PExpire(ctx, key, 2*ttl)
		return nil
	})
	return err
}

func (store *RedisStore) MarkOffline(ctx context.Context, userID uint64, instanceID string) error {
	if err := store.validate(userID, instanceID); err != nil {
		return err
	}
	return store.client.Eval(ctx, markOfflineScript, []string{rediskeys.PresenceUser(userID)}, instanceID).Err()
}

func (store *RedisStore) IsOnline(ctx context.Context, userID uint64, now time.Time) (bool, error) {
	if store == nil || store.client == nil || userID == 0 || now.IsZero() {
		return false, errors.New("invalid presence query")
	}
	count, err := store.client.Eval(ctx, isOnlineScript, []string{rediskeys.PresenceUser(userID)}, now.UnixMilli()).Int64()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (store *RedisStore) validate(userID uint64, instanceID string) error {
	if store == nil || store.client == nil || userID == 0 || instanceID == "" {
		return errors.New("invalid presence store state")
	}
	return nil
}
