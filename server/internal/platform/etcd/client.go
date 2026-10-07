// Package etcd owns GIM's reusable etcd v3 client lifecycle.
package etcd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const defaultDialTimeout = 5 * time.Second

type Config struct {
	Endpoints   []string
	DialTimeout time.Duration
}

type EndpointStatus struct {
	Endpoint string
	Version  string
	DBSize   int64
	Leader   bool
}

type Client struct {
	client    *clientv3.Client
	endpoints []string
}

type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return "etcd " + e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }

func FromValues(values config.Values) (Config, error) {
	endpoints, err := values.CSV("ETCD_ENDPOINTS")
	if err != nil {
		return Config{}, err
	}
	dialTimeout := defaultDialTimeout
	if value, ok := values.Lookup("ETCD_DIAL_TIMEOUT"); ok && value != "" {
		dialTimeout, err = time.ParseDuration(value)
		if err != nil {
			return Config{}, errors.New("configuration key ETCD_DIAL_TIMEOUT must be a duration")
		}
	}
	if dialTimeout <= 0 {
		return Config{}, errors.New("configuration key ETCD_DIAL_TIMEOUT must be positive")
	}
	return Config{Endpoints: endpoints, DialTimeout: dialTimeout}, nil
}

func Open(ctx context.Context, cfg Config) (*Client, error) {
	inner, err := clientv3.New(clientv3.Config{Endpoints: cfg.Endpoints, DialTimeout: cfg.DialTimeout})
	if err != nil {
		return nil, &operationError{"initialization", err}
	}
	client := &Client{client: inner, endpoints: append([]string(nil), cfg.Endpoints...)}
	if err := client.Health(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Raw() *clientv3.Client { return c.client }

func (c *Client) Status(ctx context.Context) ([]EndpointStatus, error) {
	if c == nil || c.client == nil {
		return nil, &operationError{"status check", errors.New("client is not initialized")}
	}
	statuses := make([]EndpointStatus, 0, len(c.endpoints))
	for _, endpoint := range c.endpoints {
		response, err := c.client.Status(ctx, endpoint)
		if err != nil {
			return nil, &operationError{"status check", err}
		}
		statuses = append(statuses, EndpointStatus{
			Endpoint: endpoint,
			Version:  response.Version,
			DBSize:   response.DbSize,
			Leader:   response.Header != nil && response.Leader == response.Header.MemberId,
		})
	}
	return statuses, nil
}

func (c *Client) Health(ctx context.Context) error {
	statuses, err := c.Status(ctx)
	if err != nil {
		return &operationError{"health check", err}
	}
	if len(statuses) == 0 {
		return &operationError{"health check", fmt.Errorf("no endpoint status returned")}
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	if err := c.client.Close(); err != nil {
		return &operationError{"close", err}
	}
	return nil
}
