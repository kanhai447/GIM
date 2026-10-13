package config

import (
	"os"
	"path/filepath"
	"testing"

	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFromValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.env")
	if err := os.WriteFile(path, []byte("FILE_STORAGE_ROOT=./storage/test\nFILE_MAX_UPLOAD_BYTES=1024\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values, err := platformconfig.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := FromValues(values)
	if err != nil || cfg.MaxUploadBytes != 1024 {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
}
