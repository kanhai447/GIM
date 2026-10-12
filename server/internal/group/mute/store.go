// Package mute owns temporary Group member prohibitions in Redis.
package mute

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
)

type Store interface {
	Mute(ctx context.Context, memberID uint64, duration time.Duration) error
	Unmute(ctx context.Context, memberID uint64) error
	IsMuted(ctx context.Context, memberID uint64) (bool, error)
}

type RedisStore struct {
	client  redisv9.Cmdable
	timeout time.Duration
}

func NewRedisStore(client redisv9.Cmdable, timeout time.Duration) (*RedisStore, error) {
	if client == nil || timeout <= 0 {
		return nil, errors.New("group mute redis configuration is invalid")
	}
	return &RedisStore{client: client, timeout: timeout}, nil
}

func (store *RedisStore) Mute(ctx context.Context, memberID uint64, duration time.Duration) error {
	if memberID == 0 || duration <= 0 {
		return errors.New("group mute arguments are invalid")
	}
	operationContext, cancel := context.WithTimeout(ctx, store.timeout)
	defer cancel()
	if err := store.client.Set(operationContext, rediskeys.GroupProhibition(memberID), "1", duration).Err(); err != nil {
		return fmt.Errorf("group mute redis set failed: %w", err)
	}
	return nil
}

func (store *RedisStore) Unmute(ctx context.Context, memberID uint64) error {
	if memberID == 0 {
		return errors.New("group unmute arguments are invalid")
	}
	operationContext, cancel := context.WithTimeout(ctx, store.timeout)
	defer cancel()
	if err := store.client.Del(operationContext, rediskeys.GroupProhibition(memberID)).Err(); err != nil {
		return fmt.Errorf("group mute redis delete failed: %w", err)
	}
	return nil
}

func (store *RedisStore) IsMuted(ctx context.Context, memberID uint64) (bool, error) {
	if memberID == 0 {
		return false, errors.New("group mute arguments are invalid")
	}
	operationContext, cancel := context.WithTimeout(ctx, store.timeout)
	defer cancel()
	exists, err := store.client.Exists(operationContext, rediskeys.GroupProhibition(memberID)).Result()
	if err != nil {
		return false, fmt.Errorf("group mute redis lookup failed: %w", err)
	}
	return exists == 1, nil
}
