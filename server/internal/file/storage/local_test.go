package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalCommitOpenDedupAndTraversal(t *testing.T) {
	local, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, _ := local.CreateTemp(context.Background())
	_, _ = first.Write([]byte("payload"))
	created, err := local.Commit(context.Background(), first, "objects/aa/bb/hash")
	if err != nil || !created {
		t.Fatalf("Commit created=%t err=%v", created, err)
	}
	second, _ := local.CreateTemp(context.Background())
	_, _ = second.Write([]byte("payload"))
	created, err = local.Commit(context.Background(), second, "objects/aa/bb/hash")
	if err != nil || created {
		t.Fatalf("dedup commit created=%t err=%v", created, err)
	}
	object, err := local.Open(context.Background(), "objects/aa/bb/hash")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	payload, err := io.ReadAll(object)
	if err != nil || string(payload) != "payload" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	if _, err := local.Open(context.Background(), "../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestLocalAbortRemovesTemp(t *testing.T) {
	local, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	temp, _ := local.CreateTemp(context.Background())
	if err := local.Abort(temp); err != nil {
		t.Fatal(err)
	}
}

func TestLocalStartupCleansOnlyStaleUploadTemps(t *testing.T) {
	root := t.TempDir()
	tempDir := filepath.Join(root, "temp")
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(tempDir, "upload-stale.part")
	keep := filepath.Join(tempDir, "unrelated.keep")
	if err := os.WriteFile(stale, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLocal(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temp remains: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
}
