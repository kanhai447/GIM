// Package config loads Gateway network and timeout settings.
package config

import (
	"errors"
	"net"
	"strconv"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

const (
	defaultDiscoveryTimeout = 2 * time.Second
	defaultAuthTimeout      = 3 * time.Second
	defaultProxyTimeout     = 15 * time.Second
)

type Config struct {
	ListenAddress    string
	DiscoveryTimeout time.Duration
	AuthTimeout      time.Duration
	ProxyTimeout     time.Duration
}

func FromValues(values platformconfig.Values) (Config, error) {
	host, err := values.Required("GATEWAY_HOST")
	if err != nil {
		return Config{}, err
	}
	port, err := values.Int("GATEWAY_PORT")
	if err != nil || port < 1 || port > 65535 {
		return Config{}, errors.New("configuration key GATEWAY_PORT must be a valid port")
	}
	discoveryTimeout, err := optionalDuration(values, "GATEWAY_DISCOVERY_TIMEOUT", defaultDiscoveryTimeout)
	if err != nil {
		return Config{}, err
	}
	authTimeout, err := optionalDuration(values, "GATEWAY_AUTH_TIMEOUT", defaultAuthTimeout)
	if err != nil {
		return Config{}, err
	}
	proxyTimeout, err := optionalDuration(values, "GATEWAY_PROXY_TIMEOUT", defaultProxyTimeout)
	if err != nil {
		return Config{}, err
	}
	return Config{
		ListenAddress: net.JoinHostPort(host, strconv.Itoa(port)), DiscoveryTimeout: discoveryTimeout,
		AuthTimeout: authTimeout, ProxyTimeout: proxyTimeout,
	}, nil
}

func optionalDuration(values platformconfig.Values, key string, fallback time.Duration) (time.Duration, error) {
	value, ok := values.Lookup(key)
	if !ok || value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, errors.New("configuration key " + key + " must be a positive duration")
	}
	return duration, nil
}
