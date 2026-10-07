package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFileAndTypedAccessors(t *testing.T) {
	path := writeConfig(t, strings.Join([]string{
		"# local test configuration",
		"MYSQL_PORT=3306",
		"ETCD_ENDPOINTS='http://127.0.0.1:2379,http://127.0.0.1:22379'",
		"REQUEST_TIMEOUT=5s",
		"JWT_SECRET=not-logged-test-value",
	}, "\n"))

	values, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	port, err := values.Int("MYSQL_PORT")
	if err != nil || port != 3306 {
		t.Fatalf("Int() = %d, %v", port, err)
	}

	endpoints, err := values.CSV("ETCD_ENDPOINTS")
	if err != nil || len(endpoints) != 2 {
		t.Fatalf("CSV() = %#v, %v", endpoints, err)
	}

	timeout, err := values.Duration("REQUEST_TIMEOUT")
	if err != nil || timeout != 5*time.Second {
		t.Fatalf("Duration() = %v, %v", timeout, err)
	}
}

func TestLoadFileRejectsDuplicateWithoutLeakingValue(t *testing.T) {
	secret := "sensitive-test-value"
	path := writeConfig(t, "JWT_SECRET="+secret+"\nJWT_SECRET="+secret+"\n")

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("LoadFile() error = nil, want duplicate-key error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("LoadFile() error leaked the configured value")
	}
}

func TestTypedErrorsDoNotLeakInvalidValue(t *testing.T) {
	secret := "sensitive-invalid-port"
	path := writeConfig(t, "MYSQL_PORT="+secret+"\n")
	values, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	_, err = values.Int("MYSQL_PORT")
	if err == nil {
		t.Fatal("Int() error = nil, want parse error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("Int() error leaked the configured value")
	}
}

func TestRequiredRejectsPlaceholder(t *testing.T) {
	path := writeConfig(t, "JWT_SECRET=CHANGE_ME\n")
	values, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if _, err := values.Required("JWT_SECRET"); err == nil {
		t.Fatal("Required() error = nil, want placeholder rejection")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.test")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
