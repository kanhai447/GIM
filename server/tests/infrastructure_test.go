package tests

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/config"
	mysqlclient "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	etcdclient "github.com/kanhai447/GIM/server/internal/platform/etcd"
	redisclient "github.com/kanhai447/GIM/server/internal/platform/redis"
)

func TestLocalInfrastructure(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run local infrastructure integration tests")
	}
	values, err := config.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}

	t.Run("mysql", func(t *testing.T) {
		cfg, err := mysqlclient.FromValues(values)
		if err != nil {
			t.Fatalf("build mysql config: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := mysqlclient.Open(ctx, cfg)
		if err != nil {
			t.Fatalf("open mysql client: %v", err)
		}
		if err := client.Health(ctx); err != nil {
			t.Fatalf("mysql health: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("close mysql client: %v", err)
		}

		invalidPassword := "gim-invalid-password-for-redaction-test"
		cfg.Password = invalidPassword
		badClient, badErr := mysqlclient.Open(ctx, cfg)
		if badClient != nil {
			_ = badClient.Close()
		}
		if badErr == nil {
			t.Fatal("mysql accepted intentionally invalid credentials")
		}
		if strings.Contains(badErr.Error(), invalidPassword) {
			t.Fatal("mysql error leaked the invalid credential")
		}
	})

	t.Run("redis", func(t *testing.T) {
		cfg, err := redisclient.FromValues(values)
		if err != nil {
			t.Fatalf("build redis config: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := redisclient.Open(ctx, cfg)
		if err != nil {
			t.Fatalf("open redis client: %s", redisErrorCategory(err))
		}
		if err := client.Health(ctx); err != nil {
			t.Fatalf("redis health: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("close redis client: %v", err)
		}
	})

	t.Run("etcd", func(t *testing.T) {
		cfg, err := etcdclient.FromValues(values)
		if err != nil {
			t.Fatalf("build etcd config: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := etcdclient.Open(ctx, cfg)
		if err != nil {
			t.Fatalf("open etcd client: %v", err)
		}
		statuses, err := client.Status(ctx)
		if err != nil || len(statuses) == 0 {
			t.Fatalf("etcd status unavailable: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("close etcd client: %v", err)
		}
	})
}

func redisErrorCategory(err error) string {
	cause := errors.Unwrap(err)
	if cause == nil {
		return "UNCLASSIFIED_[REDACTED]"
	}
	message := strings.ToLower(cause.Error())
	switch {
	case strings.Contains(message, "protocol") || strings.Contains(message, "hello") || strings.Contains(message, "unknown command"):
		return "PROTOCOL_NEGOTIATION_FAILED_[REDACTED]"
	case strings.Contains(message, "without any password configured") || strings.Contains(message, "no password is set"):
		return "SERVER_HAS_NO_PASSWORD_[REDACTED]"
	case strings.Contains(message, "wrongpass") || strings.Contains(message, "invalid username-password"):
		return "CREDENTIALS_REJECTED_[REDACTED]"
	case strings.Contains(message, "auth") || strings.Contains(message, "password"):
		return "AUTHENTICATION_FAILED_[REDACTED]"
	case strings.Contains(message, "connect") || strings.Contains(message, "refused") || strings.Contains(message, "timeout"):
		return "CONNECTION_FAILED_[REDACTED]"
	default:
		return "UNCLASSIFIED_[REDACTED]"
	}
}
