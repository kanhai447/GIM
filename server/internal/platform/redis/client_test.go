package redis

import (
	"errors"
	"strings"
	"testing"
)

func TestOperationErrorDoesNotExposeCause(t *testing.T) {
	secret := "redis-password-test-value"
	err := &operationError{"health check", errors.New(secret)}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("operationError leaked its private cause")
	}
}

func TestNilClientCloseIsSafe(t *testing.T) {
	var client *Client
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
