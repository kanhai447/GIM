package discovery

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type RegistrationConfig struct {
	Service    string
	InstanceID string
	Endpoint   url.URL
	LeaseTTL   time.Duration
}

// RegistrationFromValues builds an internal API registration exclusively from
// committed configuration keys. prefix is a trusted service constant such as
// AUTH or USER, never an external request value.
func RegistrationFromValues(values platformconfig.Values, prefix, service string) (RegistrationConfig, error) {
	if !validConfigPrefix(prefix) || !validName(service) {
		return RegistrationConfig{}, ErrInvalidService
	}
	host, err := values.Required("INTERNAL_API_HOST")
	if err != nil {
		return RegistrationConfig{}, err
	}
	port, err := values.Int(prefix + "_API_PORT")
	if err != nil || port < 1 || port > 65535 {
		return RegistrationConfig{}, errors.New("internal API port configuration is invalid")
	}
	instanceID, ok := values.Lookup(prefix + "_API_INSTANCE_ID")
	if !ok || strings.TrimSpace(instanceID) == "" {
		instanceID = strings.ReplaceAll(service, "_", "-") + "-local"
	}
	if !validName(instanceID) {
		return RegistrationConfig{}, ErrInvalidService
	}
	ttl := defaultLeaseTTL
	if value, exists := values.Lookup("ETCD_SERVICE_TTL"); exists && value != "" {
		ttl, err = time.ParseDuration(value)
		if err != nil || ttl < time.Second {
			return RegistrationConfig{}, errors.New("configuration key ETCD_SERVICE_TTL must be at least one second")
		}
	}
	endpoint := url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(port))}
	return RegistrationConfig{Service: service, InstanceID: instanceID, Endpoint: endpoint, LeaseTTL: ttl}, nil
}

func validConfigPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, character := range prefix {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}
