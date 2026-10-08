package migrations

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedMigrationsAreOrderedAndPaired(t *testing.T) {
	migrations, err := load(migrationFiles)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	wantNames := []string{"identity", "friend", "chat", "group", "file", "settings"}
	if len(migrations) != len(wantNames) {
		t.Fatalf("migration count = %d", len(migrations))
	}
	for index, migration := range migrations {
		if migration.Version != uint64(index+1) || migration.Name != wantNames[index] || migration.UpSQL == "" || migration.DownSQL == "" {
			t.Fatalf("migration[%d] = %#v", index, migration)
		}
	}
}

func TestLoadRejectsVersionGap(t *testing.T) {
	_, err := load(fstest.MapFS{
		"002_friend.up.sql":   {Data: []byte("SELECT 1")},
		"002_friend.down.sql": {Data: []byte("SELECT 1")},
	})
	if err == nil || !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("version gap error = %v", err)
	}
}

func TestLoadRejectsMissingPairAndConflictingName(t *testing.T) {
	_, err := load(fstest.MapFS{"001_identity.up.sql": {Data: []byte("SELECT 1")}})
	if err == nil || !strings.Contains(err.Error(), "up and down") {
		t.Fatalf("missing pair error = %v", err)
	}
	_, err = load(fstest.MapFS{
		"001_identity.up.sql":    {Data: []byte("SELECT 1")},
		"001_different.down.sql": {Data: []byte("SELECT 1")},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting name error = %v", err)
	}
}

func TestParseFilenameAndSafeErrors(t *testing.T) {
	version, name, direction, ok := parseFilename("006_settings.down.sql")
	if !ok || version != 6 || name != "settings" || direction != "down" {
		t.Fatalf("parseFilename() = %d %q %q %t", version, name, direction, ok)
	}
	if _, _, _, ok := parseFilename("bad.sql"); ok {
		t.Fatal("invalid filename accepted")
	}
	private := errors.New("password=private-test-value")
	err := operationError("apply 001_identity up", private)
	if strings.Contains(err.Error(), private.Error()) || !errors.Is(err, private) {
		t.Fatalf("runner error safety = %v", err)
	}
}

func TestNewRejectsNilDatabase(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil")
	}
}
