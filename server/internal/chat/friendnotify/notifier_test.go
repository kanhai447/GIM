package friendnotify

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

type fakeLookup struct {
	friends map[uint64][]uint64
	errs    map[uint64]error
}

func (lookup fakeLookup) FriendIDs(_ context.Context, userID uint64) ([]uint64, error) {
	if err := lookup.errs[userID]; err != nil {
		return nil, err
	}
	return lookup.friends[userID], nil
}

type sentEvent struct {
	userID  uint64
	payload []byte
}

type fakeSender struct {
	events chan sentEvent
}

func (sender *fakeSender) SendToUser(_ context.Context, userID uint64, payload []byte) (chat.DeliveryResult, error) {
	sender.events <- sentEvent{userID: userID, payload: payload}
	return chat.DeliveryResult{Delivered: 1}, nil
}

func receive(t *testing.T, sender *fakeSender) sentEvent {
	t.Helper()
	select {
	case event := <-sender.events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for friend notification")
		return sentEvent{}
	}
}

func decode(t *testing.T, payload []byte) (protocol.Envelope, protocol.FriendPresenceData) {
	t.Helper()
	var envelope protocol.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	data, err := protocol.DecodeData[protocol.FriendPresenceData](envelope.Data)
	if err != nil {
		t.Fatalf("decode presence: %v", err)
	}
	return envelope, data
}

func TestNotifierSendsOnlineAndOfflineToFriends(t *testing.T) {
	sender := &fakeSender{events: make(chan sentEvent, 8)}
	notifier, err := New(fakeLookup{friends: map[uint64][]uint64{1: {2, 3}}}, DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	notifier.SetSender(sender)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifier.Start(ctx)

	notifier.LocalUserOnline(1)
	seen := map[uint64]bool{}
	for range 2 {
		event := receive(t, sender)
		seen[event.userID] = true
		envelope, data := decode(t, event.payload)
		if envelope.Event != protocol.EventFriendOnline || data.UserID != 1 || data.At == "" {
			t.Fatalf("online event = %s / %#v", envelope.Event, data)
		}
	}
	if !seen[2] || !seen[3] {
		t.Fatalf("online recipients = %v", seen)
	}
	notifier.LocalUserOffline(1)
	for range 2 {
		event := receive(t, sender)
		envelope, data := decode(t, event.payload)
		if envelope.Event != protocol.EventFriendOffline || data.UserID != 1 {
			t.Fatalf("offline event = %s / %#v", envelope.Event, data)
		}
	}
}

func TestNotifierSkipsSelfAndLookupFailures(t *testing.T) {
	sender := &fakeSender{events: make(chan sentEvent, 4)}
	notifier, err := New(fakeLookup{
		friends: map[uint64][]uint64{1: {1, 2}, 2: {3}},
		errs:    map[uint64]error{2: errors.New("database unavailable")},
	}, DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	notifier.SetSender(sender)
	notifier.Start(context.Background())
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = notifier.Shutdown(shutdownContext)
	}()

	notifier.LocalUserOnline(1)
	event := receive(t, sender)
	if event.userID != 2 {
		t.Fatalf("self was not filtered: %d", event.userID)
	}
	notifier.LocalUserOnline(2)
	select {
	case event := <-sender.events:
		t.Fatalf("lookup failure unexpectedly sent to %d", event.userID)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestNotifierDropsWithoutBlockingWhenQueueIsFull(t *testing.T) {
	notifier, err := New(fakeLookup{friends: map[uint64][]uint64{1: {2}}}, Config{
		Buffer: 1, Workers: 1, OperationTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	notifier.SetSender(&fakeSender{events: make(chan sentEvent, 1)})
	started := time.Now()
	for range 100 {
		notifier.LocalUserOnline(1)
	}
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("enqueue blocked for %s", elapsed)
	}
	if dropped := notifier.Dropped(); dropped != 99 {
		t.Fatalf("dropped = %d, want 99", dropped)
	}
}

func TestNotifierShutdownDrainsAcceptedEvents(t *testing.T) {
	sender := &fakeSender{events: make(chan sentEvent, 8)}
	notifier, err := New(fakeLookup{friends: map[uint64][]uint64{1: {2, 3}}}, Config{
		Buffer: 8, Workers: 1, OperationTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	notifier.SetSender(sender)
	notifier.Start(context.Background())
	notifier.LocalUserOnline(1)
	notifier.LocalUserOffline(1)
	shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := notifier.Shutdown(shutdownContext); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if count := len(sender.events); count != 4 {
		t.Fatalf("drained events = %d, want 4", count)
	}
	notifier.LocalUserOnline(1)
	if count := len(sender.events); count != 4 {
		t.Fatalf("post-shutdown events = %d, want 4", count)
	}
	if dropped := notifier.Dropped(); dropped != 0 {
		t.Fatalf("post-shutdown dropped = %d, want 0", dropped)
	}
}

func TestNotifierRejectsInvalidConfiguration(t *testing.T) {
	if _, err := New(nil, DefaultConfig(), nil); err == nil {
		t.Fatal("nil lookup accepted")
	}
	for _, config := range []Config{
		{Buffer: 0, Workers: 1, OperationTimeout: time.Second},
		{Buffer: 1, Workers: 0, OperationTimeout: time.Second},
		{Buffer: 1, Workers: 1, OperationTimeout: 0},
	} {
		if _, err := New(fakeLookup{}, config, nil); err == nil {
			t.Fatalf("invalid config accepted: %#v", config)
		}
	}
}

type guardSender struct {
	mu     sync.Mutex
	events []sentEvent
}

func (sender *guardSender) SendToUser(_ context.Context, userID uint64, payload []byte) (chat.DeliveryResult, error) {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.events = append(sender.events, sentEvent{userID: userID, payload: payload})
	return chat.DeliveryResult{Delivered: 1}, nil
}

func TestNotifierConcurrentEnqueueIsSafe(t *testing.T) {
	sender := &guardSender{}
	notifier, err := New(fakeLookup{friends: map[uint64][]uint64{1: {2}}}, Config{
		Buffer: 64, Workers: 2, OperationTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	notifier.SetSender(sender)
	notifier.Start(context.Background())
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(2)
		go func() {
			defer wait.Done()
			notifier.LocalUserOnline(1)
		}()
		go func() {
			defer wait.Done()
			notifier.LocalUserOffline(1)
		}()
	}
	wait.Wait()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := notifier.Shutdown(shutdownContext); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
