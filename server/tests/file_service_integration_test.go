package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	filemodule "github.com/kanhai447/GIM/server/internal/file"
	"github.com/kanhai447/GIM/server/internal/file/domain"
	filerepo "github.com/kanhai447/GIM/server/internal/file/repository"
	filemysql "github.com/kanhai447/GIM/server/internal/file/repository/mysql"
	filestorage "github.com/kanhai447/GIM/server/internal/file/storage"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

func TestFileServiceMySQLFilesystemIntegration(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run file integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	db, raw := newChatMessageDatabase(t, values)
	root := t.TempDir()
	module, err := filemodule.New(db, root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("GIM file service concurrent dedup payload")
	const concurrency = 50
	results := make(chan uint64, concurrency)
	failures := make(chan error, concurrency)
	var group sync.WaitGroup
	for range concurrency {
		group.Add(1)
		go func() {
			defer group.Done()
			result, uploadErr := module.Service.Upload(context.Background(), 101, "report.txt", bytes.NewReader(content))
			if uploadErr != nil {
				failures <- uploadErr
				return
			}
			results <- result.Metadata.UserFile.ID
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for failure := range failures {
		t.Fatalf("concurrent upload: %v", failure)
	}
	var firstID uint64
	for id := range results {
		if firstID == 0 {
			firstID = id
		}
		if id != firstID {
			t.Fatalf("same upload returned ids %d/%d", firstID, id)
		}
	}
	assertTableCount(t, raw, "file_objects", 1)
	assertTableCount(t, raw, "user_files", 1)
	renamed, err := module.Service.Upload(context.Background(), 101, "renamed.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	shared, err := module.Service.Upload(context.Background(), 202, "report.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	assertTableCount(t, raw, "file_objects", 1)
	assertTableCount(t, raw, "user_files", 3)
	if _, err := module.Service.Get(context.Background(), 202, fmt.Sprint(firstID)); err == nil {
		t.Fatal("user B accessed user A reference")
	}
	owned, err := module.Service.Get(context.Background(), 202, fmt.Sprint(shared.Metadata.UserFile.ID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.Service.Get(context.Background(), 202, shared.Metadata.Object.UID); err != nil {
		t.Fatalf("owned UUID lookup: %v", err)
	}
	if _, err := module.Service.Get(context.Background(), 303, shared.Metadata.Object.UID); err == nil {
		t.Fatal("foreign UUID lookup succeeded")
	}
	object, err := module.Service.Open(context.Background(), owned)
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err := io.ReadAll(object)
	_ = object.Close()
	if err != nil || !bytes.Equal(downloaded, content) {
		t.Fatalf("download integrity err=%v payload=%q", err, downloaded)
	}
	if err := module.Service.Delete(context.Background(), 101, firstID); err != nil {
		t.Fatal(err)
	}
	if _, err := module.Service.Get(context.Background(), 101, fmt.Sprint(firstID)); err == nil {
		t.Fatal("deleted reference remained accessible")
	}
	if _, err := module.Service.Get(context.Background(), 202, fmt.Sprint(shared.Metadata.UserFile.ID)); err != nil {
		t.Fatalf("shared object lost after A delete: %v", err)
	}
	assertTableCount(t, raw, "file_objects", 1)
	assertTableCount(t, raw, "user_files", 2)
	items, err := module.Service.List(context.Background(), 101, 0, 20)
	if err != nil || len(items) != 1 || items[0].UserFile.ID != renamed.Metadata.UserFile.ID {
		t.Fatalf("list=%+v err=%v", items, err)
	}
	restarted, err := filemodule.New(db, root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Service.Get(context.Background(), 202, fmt.Sprint(shared.Metadata.UserFile.ID)); err != nil {
		t.Fatalf("restart persistence: %v", err)
	}
	html, err := module.Service.Upload(context.Background(), 101, "page.bin", bytes.NewReader([]byte("<!doctype html><html></html>")))
	if err != nil {
		t.Fatal(err)
	}
	if html.Metadata.Object.MIMEType != "text/html; charset=utf-8" {
		t.Fatalf("server MIME=%q", html.Metadata.Object.MIMEType)
	}
	empty, err := module.Service.Upload(context.Background(), 101, "empty.dat", bytes.NewReader(nil))
	if err != nil || empty.Metadata.Object.Size != 0 {
		t.Fatalf("empty upload=%+v err=%v", empty, err)
	}
	assertFileExplain(t, raw)
	local, err := filestorage.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Delete(context.Background(), shared.Metadata.Object.StoragePath); err != nil {
		t.Fatal(err)
	}
	if _, err := module.Service.Open(context.Background(), shared.Metadata); err == nil {
		t.Fatal("missing physical object opened")
	}
	if err := raw.QueryRow(`SELECT COUNT(*) FROM file_objects WHERE sha256 = ?`, shared.Metadata.Object.SHA256).Scan(new(int)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DROP TABLE user_files`); err != nil {
		t.Fatal(err)
	}
	failurePayload := []byte("database attach failure compensation")
	if _, err := module.Service.Upload(context.Background(), 303, "failure.bin", bytes.NewReader(failurePayload)); err == nil {
		t.Fatal("upload succeeded after user_files drop")
	}
	hash := sha256.Sum256(failurePayload)
	encoded := hex.EncodeToString(hash[:])
	failurePath := filepath.Join(root, "objects", encoded[:2], encoded[2:4], encoded)
	if _, err := os.Stat(failurePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("compensation left object: %v", err)
	}
	tempEntries, err := os.ReadDir(filepath.Join(root, "temp"))
	if err != nil || len(tempEntries) != 0 {
		t.Fatalf("temp entries=%d err=%v", len(tempEntries), err)
	}
}

func TestFileRepositoryOwnershipAndReferenceCount(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run file integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	db, _ := newChatMessageDatabase(t, values)
	repo := filemysql.New(db)
	ctx := context.Background()
	object := domainObject("abc", 64)
	metadata, _, _, err := repo.Attach(ctx, object, domainUserFile(1, "one.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := repo.Attach(ctx, object, domainUserFile(2, "two.bin")); err != nil {
		t.Fatal(err)
	}
	count, err := repo.CountReferences(ctx, metadata.Object.ID)
	if err != nil || count != 2 {
		t.Fatalf("references=%d err=%v", count, err)
	}
	exists, err := repo.Exists(ctx, 2, metadata.UserFile.ID)
	if err != nil || exists {
		t.Fatalf("cross-owner Exists=%t err=%v", exists, err)
	}
	if _, err := repo.GetOwnedByID(ctx, 2, metadata.UserFile.ID); !errors.Is(err, filerepo.ErrNotFound) {
		t.Fatalf("cross-owner error=%v", err)
	}
}

func domainObject(hashSeed string, size int64) domain.FileObject {
	sum := sha256.Sum256([]byte(hashSeed))
	hash := hex.EncodeToString(sum[:])
	return domain.FileObject{UID: "11111111-1111-4111-8111-111111111111", SHA256: hash, Size: size, StoragePath: filepath.ToSlash(filepath.Join("objects", hash[:2], hash[2:4], hash)), MIMEType: "application/octet-stream"}
}
func domainUserFile(userID uint64, name string) domain.UserFile {
	return domain.UserFile{UserID: userID, OriginalName: name}
}

func assertTableCount(t *testing.T, db interface{ QueryRow(string, ...any) *sql.Row }, table string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != want {
		t.Fatalf("%s count=%d want=%d err=%v", table, count, want, err)
	}
}

func assertFileExplain(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, query := range []string{`EXPLAIN SELECT * FROM file_objects WHERE sha256 = REPEAT('a',64)`, `EXPLAIN SELECT * FROM user_files WHERE user_id = 1 ORDER BY created_at DESC,id DESC LIMIT 20`, `EXPLAIN SELECT * FROM user_files WHERE id = 1 AND user_id = 1`} {
		rows, err := db.Query(query)
		if err != nil {
			t.Fatalf("EXPLAIN: %v", err)
		}
		if !rows.Next() {
			_ = rows.Close()
			t.Fatal("EXPLAIN returned no rows")
		}
		_ = rows.Close()
	}
}
