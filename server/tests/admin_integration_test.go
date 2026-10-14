package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	chatadmin "github.com/kanhai447/GIM/server/internal/chat/message/admin"
	chatadminmysql "github.com/kanhai447/GIM/server/internal/chat/message/admin/mysql"
	fileadmin "github.com/kanhai447/GIM/server/internal/file/admin"
	fileadminmysql "github.com/kanhai447/GIM/server/internal/file/admin/mysql"
	groupadmin "github.com/kanhai447/GIM/server/internal/group/admin"
	groupadminmysql "github.com/kanhai447/GIM/server/internal/group/admin/mysql"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/settings/domain"
	"github.com/kanhai447/GIM/server/internal/settings/overview"
	overviewmysql "github.com/kanhai447/GIM/server/internal/settings/overview/mysql"
	settingsmysql "github.com/kanhai447/GIM/server/internal/settings/repository/mysql"
	settingsservice "github.com/kanhai447/GIM/server/internal/settings/service"
	useradmin "github.com/kanhai447/GIM/server/internal/user/admin"
	useradminmysql "github.com/kanhai447/GIM/server/internal/user/admin/mysql"
	userdomain "github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/kanhai447/GIM/server/migrations"
	"gorm.io/gorm"
)

var adminDatabasePattern = regexp.MustCompile(`^gim_admin_test_[0-9]+_[0-9]+$`)

func TestAdminRepositoriesAndManagementActions(t *testing.T) {
	gormDB, rawDB, cleanup := openAdminTestDatabase(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	seedAdminFixtures(t, ctx, rawDB)
	assertAdminExplainPlans(t, ctx, rawDB)
	summary, err := overview.New(overviewmysql.New(gormDB)).Get(ctx)
	if err != nil || summary.UsersTotal != 3 || summary.UsersActive != 3 || summary.PrivateMessages != 1 || summary.GroupsActive != 1 || summary.GroupMessages != 1 || summary.UserFiles != 1 {
		t.Fatalf("admin overview = %#v, err=%v", summary, err)
	}

	users := useradmin.New(useradminmysql.New(gormDB), nil)
	userPage, err := users.List(ctx, useradmin.Query{Limit: 20})
	if err != nil || len(userPage.Items) != 3 || userPage.Items[1].SendMessageCount != 1 || userPage.Items[1].GroupCount != 1 {
		t.Fatalf("admin users = %#v, err=%v", userPage, err)
	}
	if err := users.UpdateRestrictions(ctx, useradmin.Restrictions{UserID: 2, CurtailChat: true, CurtailCreateGroup: true}); err != nil {
		t.Fatalf("update restrictions: %v", err)
	}
	if err := users.Disable(ctx, 1, 2); err != nil {
		t.Fatalf("disable member: %v", err)
	}
	disabled, err := users.List(ctx, useradmin.Query{Limit: 20, Status: userdomain.StatusDisabled})
	if err != nil || len(disabled.Items) != 1 || disabled.Items[0].ID != 2 {
		t.Fatalf("disabled users = %#v, err=%v", disabled, err)
	}
	if err := users.Enable(ctx, 1, 2); err != nil {
		t.Fatalf("enable member: %v", err)
	}
	enabled, err := users.List(ctx, useradmin.Query{Limit: 20, Status: userdomain.StatusActive})
	if err != nil || len(enabled.Items) != 3 {
		t.Fatalf("enabled users = %#v, err=%v", enabled, err)
	}
	if err := users.Disable(ctx, 1, 1); err != useradmin.ErrSelfAction {
		t.Fatalf("disable current admin error = %v", err)
	}

	chats := chatadmin.New(chatadminmysql.New(gormDB))
	chatPage, err := chats.List(ctx, chatadmin.Query{Limit: 20})
	if err != nil || len(chatPage.Items) != 1 || string(chatPage.Items[0].Payload) == "" {
		t.Fatalf("admin chats = %#v err=%v", chatPage, err)
	}
	if err := chats.Delete(ctx, chatPage.Items[0].ID); err != nil {
		t.Fatalf("delete chat message: %v", err)
	}
	assertTombstoned(t, ctx, rawDB, "chat_messages", chatPage.Items[0].ID)

	groups := groupadmin.New(groupadminmysql.New(gormDB), nil)
	groupPage, err := groups.List(ctx, groupadmin.Query{Limit: 20})
	if err != nil || len(groupPage.Items) != 1 || groupPage.Items[0].MessageCount != 1 || groupPage.Items[0].MemberCount != 2 {
		t.Fatalf("admin groups = %#v err=%v", groupPage, err)
	}
	messagePage, err := groups.ListMessages(ctx, groupadmin.MessageQuery{GroupID: 10, Limit: 20})
	if err != nil || len(messagePage.Items) != 1 {
		t.Fatalf("admin group messages = %#v err=%v", messagePage, err)
	}
	if err := groups.DeleteMessage(ctx, messagePage.Items[0].ID); err != nil {
		t.Fatalf("delete group message: %v", err)
	}
	assertTombstoned(t, ctx, rawDB, "group_messages", messagePage.Items[0].ID)
	if err := groups.Dissolve(ctx, 10); err != nil {
		t.Fatalf("dissolve group: %v", err)
	}

	files := fileadmin.New(fileadminmysql.New(gormDB))
	filePage, err := files.List(ctx, fileadmin.Query{Limit: 20})
	if err != nil || len(filePage.Items) != 1 || filePage.Items[0].Metadata.Object.StoragePath == "" {
		t.Fatalf("admin files = %#v err=%v", filePage, err)
	}
	publicFiles, err := json.Marshal(filePage)
	if err != nil || string(publicFiles) == "" || regexp.MustCompile(`storagePath|objects/aa/test`).Match(publicFiles) {
		t.Fatalf("admin file response leaked physical storage metadata: err=%v", err)
	}
	if err := files.Delete(ctx, filePage.Items[0].ID); err != nil {
		t.Fatalf("delete file reference: %v", err)
	}

	settings := settingsservice.New(settingsmysql.New(gormDB))
	updated, err := settings.Update(ctx, domain.Info{Site: domain.Site{Name: "GIM Admin", Version: "v1", Notice: "safe", RegistrationEnabled: true}, OpenLogin: domain.OpenLogin{Enabled: false}})
	if err != nil || updated.Site.Name != "GIM Admin" {
		t.Fatalf("update settings = %#v err=%v", updated, err)
	}
	loaded, err := settings.Get(ctx)
	if err != nil || loaded.Site.Notice != "safe" {
		t.Fatalf("load settings = %#v err=%v", loaded, err)
	}
}

func openAdminTestDatabase(t *testing.T) (*gorm.DB, *sql.DB, func()) {
	t.Helper()
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Admin integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	base, err := platformmysql.FromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gim_admin_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	if !adminDatabasePattern.MatchString(name) {
		t.Fatal("unsafe test database name")
	}
	adminConfig := base
	adminConfig.Database = ""
	adminDB, err := platformmysql.OpenSQL(context.Background(), adminConfig, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminDB.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		_ = adminDB.Close()
		t.Fatal(err)
	}
	testConfig := base
	testConfig.Database = name
	rawDB, err := platformmysql.OpenSQL(context.Background(), testConfig, true)
	if err != nil {
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		_ = adminDB.Close()
		t.Fatal(err)
	}
	client, err := platformmysql.Open(context.Background(), testConfig)
	if err != nil {
		_ = rawDB.Close()
		_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		_ = adminDB.Close()
		t.Fatal(err)
	}
	runner, err := migrations.New(rawDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_ = client.Close()
		_ = rawDB.Close()
		if adminDatabasePattern.MatchString(name) {
			_, _ = adminDB.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		}
		_ = adminDB.Close()
	}
	return client.DB(), rawDB, cleanup
}

func seedAdminFixtures(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, row := range []struct {
		id                int
		account, nickname string
		role              int
	}{{1, "admin.cp2", "Admin", 1}, {2, "member.cp2", "Member", 2}, {3, "peer.cp2", "Peer", 2}} {
		mustExec(t, ctx, db, `INSERT INTO users(id,account,pwd_hash,nickname,role,status) VALUES (?,?,?, ?,?,1)`, row.id, row.account, "integration-hash-not-plaintext", row.nickname, row.role)
	}
	mustExec(t, ctx, db, `INSERT INTO user_confs(user_id) VALUES(2)`)
	mustExec(t, ctx, db, `INSERT INTO chat_messages(id,send_user_id,rev_user_id,client_msg_id,msg_type,msg_preview,msg) VALUES(100,2,3,'admin-chat-1',1,'hello',JSON_OBJECT('type',1,'textMsg',JSON_OBJECT('content','hello')))`)
	mustExec(t, ctx, db, `INSERT INTO chat_sessions(user_id,peer_user_id,last_message_id,last_message_at,unread_count) VALUES(2,3,100,CURRENT_TIMESTAMP(3),0),(3,2,100,CURRENT_TIMESTAMP(3),1)`)
	mustExec(t, ctx, db, "INSERT INTO `groups`(id,name,owner_id) VALUES(10,'Admin Group',2)")
	mustExec(t, ctx, db, `INSERT INTO group_members(id,group_id,user_id,role) VALUES(20,10,2,1),(21,10,3,3)`)
	mustExec(t, ctx, db, `INSERT INTO group_messages(id,group_id,send_user_id,group_member_id,client_msg_id,msg_type,msg_preview,msg) VALUES(200,10,2,20,'admin-group-1',1,'group hello',JSON_OBJECT('type',1,'textMsg',JSON_OBJECT('content','group hello')))`)
	mustExec(t, ctx, db, `INSERT INTO group_sessions(user_id,group_id,last_message_id,last_message_at,unread_count) VALUES(2,10,200,CURRENT_TIMESTAMP(3),0),(3,10,200,CURRENT_TIMESTAMP(3),1)`)
	mustExec(t, ctx, db, `INSERT INTO file_objects(id,uid,sha256,size,storage_path,mime_type) VALUES(30,'00000000-0000-4000-8000-000000000030',REPEAT('a',64),4,'objects/aa/test','text/plain')`)
	mustExec(t, ctx, db, `INSERT INTO user_files(id,user_id,file_object_id,original_name) VALUES(40,2,30,'safe.txt')`)
}

func assertTombstoned(t *testing.T, ctx context.Context, db *sql.DB, table string, id uint64) {
	t.Helper()
	var deleted sql.NullTime
	query := "SELECT admin_deleted_at FROM `" + table + "` WHERE id = ?"
	if err := db.QueryRowContext(ctx, query, id).Scan(&deleted); err != nil || !deleted.Valid {
		t.Fatalf("%s %d tombstone valid=%t err=%v", table, id, deleted.Valid, err)
	}
}

func assertAdminExplainPlans(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, test := range []struct {
		name  string
		index string
		query string
	}{
		{name: "user list/search", index: "PRIMARY", query: `SELECT id FROM users WHERE id < 1000 AND status = 1 AND (account LIKE '%member%' OR nickname LIKE '%member%') ORDER BY id DESC LIMIT 50`},
		{name: "private chat admin history", index: "PRIMARY", query: `SELECT id FROM chat_messages WHERE id < 1000 AND admin_deleted_at IS NULL ORDER BY id DESC LIMIT 50`},
		{name: "group list", index: "PRIMARY", query: "SELECT id FROM `groups` WHERE id < 1000 AND status = 1 ORDER BY id DESC LIMIT 30"},
		{name: "file list", index: "PRIMARY", query: `SELECT id FROM user_files WHERE id < 1000 ORDER BY id DESC LIMIT 50`},
		{name: "audit log filter", index: "ix_audit_logs_type_occurred", query: `SELECT id FROM audit_logs WHERE event_type = 'auth.login.success' ORDER BY occurred_at DESC, id DESC LIMIT 50`},
	} {
		plan := explainIndex(t, ctx, db, test.query)
		if !strings.Contains(plan, test.index) {
			t.Fatalf("Admin EXPLAIN %s did not expose %s: %s", test.name, test.index, plan)
		}
		t.Logf("Admin EXPLAIN %-28s index=%s plan=%s", test.name, test.index, plan)
	}
	for _, test := range []struct{ name, query string }{
		{name: "overview users", query: `SELECT COUNT(*) FROM users WHERE status = 1`},
		{name: "overview private messages", query: `SELECT COUNT(*) FROM chat_messages WHERE admin_deleted_at IS NULL`},
		{name: "overview groups", query: "SELECT COUNT(*) FROM `groups` WHERE status = 1"},
		{name: "overview files", query: `SELECT COUNT(*) FROM user_files`},
	} {
		t.Logf("Admin EXPLAIN %-28s plan=%s", test.name, explainIndex(t, ctx, db, test.query))
	}
}
