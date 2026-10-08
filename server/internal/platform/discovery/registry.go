package discovery

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const defaultLeaseTTL = 15 * time.Second

type Registry struct {
	client *clientv3.Client
	ttl    time.Duration
}

type Registration struct {
	client  *clientv3.Client
	leaseID clientv3.LeaseID
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

func NewRegistry(client *clientv3.Client, ttl time.Duration) (*Registry, error) {
	if client == nil || ttl < time.Second {
		return nil, errors.New("invalid service registry configuration")
	}
	return &Registry{client: client, ttl: ttl}, nil
}

func NewDefaultRegistry(client *clientv3.Client) (*Registry, error) {
	return NewRegistry(client, defaultLeaseTTL)
}

func (registry *Registry) Register(ctx context.Context, service, instanceID string, endpoint url.URL) (*Registration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := serviceKey(service, instanceID)
	if err != nil {
		return nil, err
	}
	parsed, err := parseEndpoint(endpoint.String())
	if err != nil {
		return nil, err
	}
	seconds := int64(registry.ttl / time.Second)
	lease, err := registry.client.Grant(ctx, seconds)
	if err != nil {
		return nil, &operationError{cause: err}
	}
	if _, err := registry.client.Put(ctx, key, parsed.String(), clientv3.WithLease(lease.ID)); err != nil {
		_, _ = registry.client.Revoke(ctx, lease.ID)
		return nil, &operationError{cause: err}
	}
	keepaliveContext, cancel := context.WithCancel(ctx)
	channel, err := registry.client.KeepAlive(keepaliveContext, lease.ID)
	if err != nil {
		cancel()
		_, _ = registry.client.Revoke(ctx, lease.ID)
		return nil, &operationError{cause: err}
	}
	registration := &Registration{client: registry.client, leaseID: lease.ID, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(registration.done)
		for range channel {
		}
	}()
	return registration, nil
}

func (registration *Registration) Close(ctx context.Context) error {
	if registration == nil {
		return nil
	}
	var closeErr error
	registration.once.Do(func() {
		registration.cancel()
		_, closeErr = registration.client.Revoke(ctx, registration.leaseID)
		select {
		case <-registration.done:
		case <-ctx.Done():
			if closeErr == nil {
				closeErr = ctx.Err()
			}
		}
	})
	if closeErr != nil {
		return &operationError{cause: closeErr}
	}
	return nil
}
