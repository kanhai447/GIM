package mysql

import (
	"errors"
	"strings"
	"testing"
)

func TestOperationErrorDoesNotExposeCause(t *testing.T) {
	secret := "password=secret-test-value"
	err := &operationError{"health check", errors.New(secret)}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("operationError leaked its private cause")
	}
	if !errors.Is(err, err.cause) {
		t.Fatal("operationError did not retain its private cause")
	}
}

func TestNilClientCloseIsSafe(t *testing.T) {
	var client *Client
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
