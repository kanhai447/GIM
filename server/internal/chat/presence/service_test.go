package presence

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu           sync.Mutex
	entries      map[uint64]map[string]time.Time
	fail         bool
	gate         <-chan struct{}
	onlineCalls  int
	offlineCalls int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{entries: make(map[uint64]map[string]time.Time)}
}

func (store *memoryStore) MarkOnline(ctx context.Context, userID uint64, instanceID string, expiresAt time.Time, _ time.Duration) error {
	if err := store.wait(ctx); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.onlineCalls++
	if store.fail {
		return errors.New("redis unavailable")
	}
	if store.entries[userID] == nil {
		store.entries[userID] = make(map[string]time.Time)
	}
	store.entries[userID][instanceID] = expiresAt
	return nil
}

func (store *memoryStore) MarkOffline(ctx context.Context, userID uint64, instanceID string) error {
	if err := store.wait(ctx); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.offlineCalls++
	if store.fail {
		return errors.New("redis unavailable")
	}
	delete(store.entries[userID], instanceID)
	if len(store.entries[userID]) == 0 {
		delete(store.entries, userID)
	}
	return nil
}

func (store *memoryStore) IsOnline(ctx context.Context, userID uint64, now time.Time) (bool, error) {
	if err := store.wait(ctx); err != nil {
		return false, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.fail {
		return false, errors.New("redis unavailable")
	}
	for instanceID, expiresAt := range store.entries[userID] {
		if !expiresAt.After(now) {
			delete(store.entries[userID], instanceID)
		}
	}
	return len(store.entries[userID]) > 0, nil
}

func (store *memoryStore) wait(ctx context.Context) error {
	store.mu.Lock()
	gate := store.gate
	store.mu.Unlock()
	if gate == nil {
		return nil
	}
	select {
	case <-gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (store *memoryStore) setFailure(fail bool) {
	store.mu.Lock()
	store.fail = fail
	store.mu.Unlock()
}

func (store *memoryStore) setGate(gate <-chan struct{}) {
	store.mu.Lock()
	store.gate = gate
	store.mu.Unlock()
}

func TestServiceOnlineQueryOfflineAndShutdown(t *testing.T) {
	store := newMemoryStore()
	service := runningService(t, store, testConfig("chat-a"))
	service.LocalUserOnline(7)
	waitFor(t, time.Second, func() bool {
		online, _ := service.IsOnline(context.Background(), 7)
		return online
	})
	service.LocalUserOnline(7)
	time.Sleep(20 * time.Millisecond)
	store.mu.Lock()
	if store.onlineCalls != 1 {
		t.Fatalf("duplicate online writes = %d", store.onlineCalls)
	}
	store.mu.Unlock()
	service.LocalUserOffline(7)
	waitFor(t, time.Second, func() bool {
		online, _ := service.IsOnline(context.Background(), 7)
		return !online
	})

	service.LocalUserOnline(8)
	waitFor(t, time.Second, func() bool {
		online, _ := service.IsOnline(context.Background(), 8)
		return online
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.entries) != 0 {
		t.Fatalf("shutdown left contributions: %#v", store.entries)
	}
}

func TestServiceTimeoutUnavailableAndRecovery(t *testing.T) {
	store := newMemoryStore()
	configuration := testConfig("chat-recovery")
	configuration.OperationTimeout = 25 * time.Millisecond
	configuration.RetryInterval = 30 * time.Millisecond
	service := runningService(t, store, configuration)

	blocked := make(chan struct{})
	store.setGate(blocked)
	started := time.Now()
	service.LocalUserOnline(9)
	if elapsed := time.Since(started); elapsed > 20*time.Millisecond {
		t.Fatalf("UserOnline blocked Hub-facing path for %v", elapsed)
	}
	time.Sleep(40 * time.Millisecond)
	store.setGate(nil)
	store.setFailure(true)
	waitFor(t, time.Second, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return store.onlineCalls > 0
	})
	store.setFailure(false)
	waitFor(t, time.Second, func() bool {
		online, _ := service.IsOnline(context.Background(), 9)
		return online
	})

	service.LocalUserOffline(9)
	waitFor(t, time.Second, func() bool {
		online, _ := service.IsOnline(context.Background(), 9)
		return !online
	})
}

func TestServiceIsOnlineHonorsOperationTimeout(t *testing.T) {
	store := newMemoryStore()
	blocked := make(chan struct{})
	store.setGate(blocked)
	configuration := testConfig("chat-timeout")
	configuration.OperationTimeout = 20 * time.Millisecond
	service := runningService(t, store, configuration)
	started := time.Now()
	if _, err := service.IsOnline(context.Background(), 10); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("IsOnline() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 150*time.Millisecond {
		t.Fatalf("IsOnline timeout took %v", elapsed)
	}
}

func TestServiceRefreshKeepsContributionAlivePastTTL(t *testing.T) {
	store := newMemoryStore()
	configuration := testConfig("chat-refresh")
	configuration.TTL = 90 * time.Millisecond
	configuration.RefreshInterval = 25 * time.Millisecond
	configuration.RetryInterval = 20 * time.Millisecond
	service := runningService(t, store, configuration)
	service.LocalUserOnline(11)
	waitFor(t, time.Second, func() bool {
		online, _ := service.IsOnline(context.Background(), 11)
		return online
	})
	time.Sleep(3 * configuration.TTL)
	online, err := service.IsOnline(context.Background(), 11)
	if err != nil || !online {
		t.Fatalf("refreshed Presence = %t, %v", online, err)
	}
}

func runningService(t *testing.T, store Store, configuration Config) *Service {
	t.Helper()
	service, err := NewService(store, configuration, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go service.Run(ctx)
	t.Cleanup(func() {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = service.Shutdown(shutdownContext)
		cancel()
	})
	return service
}

func testConfig(instanceID string) Config {
	return Config{
		InstanceID: instanceID, TTL: 300 * time.Millisecond, RefreshInterval: 100 * time.Millisecond,
		RetryInterval: 40 * time.Millisecond, OperationTimeout: 50 * time.Millisecond,
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
