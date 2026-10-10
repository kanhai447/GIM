package presence

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var ErrServiceStopped = errors.New("chat presence service is stopped")

type desiredOperation struct {
	userID uint64
	online bool
}

type shutdownRequest struct {
	ctx    context.Context
	result chan error
}

// Service converts local Hub transitions into bounded, eventually consistent
// Redis operations. Transition methods only update in-memory desired state and
// never perform network I/O on the Hub event loop.
type Service struct {
	store  Store
	config Config
	logger *log.Logger

	mu         sync.Mutex
	desired    map[uint64]bool
	dirty      map[uint64]struct{}
	wake       chan struct{}
	stop       chan shutdownRequest
	done       chan struct{}
	runOnce    sync.Once
	shutdownMu sync.Mutex
}

func NewService(store Store, config Config, logger *log.Logger) (*Service, error) {
	if store == nil || config.InstanceID == "" || config.TTL <= 0 || config.RefreshInterval <= 0 ||
		config.RefreshInterval >= config.TTL || config.RetryInterval <= 0 || config.RetryInterval >= config.TTL ||
		config.OperationTimeout <= 0 {
		return nil, errors.New("invalid chat presence service configuration")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Service{
		store: store, config: config, logger: logger, desired: make(map[uint64]bool),
		dirty: make(map[uint64]struct{}), wake: make(chan struct{}, 1),
		stop: make(chan shutdownRequest, 1), done: make(chan struct{}),
	}, nil
}

func (service *Service) Run(ctx context.Context) {
	service.runOnce.Do(func() { service.run(ctx) })
}

func (service *Service) run(ctx context.Context) {
	refresh := time.NewTicker(service.config.RefreshInterval)
	retry := time.NewTicker(service.config.RetryInterval)
	defer refresh.Stop()
	defer retry.Stop()
	defer close(service.done)
	for {
		select {
		case request := <-service.stop:
			request.result <- service.flush(request.ctx)
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case request := <-service.stop:
			request.result <- service.flush(request.ctx)
			return
		case <-service.wake:
			_ = service.flush(ctx)
		case <-retry.C:
			_ = service.flush(ctx)
		case <-refresh.C:
			service.markActiveDirty()
			_ = service.flush(ctx)
		}
	}
}

func (service *Service) LocalUserOnline(userID uint64) {
	service.setDesired(userID, true)
}

func (service *Service) LocalUserOffline(userID uint64) {
	service.setDesired(userID, false)
}

func (service *Service) setDesired(userID uint64, online bool) {
	if userID == 0 {
		return
	}
	service.mu.Lock()
	current, known := service.desired[userID]
	if known && current == online {
		service.mu.Unlock()
		return
	}
	service.desired[userID] = online
	service.dirty[userID] = struct{}{}
	service.mu.Unlock()
	service.signal()
}

func (service *Service) IsOnline(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, errors.New("presence user ID is required")
	}
	operationContext, cancel := context.WithTimeout(ctx, service.config.OperationTimeout)
	defer cancel()
	return service.store.IsOnline(operationContext, userID, time.Now())
}

func (service *Service) Shutdown(ctx context.Context) error {
	service.shutdownMu.Lock()
	defer service.shutdownMu.Unlock()
	select {
	case <-service.done:
		return ErrServiceStopped
	default:
	}
	service.mu.Lock()
	for userID, online := range service.desired {
		if online {
			service.desired[userID] = false
			service.dirty[userID] = struct{}{}
		}
	}
	service.mu.Unlock()
	request := shutdownRequest{ctx: ctx, result: make(chan error, 1)}
	select {
	case service.stop <- request:
	case <-service.done:
		return ErrServiceStopped
	case <-ctx.Done():
		return ctx.Err()
	}
	var result error
	select {
	case result = <-request.result:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-service.done:
		return result
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (service *Service) markActiveDirty() {
	service.mu.Lock()
	defer service.mu.Unlock()
	for userID, online := range service.desired {
		if online {
			service.dirty[userID] = struct{}{}
		}
	}
}

func (service *Service) flush(parent context.Context) error {
	operations := service.snapshot()
	var result error
	for _, operation := range operations {
		operationContext, cancel := context.WithTimeout(parent, service.config.OperationTimeout)
		var err error
		if operation.online {
			err = service.store.MarkOnline(operationContext, operation.userID, service.config.InstanceID, time.Now().Add(service.config.TTL), service.config.TTL)
		} else {
			err = service.store.MarkOffline(operationContext, operation.userID, service.config.InstanceID)
		}
		cancel()
		if err != nil {
			service.logger.Printf("chat presence operation failed operation=%s user_id=%d error=%v", operationName(operation.online), operation.userID, err)
			result = errors.Join(result, err)
			continue
		}
		service.complete(operation)
	}
	return result
}

func (service *Service) snapshot() []desiredOperation {
	service.mu.Lock()
	defer service.mu.Unlock()
	operations := make([]desiredOperation, 0, len(service.dirty))
	for userID := range service.dirty {
		operations = append(operations, desiredOperation{userID: userID, online: service.desired[userID]})
	}
	return operations
}

func (service *Service) complete(operation desiredOperation) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.desired[operation.userID] == operation.online {
		delete(service.dirty, operation.userID)
		if !operation.online {
			delete(service.desired, operation.userID)
		}
	}
}

func (service *Service) signal() {
	select {
	case service.wake <- struct{}{}:
	default:
	}
}

func operationName(online bool) string {
	if online {
		return "online"
	}
	return "offline"
}
