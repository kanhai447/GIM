// Package process provides safe, shared configuration for executable service wiring.
package process

import (
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
)

const (
	defaultStartupTimeout  = 10 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

type Config struct {
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}

func LoadValues(args []string) (platformconfig.Values, error) {
	flags := flag.NewFlagSet("gim-service", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	envFile := flags.String("env", "", "path to the ignored GIM environment file")
	if err := flags.Parse(args); err != nil {
		return platformconfig.Values{}, errors.New("invalid service arguments")
	}
	if *envFile == "" {
		*envFile = os.Getenv("GIM_ENV_FILE")
	}
	if *envFile == "" {
		return platformconfig.Values{}, errors.New("service environment file is required")
	}
	return platformconfig.LoadFile(*envFile)
}

func FromValues(values platformconfig.Values) (Config, error) {
	startup, err := optionalDuration(values, "SERVICE_STARTUP_TIMEOUT", defaultStartupTimeout)
	if err != nil {
		return Config{}, err
	}
	shutdown, err := optionalDuration(values, "SERVICE_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}
	return Config{StartupTimeout: startup, ShutdownTimeout: shutdown}, nil
}

func RPCAddress(values platformconfig.Values, prefix string) (string, error) {
	host, err := values.Required("INTERNAL_API_HOST")
	if err != nil {
		return "", err
	}
	port, err := values.Int(prefix + "_RPC_PORT")
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("internal RPC port configuration is invalid")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func RESTConfig(registration discovery.RegistrationConfig, name string) (rest.RestConf, error) {
	port, err := strconv.Atoi(registration.Endpoint.Port())
	if err != nil || port < 1 || port > 65535 || registration.Endpoint.Hostname() == "" {
		return rest.RestConf{}, errors.New("internal API endpoint configuration is invalid")
	}
	return rest.RestConf{
		ServiceConf: service.ServiceConf{Name: name},
		Host:        registration.Endpoint.Hostname(),
		Port:        port,
		Timeout:     5000,
		MaxBytes:    1 << 20,
	}, nil
}

func optionalDuration(values platformconfig.Values, key string, fallback time.Duration) (time.Duration, error) {
	value, ok := values.Lookup(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("configuration key " + key + " must be a positive duration")
	}
	return parsed, nil
}
