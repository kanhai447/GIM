package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func runPrepare(arguments []string) error {
	flags := flag.NewFlagSet("prepare", flag.ContinueOnError)
	baseURL := flags.String("base-url", "http://127.0.0.1:8080", "Gateway base URL")
	origin := flags.String("origin", "http://127.0.0.1:5173", "allowed WebSocket browser origin")
	usersCount := flags.Int("users", 40, "number of perf users")
	groupSizesText := flags.String("group-sizes", "3,5,10", "comma-separated group sizes")
	manifestPath := flags.String("manifest", "../.tmp/perf/manifest.json", "manifest output path")
	prefix := flags.String("prefix", "perf-user-"+time.Now().Format("20060102-150405"), "test account prefix")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *usersCount < 2 || *usersCount > 500 || !strings.HasPrefix(*prefix, "perf-user-") {
		return errors.New("users must be 2..500 and prefix must start with perf-user-")
	}
	groupSizes, err := parseGroupSizes(*groupSizesText, *usersCount)
	if err != nil {
		return err
	}
	password, err := requiredSecret(passwordEnv)
	if err != nil {
		return err
	}
	client, err := newAPIClient(*baseURL, 15*time.Second)
	if err != nil {
		return err
	}
	if _, err := newAPIClient(*origin, time.Second); err != nil {
		return fmt.Errorf("invalid websocket origin")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*usersCount)*5*time.Second+30*time.Second)
	defer cancel()
	value := manifest{SchemaVersion: 1, Prefix: *prefix, BaseURL: client.baseURL, Origin: strings.TrimRight(*origin, "/"), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	tokens := make(map[uint64]string, *usersCount)
	for index := 0; index < *usersCount; index++ {
		account := fmt.Sprintf("%s-%03d", *prefix, index+1)
		nickname := fmt.Sprintf("Perf %03d", index+1)
		envelope, status, requestErr := client.json(ctx, http.MethodPost, "/api/auth/register", "", map[string]string{"account": account, "nickname": nickname, "pwd": password, "rePwd": password})
		if requestErr != nil || status != http.StatusOK || envelope.Code != 0 {
			return fmt.Errorf("register perf user %d failed: http=%d code=%d", index+1, status, envelope.Code)
		}
		loggedIn, loginErr := client.login(ctx, account, password)
		if loginErr != nil {
			return fmt.Errorf("login perf user %d failed: %w", index+1, loginErr)
		}
		value.Users = append(value.Users, perfUser{ID: loggedIn.User.ID, Account: account})
		tokens[loggedIn.User.ID] = loggedIn.Token
	}
	db, err := openPerfDB()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := prepareFriendPairs(ctx, db, value.Users); err != nil {
		return err
	}
	for _, size := range groupSizes {
		memberIDs := make([]uint64, size)
		for index := range memberIDs {
			memberIDs[index] = value.Users[index].ID
		}
		input := map[string]any{"name": fmt.Sprintf("perf-group-%d-%s", size, *prefix), "size": size + 5, "isSearch": false, "joinPolicy": 0, "userIdList": memberIDs[1:]}
		envelope, status, requestErr := client.json(ctx, http.MethodPost, "/api/group/group", tokens[memberIDs[0]], input)
		if requestErr != nil || status != http.StatusOK || envelope.Code != 0 {
			return fmt.Errorf("create perf group size %d failed: http=%d code=%d", size, status, envelope.Code)
		}
		var created struct {
			GroupID uint64 `json:"groupId"`
		}
		if err := json.Unmarshal(envelope.Data, &created); err != nil || created.GroupID == 0 {
			return fmt.Errorf("create perf group size %d returned invalid data", size)
		}
		value.Groups = append(value.Groups, perfGroup{ID: created.GroupID, Size: size, MemberIDs: memberIDs})
	}
	if err := writeManifest(*manifestPath, value); err != nil {
		return err
	}
	return printJSON(map[string]any{"operation": "prepare", "prefix": value.Prefix, "users": len(value.Users), "groups": value.Groups, "manifest": *manifestPath})
}

func parseGroupSizes(input string, users int) ([]int, error) {
	seen := make(map[int]struct{})
	var result []int
	for _, part := range strings.Split(input, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 2 || value > users {
			return nil, errors.New("group sizes must be unique integers between 2 and users")
		}
		if _, exists := seen[value]; exists {
			return nil, errors.New("group sizes must be unique integers between 2 and users")
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func prepareFriendPairs(ctx context.Context, db *sql.DB, users []perfUser) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("begin friend fixture transaction failed")
	}
	defer tx.Rollback()
	for index := 0; index+1 < len(users); index += 2 {
		first, second := users[index].ID, users[index+1].ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO friends(user_id,friend_id) VALUES (?,?),(?,?)`, first, second, second, first); err != nil {
			return errors.New("create friend fixture failed")
		}
	}
	if err := tx.Commit(); err != nil {
		return errors.New("commit friend fixture failed")
	}
	return nil
}

func runCleanup(arguments []string) error {
	flags := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "../.tmp/perf/manifest.json", "manifest path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	value, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	db, err := openPerfDB()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	userIDs := make([]uint64, len(value.Users))
	for index := range value.Users {
		userIDs[index] = value.Users[index].ID
	}
	groupIDs := make([]uint64, len(value.Groups))
	for index := range value.Groups {
		groupIDs[index] = value.Groups[index].ID
	}
	if err := cleanupRows(ctx, db, userIDs, groupIDs); err != nil {
		return err
	}
	return printJSON(map[string]any{"operation": "cleanup", "prefix": value.Prefix, "usersRemoved": len(userIDs), "groupsRemoved": len(groupIDs), "physicalFileGC": "not implemented by V1; unreferenced content-addressed objects remain safely reusable"})
}

func cleanupRows(ctx context.Context, db *sql.DB, userIDs, groupIDs []uint64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("begin cleanup transaction failed")
	}
	defer tx.Rollback()
	usersClause, usersArgs := placeholders(userIDs)
	groupsClause, groupsArgs := placeholders(groupIDs)
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM chat_message_deletions WHERE user_id IN (` + usersClause + `) OR message_id IN (SELECT id FROM chat_messages WHERE send_user_id IN (` + usersClause + `) OR rev_user_id IN (` + usersClause + `))`, joinArgs(usersArgs, usersArgs, usersArgs)},
		{`DELETE FROM group_message_deletions WHERE user_id IN (` + usersClause + `) OR group_id IN (` + groupsClause + `)`, joinArgs(usersArgs, groupsArgs)},
		{`DELETE FROM user_files WHERE user_id IN (` + usersClause + `)`, usersArgs},
		{`DELETE FROM chat_sessions WHERE user_id IN (` + usersClause + `) OR peer_user_id IN (` + usersClause + `)`, joinArgs(usersArgs, usersArgs)},
		{`DELETE FROM chat_messages WHERE send_user_id IN (` + usersClause + `) OR rev_user_id IN (` + usersClause + `)`, joinArgs(usersArgs, usersArgs)},
		{`DELETE FROM group_sessions WHERE user_id IN (` + usersClause + `) OR group_id IN (` + groupsClause + `)`, joinArgs(usersArgs, groupsArgs)},
		{`DELETE FROM group_messages WHERE send_user_id IN (` + usersClause + `) OR group_id IN (` + groupsClause + `)`, joinArgs(usersArgs, groupsArgs)},
		{`DELETE FROM group_verifies WHERE user_id IN (` + usersClause + `) OR group_id IN (` + groupsClause + `)`, joinArgs(usersArgs, groupsArgs)},
		{`DELETE FROM group_members WHERE user_id IN (` + usersClause + `) OR group_id IN (` + groupsClause + `)`, joinArgs(usersArgs, groupsArgs)},
		{`DELETE FROM ` + "`groups`" + ` WHERE id IN (` + groupsClause + `)`, groupsArgs},
		{`DELETE FROM friend_verifies WHERE requester_id IN (` + usersClause + `) OR receiver_id IN (` + usersClause + `)`, joinArgs(usersArgs, usersArgs)},
		{`DELETE FROM friends WHERE user_id IN (` + usersClause + `) OR friend_id IN (` + usersClause + `)`, joinArgs(usersArgs, usersArgs)},
		{`DELETE FROM audit_logs WHERE user_id IN (` + usersClause + `)`, usersArgs},
		{`DELETE FROM user_confs WHERE user_id IN (` + usersClause + `)`, usersArgs},
		{`DELETE FROM users WHERE id IN (` + usersClause + `)`, usersArgs},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return errors.New("cleanup prefixed performance rows failed")
		}
	}
	if err := tx.Commit(); err != nil {
		return errors.New("commit cleanup failed")
	}
	return nil
}

func placeholders(values []uint64) (string, []any) {
	if len(values) == 0 {
		return "0", nil
	}
	parts := make([]string, len(values))
	args := make([]any, len(values))
	for index, value := range values {
		parts[index], args[index] = "?", value
	}
	return strings.Join(parts, ","), args
}

func joinArgs(groups ...[]any) []any {
	var result []any
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}
