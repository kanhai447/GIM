// Package migrations provides GIM's embedded, versioned MySQL schema runner.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed *.up.sql *.down.sql
var migrationFiles embed.FS

const migrationTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT UNSIGNED NOT NULL,
  name VARCHAR(128) NOT NULL,
  dirty TINYINT(1) NOT NULL DEFAULT 1,
  applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (version),
  CONSTRAINT ck_schema_migrations_dirty CHECK (dirty IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`

type Migration struct {
	Version uint64
	Name    string
	UpSQL   string
	DownSQL string
}

type Status struct {
	Version   uint64
	Name      string
	Applied   bool
	Dirty     bool
	AppliedAt *time.Time
}

type Runner struct {
	db         *sql.DB
	migrations []Migration
}

func New(db *sql.DB) (*Runner, error) {
	if db == nil {
		return nil, errors.New("migration database is not initialized")
	}
	loaded, err := load(migrationFiles)
	if err != nil {
		return nil, err
	}
	return &Runner{db: db, migrations: loaded}, nil
}

func (runner *Runner) Status(ctx context.Context) ([]Status, error) {
	if err := runner.ensureTable(ctx); err != nil {
		return nil, err
	}
	rows, err := runner.db.QueryContext(ctx, `SELECT version, name, dirty, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, operationError("read status", err)
	}
	defer rows.Close()
	type appliedMigration struct {
		name      string
		dirty     bool
		appliedAt time.Time
	}
	applied := make(map[uint64]appliedMigration)
	for rows.Next() {
		var version uint64
		var name string
		var dirty bool
		var appliedAt time.Time
		if err := rows.Scan(&version, &name, &dirty, &appliedAt); err != nil {
			return nil, operationError("scan status", err)
		}
		applied[version] = appliedMigration{name: name, dirty: dirty, appliedAt: appliedAt}
	}
	if err := rows.Err(); err != nil {
		return nil, operationError("read status", err)
	}
	for version, appliedState := range applied {
		migration, exists := runner.byVersion(version)
		if !exists {
			return nil, fmt.Errorf("database contains unknown migration version %03d", version)
		}
		if appliedState.name != migration.Name {
			return nil, fmt.Errorf("migration version %03d name does not match embedded schema", version)
		}
	}
	statuses := make([]Status, 0, len(runner.migrations))
	for _, migration := range runner.migrations {
		status := Status{Version: migration.Version, Name: migration.Name}
		if appliedState, exists := applied[migration.Version]; exists {
			status.Applied = true
			status.Dirty = appliedState.dirty
			status.AppliedAt = &appliedState.appliedAt
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (runner *Runner) Up(ctx context.Context) error {
	statuses, err := runner.Status(ctx)
	if err != nil {
		return err
	}
	for index, status := range statuses {
		if status.Applied {
			if status.Dirty {
				return fmt.Errorf("migration %03d_%s is dirty; run a reviewed down before retrying", status.Version, status.Name)
			}
			continue
		}
		if err := runner.applyUp(ctx, runner.migrations[index]); err != nil {
			return err
		}
	}
	return nil
}

func (runner *Runner) Down(ctx context.Context, steps int) error {
	if steps < 1 {
		return errors.New("migration down steps must be positive")
	}
	if err := runner.ensureTable(ctx); err != nil {
		return err
	}
	rows, err := runner.db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT ?`, steps)
	if err != nil {
		return operationError("read applied migrations", err)
	}
	var versions []uint64
	for rows.Next() {
		var version uint64
		if err := rows.Scan(&version); err != nil {
			_ = rows.Close()
			return operationError("scan applied migrations", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Close(); err != nil {
		return operationError("close applied migrations", err)
	}
	for _, version := range versions {
		migration, exists := runner.byVersion(version)
		if !exists {
			return fmt.Errorf("migration version %d has no embedded down script", version)
		}
		if err := runner.applyDown(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (runner *Runner) DownAll(ctx context.Context) error {
	if err := runner.ensureTable(ctx); err != nil {
		return err
	}
	var count int
	if err := runner.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		return operationError("count applied migrations", err)
	}
	if count == 0 {
		return nil
	}
	return runner.Down(ctx, count)
}

func (runner *Runner) applyUp(ctx context.Context, migration Migration) error {
	if _, err := runner.db.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, dirty) VALUES (?, ?, 1)`, migration.Version, migration.Name); err != nil {
		return operationError(fmt.Sprintf("record %03d_%s dirty state", migration.Version, migration.Name), err)
	}
	if _, err := runner.db.ExecContext(ctx, migration.UpSQL); err != nil {
		return operationError(fmt.Sprintf("apply %03d_%s up", migration.Version, migration.Name), err)
	}
	if _, err := runner.db.ExecContext(ctx, `UPDATE schema_migrations SET dirty = 0, applied_at = CURRENT_TIMESTAMP(3) WHERE version = ?`, migration.Version); err != nil {
		return operationError(fmt.Sprintf("complete %03d_%s", migration.Version, migration.Name), err)
	}
	return nil
}

func (runner *Runner) applyDown(ctx context.Context, migration Migration) error {
	if _, err := runner.db.ExecContext(ctx, migration.DownSQL); err != nil {
		return operationError(fmt.Sprintf("apply %03d_%s down", migration.Version, migration.Name), err)
	}
	if _, err := runner.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = ?`, migration.Version); err != nil {
		return operationError(fmt.Sprintf("unrecord %03d_%s", migration.Version, migration.Name), err)
	}
	return nil
}

func (runner *Runner) ensureTable(ctx context.Context) error {
	if _, err := runner.db.ExecContext(ctx, migrationTableSQL); err != nil {
		return operationError("initialize schema_migrations", err)
	}
	return nil
}

func (runner *Runner) byVersion(version uint64) (Migration, bool) {
	index := sort.Search(len(runner.migrations), func(index int) bool { return runner.migrations[index].Version >= version })
	if index == len(runner.migrations) || runner.migrations[index].Version != version {
		return Migration{}, false
	}
	return runner.migrations[index], true
}

type migrationPair struct {
	name string
	up   string
	down string
}

func load(files fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	pairs := make(map[uint64]migrationPair)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version, name, direction, ok := parseFilename(entry.Name())
		if !ok {
			continue
		}
		body, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		pair := pairs[version]
		if pair.name != "" && pair.name != name {
			return nil, fmt.Errorf("migration version %03d has conflicting names", version)
		}
		pair.name = name
		if direction == "up" {
			if pair.up != "" {
				return nil, fmt.Errorf("migration version %03d has duplicate up scripts", version)
			}
			pair.up = strings.TrimSpace(string(body))
		} else {
			if pair.down != "" {
				return nil, fmt.Errorf("migration version %03d has duplicate down scripts", version)
			}
			pair.down = strings.TrimSpace(string(body))
		}
		pairs[version] = pair
	}
	versions := make([]uint64, 0, len(pairs))
	for version := range pairs {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(left, right int) bool { return versions[left] < versions[right] })
	migrations := make([]Migration, 0, len(versions))
	for index, version := range versions {
		if version != uint64(index+1) {
			return nil, fmt.Errorf("migration sequence must be contiguous from 001; found %03d", version)
		}
		pair := pairs[version]
		if pair.up == "" || pair.down == "" {
			return nil, fmt.Errorf("migration %03d_%s must have up and down scripts", version, pair.name)
		}
		migrations = append(migrations, Migration{Version: version, Name: pair.name, UpSQL: pair.up, DownSQL: pair.down})
	}
	if len(migrations) == 0 {
		return nil, errors.New("no embedded migrations found")
	}
	return migrations, nil
}

func parseFilename(filename string) (uint64, string, string, bool) {
	parts := strings.Split(filename, ".")
	if len(parts) != 3 || parts[2] != "sql" || parts[1] != "up" && parts[1] != "down" {
		return 0, "", "", false
	}
	separator := strings.IndexByte(parts[0], '_')
	if separator != 3 || separator == len(parts[0])-1 {
		return 0, "", "", false
	}
	version, err := strconv.ParseUint(parts[0][:separator], 10, 64)
	if err != nil || version == 0 {
		return 0, "", "", false
	}
	name := parts[0][separator+1:]
	for _, character := range name {
		if character < 'a' || character > 'z' {
			if character < '0' || character > '9' {
				if character != '_' {
					return 0, "", "", false
				}
			}
		}
	}
	return version, name, parts[1], true
}

type runnerError struct {
	operation string
	cause     error
}

func (err *runnerError) Error() string { return "migration " + err.operation + " failed" }
func (err *runnerError) Unwrap() error { return err.cause }

func operationError(operation string, cause error) error {
	return &runnerError{operation: operation, cause: cause}
}
