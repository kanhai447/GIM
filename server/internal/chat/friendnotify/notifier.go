// Package friendnotify turns Chat Hub local presence transitions into
// best-effort friend.online/friend.offline notifications.
package friendnotify

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

const (
	defaultBuffer  = 256
	defaultWorkers = 1
	defaultTimeout = 2 * time.Second
	maximumBuffer  = 65536
	maximumWorkers = 8
	maximumTimeout = 30 * time.Second
)

// Config controls the bounded notification queue.
type Config struct {
	Buffer           int
	Workers          int
	OperationTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{Buffer: defaultBuffer, Workers: defaultWorkers, OperationTimeout: defaultTimeout}
}

// FriendLookup returns the active friends of a user.
type FriendLookup interface {
	FriendIDs(ctx context.Context, userID uint64) ([]uint64, error)
}

// Sender delivers an envelope to a locally connected user through the Hub.
type Sender interface {
	SendToUser(ctx context.Context, userID uint64, payload []byte) (chat.DeliveryResult, error)
}

// GlobalPresenceCheck is the V2 multi-instance hook. The single-instance
// implementation always returns true; a future implementation reads global
// Redis Presence before notifying.
type GlobalPresenceCheck interface {
	ShouldNotify(ctx context.Context, userID uint64, online bool) (bool, error)
}

type singleInstancePresence struct{}

func (singleInstancePresence) ShouldNotify(context.Context, uint64, bool) (bool, error) {
	return true, nil
}

type event struct {
	userID uint64
	online bool
	at     time.Time
}

// Notifier implements chat.LocalConnectionTransitionObserver. Its callbacks
// only try a non-blocking send to a bounded channel; all I/O happens in workers.
type Notifier struct {
	mu      sync.RWMutex
	closed  bool
	events  chan event
	lookup  FriendLookup
	sender  Sender
	checker GlobalPresenceCheck
	config  Config
	logger  *log.Logger
	dropped atomic.Uint64
	start   sync.Once
	stop    sync.Once
	wg      sync.WaitGroup
}

func New(lookup FriendLookup, config Config, logger *log.Logger) (*Notifier, error) {
	if lookup == nil {
		return nil, errors.New("friend lookup is required")
	}
	if config.Buffer < 1 || config.Buffer > maximumBuffer {
		return nil, errors.New("friend notify buffer must be between 1 and 65536")
	}
	if config.Workers < 1 || config.Workers > maximumWorkers {
		return nil, errors.New("friend notify workers must be between 1 and 8")
	}
	if config.OperationTimeout <= 0 || config.OperationTimeout > maximumTimeout {
		return nil, errors.New("friend notify operation timeout must be positive and no greater than 30s")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Notifier{
		events: make(chan event, config.Buffer), lookup: lookup, checker: singleInstancePresence{},
		config: config, logger: logger,
	}, nil
}

// SetSender attaches the Hub after module construction. It must be called
// before Start.
func (notifier *Notifier) SetSender(sender Sender) {
	notifier.mu.Lock()
	notifier.sender = sender
	notifier.mu.Unlock()
}

// Start launches the bounded worker pool.
func (notifier *Notifier) Start(ctx context.Context) {
	notifier.start.Do(func() {
		for range notifier.config.Workers {
			notifier.wg.Add(1)
			go notifier.run(ctx)
		}
	})
}

func (notifier *Notifier) LocalUserOnline(userID uint64) {
	notifier.enqueue(event{userID: userID, online: true, at: time.Now().UTC()})
}

func (notifier *Notifier) LocalUserOffline(userID uint64) {
	notifier.enqueue(event{userID: userID, online: false, at: time.Now().UTC()})
}

func (notifier *Notifier) enqueue(value event) {
	if value.userID == 0 {
		return
	}
	notifier.mu.RLock()
	defer notifier.mu.RUnlock()
	if notifier.closed {
		return
	}
	select {
	case notifier.events <- value:
	default:
		count := notifier.dropped.Add(1)
		notifier.logger.Printf("friend notify dropped event user_id=%d online=%t dropped_total=%d", value.userID, value.online, count)
	}
}

func (notifier *Notifier) Dropped() uint64 {
	return notifier.dropped.Load()
}

// Shutdown stops accepting events, closes the queue and waits for accepted
// events to drain. Workers use a background context, so an already-cancelled
// lifecycle context does not discard queued notifications.
func (notifier *Notifier) Shutdown(ctx context.Context) error {
	notifier.stop.Do(func() {
		notifier.mu.Lock()
		notifier.closed = true
		close(notifier.events)
		notifier.mu.Unlock()
	})
	done := make(chan struct{})
	go func() {
		notifier.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (notifier *Notifier) run(context.Context) {
	defer notifier.wg.Done()
	for value := range notifier.events {
		notifier.handle(value)
	}
}

func (notifier *Notifier) handle(value event) {
	notifier.mu.RLock()
	sender := notifier.sender
	notifier.mu.RUnlock()
	if sender == nil {
		notifier.logger.Printf("friend notify skipped event without sender user_id=%d", value.userID)
		return
	}
	checkContext, checkCancel := context.WithTimeout(context.Background(), notifier.config.OperationTimeout)
	allowed, err := notifier.checker.ShouldNotify(checkContext, value.userID, value.online)
	checkCancel()
	if err != nil {
		notifier.logger.Printf("friend notify presence check failed user_id=%d", value.userID)
		return
	}
	if !allowed {
		return
	}
	lookupContext, lookupCancel := context.WithTimeout(context.Background(), notifier.config.OperationTimeout)
	friendIDs, err := notifier.lookup.FriendIDs(lookupContext, value.userID)
	lookupCancel()
	if err != nil {
		notifier.logger.Printf("friend notify friend lookup failed user_id=%d", value.userID)
		return
	}
	envelope, err := notificationEnvelope(value)
	if err != nil {
		notifier.logger.Printf("friend notify envelope failed user_id=%d", value.userID)
		return
	}
	for _, friendID := range friendIDs {
		if friendID == 0 || friendID == value.userID {
			continue
		}
		sendContext, sendCancel := context.WithTimeout(context.Background(), notifier.config.OperationTimeout)
		_, sendErr := sender.SendToUser(sendContext, friendID, envelope)
		sendCancel()
		if sendErr != nil {
			notifier.logger.Printf("friend notify delivery failed friend_id=%d user_id=%d error=%v", friendID, value.userID, sendErr)
		}
	}
}

func notificationEnvelope(value event) ([]byte, error) {
	eventName := protocol.EventFriendOnline
	if !value.online {
		eventName = protocol.EventFriendOffline
	}
	data, err := protocol.Data(protocol.FriendPresenceData{UserID: value.userID, At: value.at.UTC().Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	return protocol.Encode(protocol.Envelope{Event: eventName, Data: data})
}
