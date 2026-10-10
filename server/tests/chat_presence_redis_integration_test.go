package tests

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/presence"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
)

func TestChatPresenceRedisMultiInstanceTTL(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Chat Presence Redis integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := platformredis.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := platformredis.Open(ctx, configuration)
	if err != nil {
		t.Fatalf("open Redis client: %s", redisErrorCategory(err))
	}
	defer client.Close()

	userID := uint64(time.Now().UnixNano())
	key := rediskeys.PresenceUser(userID)
	t.Cleanup(func() { _ = client.Raw().Del(context.Background(), key).Err() })
	store := presence.NewRedisStore(client.Raw())

	now := time.Now()
	if err := store.MarkOnline(ctx, userID, "chat-a", now.Add(250*time.Millisecond), 250*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOnline(ctx, userID, "chat-b", now.Add(250*time.Millisecond), 250*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	assertPresenceOnline(t, store, userID, true)
	if err := store.MarkOffline(ctx, userID, "chat-a"); err != nil {
		t.Fatal(err)
	}
	assertPresenceOnline(t, store, userID, true)
	if err := store.MarkOffline(ctx, userID, "chat-b"); err != nil {
		t.Fatal(err)
	}
	assertPresenceOnline(t, store, userID, false)

	now = time.Now()
	if err := store.MarkOnline(ctx, userID, "chat-a", now.Add(70*time.Millisecond), 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOnline(ctx, userID, "chat-b", now.Add(300*time.Millisecond), 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(110 * time.Millisecond)
	assertPresenceOnline(t, store, userID, true)
	if score, err := client.Raw().ZScore(ctx, key, "chat-a").Result(); !errors.Is(err, redisv9.Nil) || score != 0 {
		t.Fatalf("expired instance contribution was not pruned: score=%v err=%v", score, err)
	}
	if err := store.MarkOffline(ctx, userID, "chat-b"); err != nil {
		t.Fatal(err)
	}
	assertPresenceOnline(t, store, userID, false)
	if count, err := client.Raw().Exists(ctx, key).Result(); err != nil || count != 0 {
		t.Fatalf("Presence test key remains count=%d err=%v", count, err)
	}
}

func TestChatPresenceServicesRecoverCrashAndPreserveOtherInstance(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Chat Presence Redis integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := platformredis.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := platformredis.Open(ctx, configuration)
	if err != nil {
		t.Fatalf("open Redis client: %s", redisErrorCategory(err))
	}
	defer client.Close()

	userID := uint64(time.Now().UnixNano())
	key := rediskeys.PresenceUser(userID)
	t.Cleanup(func() { _ = client.Raw().Del(context.Background(), key).Err() })
	store := presence.NewRedisStore(client.Raw())
	logger := log.New(io.Discard, "", 0)
	serviceConfig := func(instanceID string) presence.Config {
		return presence.Config{
			InstanceID: instanceID, TTL: 180 * time.Millisecond, RefreshInterval: 50 * time.Millisecond,
			RetryInterval: 30 * time.Millisecond, OperationTimeout: 100 * time.Millisecond,
		}
	}
	serviceA, err := presence.NewService(store, serviceConfig("chat-a"), logger)
	if err != nil {
		t.Fatal(err)
	}
	serviceB, err := presence.NewService(store, serviceConfig("chat-b"), logger)
	if err != nil {
		t.Fatal(err)
	}
	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	go serviceA.Run(ctxA)
	go serviceB.Run(ctxB)
	defer cancelA()
	defer cancelB()

	serviceA.LocalUserOnline(userID)
	serviceB.LocalUserOnline(userID)
	waitForRedisPresence(t, store, userID, true)
	serviceA.LocalUserOffline(userID)
	waitForRedisMemberCount(t, client, key, 1)
	assertPresenceOnline(t, store, userID, true)
	serviceA.LocalUserOnline(userID)
	waitForRedisMemberCount(t, client, key, 2)

	// Simulate instance A crashing without graceful cleanup. Instance B keeps
	// refreshing, so A's stale member expires without making the user offline.
	cancelA()
	time.Sleep(260 * time.Millisecond)
	assertPresenceOnline(t, store, userID, true)
	waitForRedisMemberCount(t, client, key, 1)

	serviceB.LocalUserOffline(userID)
	waitForRedisPresence(t, store, userID, false)
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := serviceB.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
	if count, err := client.Raw().Exists(ctx, key).Result(); err != nil || count != 0 {
		t.Fatalf("Presence service test key remains count=%d err=%v", count, err)
	}
}

func assertPresenceOnline(t *testing.T, store *presence.RedisStore, userID uint64, expected bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	online, err := store.IsOnline(ctx, userID, time.Now())
	if err != nil || online != expected {
		t.Fatalf("IsOnline() = %t, %v; expected %t", online, err, expected)
	}
}

func waitForRedisPresence(t *testing.T, store *presence.RedisStore, userID uint64, expected bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		online, err := store.IsOnline(ctx, userID, time.Now())
		cancel()
		if err == nil && online == expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Redis Presence user=%d online=%t err=%v expected=%t", userID, online, err, expected)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForRedisMemberCount(t *testing.T, client *platformredis.Client, key string, expected int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		count, err := client.Raw().ZCard(ctx, key).Result()
		cancel()
		if err == nil && count == expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Redis Presence members=%d err=%v expected=%d", count, err, expected)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
