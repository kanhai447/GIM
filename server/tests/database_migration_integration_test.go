package tests

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kanhai447/GIM/server/internal/auth/credential"
	authservice "github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	"github.com/kanhai447/GIM/server/internal/auth/userclient"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	usermysql "github.com/kanhai447/GIM/server/internal/user/repository/mysql"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	"github.com/kanhai447/GIM/server/migrations"
)

const migrationCount = 6

var migrationDatabasePattern = regexp.MustCompile(`^gim_migration_test_[0-9]+_[0-9]+$`)

func TestDatabaseMigrationsAndUserAuthRegression(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run database migration integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	baseConfig, err := platformmysql.FromValues(values)
	if err != nil {
		t.Fatalf("build MySQL configuration: %v", err)
	}

	testDatabase := fmt.Sprintf("gim_migration_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if !migrationDatabasePattern.MatchString(testDatabase) {
		t.Fatal("generated migration database name is unsafe")
	}
	adminConfig := baseConfig
	adminConfig.Database = ""
	adminContext, adminCancel := context.WithTimeout(context.Background(), 10*time.Second)
	adminDB, err := platformmysql.OpenSQL(adminContext, adminConfig, false)
	adminCancel()
	if err != nil {
		t.Fatalf("open MySQL administration connection: %v", err)
	}
	if _, err := adminDB.Exec("CREATE DATABASE `" + testDatabase + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create isolated migration database: %v", err)
	}

	testConfig := baseConfig
	testConfig.Database = testDatabase
	testDB, err := platformmysql.OpenSQL(context.Background(), testConfig, true)
	if err != nil {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + testDatabase + "`")
		_ = adminDB.Close()
		t.Fatalf("open isolated migration database: %v", err)
	}
	t.Cleanup(func() {
		_ = testDB.Close()
		if migrationDatabasePattern.MatchString(testDatabase) {
			_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + testDatabase + "`")
		}
		_ = adminDB.Close()
	})

	runner, err := migrations.New(testDB)
	if err != nil {
		t.Fatalf("create migration runner: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate empty database up: %v", err)
	}
	assertMigrationStatus(t, ctx, runner, migrationCount)
	assertSchema(t, ctx, testDB)
	seedAndAssertConstraints(t, ctx, testDB)
	assertExplainPlans(t, ctx, testDB)
	assertUserAuthRegression(t, ctx, testConfig)

	if err := runner.Up(ctx); err != nil {
		t.Fatalf("idempotent migration up: %v", err)
	}
	assertMigrationStatus(t, ctx, runner, migrationCount)

	if err := runner.DownAll(ctx); err != nil {
		t.Fatalf("migrate database down: %v", err)
	}
	assertBusinessTablesAbsent(t, ctx, testDB)
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("migrate database up after down: %v", err)
	}
	assertMigrationStatus(t, ctx, runner, migrationCount)
	assertSchema(t, ctx, testDB)
}

func assertMigrationStatus(t *testing.T, ctx context.Context, runner *migrations.Runner, wantApplied int) {
	t.Helper()
	statuses, err := runner.Status(ctx)
	if err != nil {
		t.Fatalf("read migration status: %v", err)
	}
	if len(statuses) != migrationCount {
		t.Fatalf("migration status count = %d, want %d", len(statuses), migrationCount)
	}
	applied := 0
	for _, status := range statuses {
		if status.Applied {
			applied++
		}
	}
	if applied != wantApplied {
		t.Fatalf("applied migration count = %d, want %d", applied, wantApplied)
	}
}

func assertSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	tables := []string{
		"users", "user_confs", "friends", "friend_verifies", "chat_messages", "chat_sessions",
		"chat_message_deletions", "groups", "group_members", "group_verifies", "group_messages",
		"group_sessions", "group_message_deletions", "file_objects", "user_files", "settings",
	}
	for _, table := range tables {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s unavailable: count=%d err=%v", table, count, err)
		}
		var collation string
		if err := db.QueryRowContext(ctx, `SELECT table_collation FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&collation); err != nil || collation != "utf8mb4_0900_ai_ci" {
			t.Fatalf("table %s collation=%q err=%v", table, collation, err)
		}
	}

	var plaintextColumns int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'users' AND column_name IN ('password', 'pwd')`).Scan(&plaintextColumns); err != nil || plaintextColumns != 0 {
		t.Fatalf("users plaintext credential columns = %d err=%v", plaintextColumns, err)
	}
	assertIndexExists(t, ctx, db, "users", "ux_users_account")
	assertIndexExists(t, ctx, db, "chat_messages", "ux_chat_messages_sender_client")
	assertIndexExists(t, ctx, db, "chat_sessions", "ux_chat_sessions_user_peer")
	assertIndexExists(t, ctx, db, "group_members", "ux_group_members_group_user")
	assertIndexExists(t, ctx, db, "group_messages", "ux_group_messages_sender_client")
	assertIndexExists(t, ctx, db, "group_sessions", "ux_group_sessions_user_group")
	assertIndexExists(t, ctx, db, "file_objects", "ux_file_objects_sha256")
	assertColumn(t, ctx, db, "users", "pwd_hash", columnExpectation{dataType: "varchar", columnType: "varchar(255)", nullable: false, length: 255})
	assertColumn(t, ctx, db, "users", "role", columnExpectation{dataType: "tinyint", columnType: "tinyint unsigned", nullable: false, defaultValue: "2"})
	assertColumn(t, ctx, db, "users", "status", columnExpectation{dataType: "tinyint", columnType: "tinyint unsigned", nullable: false, defaultValue: "1"})
	assertColumn(t, ctx, db, "chat_messages", "client_msg_id", columnExpectation{dataType: "varchar", columnType: "varchar(64)", nullable: false, length: 64})
	assertColumn(t, ctx, db, "chat_messages", "msg", columnExpectation{dataType: "json", columnType: "json", nullable: false})
	assertColumn(t, ctx, db, "chat_messages", "conversation_low_id", columnExpectation{dataType: "bigint", columnType: "bigint unsigned", nullable: true, extra: "STORED GENERATED"})
	assertColumn(t, ctx, db, "chat_sessions", "unread_count", columnExpectation{dataType: "int", columnType: "int unsigned", nullable: false, defaultValue: "0"})
	assertColumn(t, ctx, db, "chat_sessions", "is_top", columnExpectation{dataType: "tinyint", columnType: "tinyint(1)", nullable: false, defaultValue: "0"})
	assertColumn(t, ctx, db, "chat_sessions", "hidden_at", columnExpectation{dataType: "datetime", columnType: "datetime(3)", nullable: true})
	assertColumn(t, ctx, db, "group_messages", "client_msg_id", columnExpectation{dataType: "varchar", columnType: "varchar(64)", nullable: false, length: 64})
	assertColumn(t, ctx, db, "group_messages", "msg", columnExpectation{dataType: "json", columnType: "json", nullable: false})
	assertColumn(t, ctx, db, "group_sessions", "unread_count", columnExpectation{dataType: "int", columnType: "int unsigned", nullable: false, defaultValue: "0"})
	assertColumn(t, ctx, db, "file_objects", "sha256", columnExpectation{dataType: "char", columnType: "char(64)", nullable: false, length: 64})
	assertColumn(t, ctx, db, "settings", "setting_value", columnExpectation{dataType: "json", columnType: "json", nullable: false})

	var foreignKeys int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE()`).Scan(&foreignKeys); err != nil || foreignKeys != 0 {
		t.Fatalf("foreign key count = %d, want 0; err=%v", foreignKeys, err)
	}
}

type columnExpectation struct {
	dataType     string
	columnType   string
	nullable     bool
	defaultValue string
	length       int64
	extra        string
}

func assertColumn(t *testing.T, ctx context.Context, db *sql.DB, table, column string, want columnExpectation) {
	t.Helper()
	var dataType, columnType, nullable, extra string
	var defaultValue sql.NullString
	var length sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT data_type, column_type, is_nullable, column_default, character_maximum_length, extra FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&dataType, &columnType, &nullable, &defaultValue, &length, &extra)
	if err != nil {
		t.Fatalf("read column %s.%s: %v", table, column, err)
	}
	if dataType != want.dataType || columnType != want.columnType || (nullable == "YES") != want.nullable {
		t.Fatalf("column %s.%s metadata type=%s columnType=%s nullable=%s", table, column, dataType, columnType, nullable)
	}
	if want.defaultValue != "" && (!defaultValue.Valid || defaultValue.String != want.defaultValue) {
		t.Fatalf("column %s.%s default=%q valid=%t", table, column, defaultValue.String, defaultValue.Valid)
	}
	if want.length > 0 && (!length.Valid || length.Int64 != want.length) {
		t.Fatalf("column %s.%s length=%d valid=%t", table, column, length.Int64, length.Valid)
	}
	if want.extra != "" && !strings.Contains(extra, want.extra) {
		t.Fatalf("column %s.%s extra=%q", table, column, extra)
	}
}

func assertIndexExists(t *testing.T, ctx context.Context, db *sql.DB, table, index string) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, table, index).Scan(&count); err != nil || count == 0 {
		t.Fatalf("index %s.%s unavailable: count=%d err=%v", table, index, count, err)
	}
}

func seedAndAssertConstraints(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for id, account := range []string{"schema.alice", "schema.bob", "schema.carol"} {
		_, err := db.ExecContext(ctx, `INSERT INTO users(id, account, pwd_hash, nickname, role, status) VALUES (?, ?, ?, ?, 2, 1)`, id+1, account, "integration-hash-value-not-plaintext", account)
		if err != nil {
			t.Fatalf("seed user %d: %v", id+1, err)
		}
	}
	assertDuplicateExec(t, ctx, db, "users account", `INSERT INTO users(account, pwd_hash, nickname, role, status) VALUES ('schema.alice', 'another-integration-hash', 'duplicate', 2, 1)`)

	mustExec(t, ctx, db, `INSERT INTO friends(user_id, friend_id) VALUES (1, 2)`)
	assertDuplicateExec(t, ctx, db, "friend direction", `INSERT INTO friends(user_id, friend_id) VALUES (1, 2)`)
	mustExec(t, ctx, db, `INSERT INTO friend_verifies(requester_id, receiver_id, status) VALUES (1, 3, 0)`)
	assertDuplicateExec(t, ctx, db, "pending friend pair", `INSERT INTO friend_verifies(requester_id, receiver_id, status) VALUES (3, 1, 0)`)

	mustExec(t, ctx, db, `INSERT INTO chat_messages(send_user_id, rev_user_id, client_msg_id, msg_type, msg_preview, msg) VALUES (1, 2, 'private-1', 1, 'hello', JSON_OBJECT('text', 'hello'))`)
	assertDuplicateExec(t, ctx, db, "private message idempotency", `INSERT INTO chat_messages(send_user_id, rev_user_id, client_msg_id, msg_type, msg) VALUES (1, 3, 'private-1', 1, JSON_OBJECT('text', 'duplicate'))`)
	mustExec(t, ctx, db, `INSERT INTO chat_sessions(user_id, peer_user_id, last_message_id, last_message_at) VALUES (1, 2, 1, CURRENT_TIMESTAMP(3))`)
	assertDuplicateExec(t, ctx, db, "private session", `INSERT INTO chat_sessions(user_id, peer_user_id) VALUES (1, 2)`)

	mustExec(t, ctx, db, "INSERT INTO `groups`(id, name, owner_id) VALUES (1, 'schema group', 1)")
	mustExec(t, ctx, db, `INSERT INTO group_members(group_id, user_id, role) VALUES (1, 1, 1)`)
	assertDuplicateExec(t, ctx, db, "group member", `INSERT INTO group_members(group_id, user_id, role) VALUES (1, 1, 3)`)
	mustExec(t, ctx, db, `INSERT INTO group_messages(group_id, send_user_id, group_member_id, client_msg_id, msg_type, msg_preview, msg) VALUES (1, 1, 1, 'group-1', 1, 'hello', JSON_OBJECT('text', 'hello'))`)
	assertDuplicateExec(t, ctx, db, "group message idempotency", `INSERT INTO group_messages(group_id, send_user_id, group_member_id, client_msg_id, msg_type, msg) VALUES (1, 1, 1, 'group-1', 1, JSON_OBJECT('text', 'duplicate'))`)
	mustExec(t, ctx, db, `INSERT INTO group_sessions(user_id, group_id, last_message_id, last_message_at) VALUES (1, 1, 1, CURRENT_TIMESTAMP(3))`)
	assertDuplicateExec(t, ctx, db, "group session", `INSERT INTO group_sessions(user_id, group_id) VALUES (1, 1)`)

	sha := strings.Repeat("a", 64)
	mustExec(t, ctx, db, `INSERT INTO file_objects(id, uid, sha256, size, storage_path) VALUES (1, '00000000-0000-0000-0000-000000000001', ?, 12, 'objects/aa/test')`, sha)
	assertDuplicateExec(t, ctx, db, "file sha256", `INSERT INTO file_objects(uid, sha256, size, storage_path) VALUES ('00000000-0000-0000-0000-000000000002', ?, 13, 'objects/aa/test-duplicate')`, sha)
	mustExec(t, ctx, db, `INSERT INTO user_files(user_id, file_object_id, original_name) VALUES (1, 1, 'hello.txt')`)
}

func assertDuplicateExec(t *testing.T, ctx context.Context, db *sql.DB, label, query string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(ctx, query, args...)
	if err == nil {
		t.Fatalf("%s duplicate insert unexpectedly succeeded", label)
	}
	var mysqlError *mysqldriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		t.Fatalf("%s error is not MySQL duplicate-key error: %v", label, err)
	}
}

func mustExec(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		t.Fatalf("seed migration schema: %v", err)
	}
}

func assertExplainPlans(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	cases := []struct {
		name  string
		index string
		query string
	}{
		{"account lookup", "ux_users_account", `SELECT * FROM users WHERE account = 'schema.alice'`},
		{"pending friend requests", "ix_friend_verifies_receiver_status_created", `SELECT * FROM friend_verifies WHERE receiver_id = 3 AND status = 0 ORDER BY created_at DESC, id DESC LIMIT 20`},
		{"friend list", "ux_friends_user_friend", `SELECT * FROM friends WHERE user_id = 1 ORDER BY friend_id LIMIT 20`},
		{"private history", "ix_chat_messages_conversation_cursor", `SELECT * FROM chat_messages WHERE conversation_low_id = 1 AND conversation_high_id = 2 AND id < 1000 ORDER BY id DESC LIMIT 20`},
		{"private sessions", "ix_chat_sessions_user_sort", `SELECT * FROM chat_sessions WHERE user_id = 1 ORDER BY is_top DESC, last_message_at DESC, id DESC LIMIT 20`},
		{"group members", "ix_group_members_group_role_user", `SELECT * FROM group_members WHERE group_id = 1 ORDER BY role, user_id LIMIT 20`},
		{"my groups", "ix_group_members_user_group", `SELECT * FROM group_members WHERE user_id = 1 ORDER BY group_id LIMIT 20`},
		{"group history", "ix_group_messages_group_cursor", `SELECT * FROM group_messages WHERE group_id = 1 AND id < 1000 ORDER BY id DESC LIMIT 20`},
		{"group sessions", "ix_group_sessions_user_sort", `SELECT * FROM group_sessions WHERE user_id = 1 ORDER BY is_top DESC, last_message_at DESC, id DESC LIMIT 20`},
		{"user files", "ix_user_files_user_created", `SELECT * FROM user_files WHERE user_id = 1 ORDER BY created_at DESC, id DESC LIMIT 20`},
		{"file deduplication", "ux_file_objects_sha256", `SELECT * FROM file_objects WHERE sha256 = '` + strings.Repeat("a", 64) + `'`},
	}
	for _, testCase := range cases {
		plan := explainIndex(t, ctx, db, testCase.query)
		if !strings.Contains(plan, testCase.index) {
			t.Fatalf("EXPLAIN %s did not expose index %s: %s", testCase.name, testCase.index, plan)
		}
		t.Logf("EXPLAIN %-24s index=%s plan=%s", testCase.name, testCase.index, plan)
	}
}

func explainIndex(t *testing.T, ctx context.Context, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "EXPLAIN "+query)
	if err != nil {
		t.Fatalf("run EXPLAIN: %v", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("read EXPLAIN columns: %v", err)
	}
	values := make([]sql.RawBytes, len(columns))
	destinations := make([]any, len(columns))
	for index := range values {
		destinations[index] = &values[index]
	}
	var plans []string
	for rows.Next() {
		if err := rows.Scan(destinations...); err != nil {
			t.Fatalf("scan EXPLAIN row: %v", err)
		}
		parts := make([]string, 0, len(columns))
		for index, column := range columns {
			if len(values[index]) > 0 {
				parts = append(parts, column+"="+string(values[index]))
			}
		}
		plans = append(plans, strings.Join(parts, ","))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate EXPLAIN rows: %v", err)
	}
	return strings.Join(plans, ";")
}

func assertUserAuthRegression(t *testing.T, ctx context.Context, config platformmysql.Config) {
	t.Helper()
	mysqlClient, err := platformmysql.Open(ctx, config)
	if err != nil {
		t.Fatalf("open migrated database through GORM: %v", err)
	}
	defer func() {
		if err := mysqlClient.Close(); err != nil {
			t.Errorf("close migrated database GORM client: %v", err)
		}
	}()

	userService := userservice.New(usermysql.New(mysqlClient.DB()))
	rpcClient, stopRPC := startUserRPC(t, userService)
	defer stopRPC()
	tokenManager, err := token.NewManager(bytes.Repeat([]byte{0x7d}, 32), time.Hour)
	if err != nil {
		t.Fatalf("create integration token manager: %v", err)
	}
	authService := authservice.New(userclient.New(rpcClient), credential.NewPasswords(4), tokenManager, newMemoryRevocations(), authservice.DefaultAllowlist())
	registered, err := authService.Register(ctx, authservice.RegisterInput{
		Account: "migration.auth", Nickname: "Migration Auth", Password: "safe-test-pass", Repeat: "safe-test-pass",
	})
	if err != nil || registered.UserID == 0 || registered.Role != 2 {
		t.Fatalf("register through Auth/User on migrated database: userID=%d role=%d err=%v", registered.UserID, registered.Role, err)
	}
	loggedIn, err := authService.Login(ctx, authservice.LoginInput{Account: "migration.auth", Password: "safe-test-pass"})
	if err != nil || loggedIn.Token == "" || loggedIn.User.UserID != registered.UserID {
		t.Fatalf("login through Auth/User on migrated database: tokenSet=%t userID=%d err=%v", loggedIn.Token != "", loggedIn.User.UserID, err)
	}
	stored, err := userService.GetUserByID(ctx, registered.UserID)
	if err != nil || stored.Account != "migration.auth" || stored.PasswordHash == "safe-test-pass" || stored.RegisterSource != "account" {
		t.Fatalf("read registered user from migrated database: account=%q source=%q err=%v", stored.Account, stored.RegisterSource, err)
	}
}

func assertBusinessTablesAbsent(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name <> 'schema_migrations'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("business tables after down = %d, want 0; err=%v", count, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("schema_migrations rows after down = %d, want 0; err=%v", count, err)
	}
}
