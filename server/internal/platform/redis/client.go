// Package redis owns GIM's reusable Redis client lifecycle.
package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/kanhai447/GIM/server/internal/platform/config"
	redisv9 "github.com/redis/go-redis/v9"
)

const defaultPoolSize = 20

type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

type Client struct{ client *redisv9.Client }

type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return "redis " + e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }

func FromValues(values config.Values) (Config, error) {
	host, err := values.Required("REDIS_HOST")
	if err != nil {
		return Config{}, err
	}
	port, err := values.Int("REDIS_PORT")
	if err != nil || port < 1 || port > 65535 {
		return Config{}, errors.New("configuration key REDIS_PORT must be a valid port")
	}
	password, _ := values.Lookup("REDIS_PASSWORD")
	if password == "CHANGE_ME" {
		return Config{}, errors.New("configuration key REDIS_PASSWORD is not configured")
	}
	database, err := optionalInt(values, "REDIS_DB", 0)
	if err != nil || database < 0 {
		return Config{}, errors.New("configuration key REDIS_DB is invalid")
	}
	poolSize, err := optionalInt(values, "REDIS_POOL_SIZE", defaultPoolSize)
	if err != nil || poolSize < 1 {
		return Config{}, errors.New("configuration key REDIS_POOL_SIZE must be positive")
	}
	return Config{host, port, password, database, poolSize}, nil
}

func Open(ctx context.Context, cfg Config) (*Client, error) {
	inner := redisv9.NewClient(&redisv9.Options{
		Addr:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
		Protocol: 2,
	})
	client := &Client{client: inner}
	if err := client.Health(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Raw() *redisv9.Client { return c.client }

func (c *Client) Health(ctx context.Context) error {
	if c == nil || c.client == nil {
		return &operationError{"health check", errors.New("client is not initialized")}
	}
	if err := c.client.Ping(ctx).Err(); err != nil {
		return &operationError{"health check", err}
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

func optionalInt(values config.Values, key string, fallback int) (int, error) {
	value, ok := values.Lookup(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("configuration key %s must be an integer", key)
	}
	return parsed, nil
}
