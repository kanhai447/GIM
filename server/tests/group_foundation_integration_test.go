package tests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/domain"
	"github.com/kanhai447/GIM/server/internal/group/mute"
	groupmysql "github.com/kanhai447/GIM/server/internal/group/repository/mysql"
	groupservice "github.com/kanhai447/GIM/server/internal/group/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/internal/platform/rediskeys"
	redisv9 "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type integrationGroupUsers struct {
	profiles   map[uint64]domain.UserProfile
	batchCalls atomic.Int32
}

func (users *integrationGroupUsers) GetByID(ctx context.Context, userID uint64) (domain.UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return domain.UserProfile{}, err
	}
	profile, exists := users.profiles[userID]
	if !exists {
		return domain.UserProfile{}, errors.New("user not found")
	}
	return profile, nil
}

func (users *integrationGroupUsers) BatchGet(ctx context.Context, userIDs []uint64) (map[uint64]domain.UserProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	users.batchCalls.Add(1)
	result := make(map[uint64]domain.UserProfile, len(userIDs))
	for _, userID := range userIDs {
		if profile, exists := users.profiles[userID]; exists {
			result[userID] = profile
		}
	}
	return result, nil
}

func TestGroupFoundationMySQLRedisTransactionsAndConcurrency(t *testing.T) {
	envFile := os.Getenv("GIM_ENV_FILE")
	if envFile == "" {
		t.Skip("set GIM_ENV_FILE to run Group MySQL/Redis integration tests")
	}
	values, err := platformconfig.LoadFile(envFile)
	if err != nil {
		t.Fatalf("load local configuration: %v", err)
	}
	database, rawDB := newChatMessageDatabase(t, values)
	redisConfig, err := platformredis.FromValues(values)
	if err != nil {
		t.Fatalf("redis config: %v", err)
	}
	redisClient, err := platformredis.Open(context.Background(), redisConfig)
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	seed := uint64(time.Now().UnixNano()%1_000_000_000 + 20_000)
	profiles := make(map[uint64]domain.UserProfile)
	for offset := uint64(0); offset < 14; offset++ {
		userID := seed + offset
		account := fmt.Sprintf("group.integration.%d", userID)
		if _, err := rawDB.Exec(`INSERT INTO users(id, account, pwd_hash, nickname, role, status) VALUES (?, ?, 'integration-hash', ?, 2, 1)`, userID, account, account); err != nil {
			t.Fatalf("seed group user: %v", err)
		}
		profiles[userID] = domain.UserProfile{ID: userID, Account: account, Nickname: account, Active: true}
	}
	users := &integrationGroupUsers{profiles: profiles}
	muteStore, err := mute.NewRedisStore(redisClient.Raw(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	repository := groupmysql.New(database)
	groups, err := groupservice.New(repository, users, muteStore, groupservice.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ownerID, adminOneID, adminTwoID := seed, seed+1, seed+2
	memberOneID, memberTwoID := seed+3, seed+4
	applicantID, rejectedID, concurrentID := seed+5, seed+6, seed+7
	failureApplicantID, failureOwnerID := seed+8, seed+9
	inviteID, openUserID := seed+10, seed+11

	created, err := groups.CreateGroup(context.Background(), ownerID, groupservice.CreateGroupInput{
		Name: "GIM Group Foundation", JoinPolicy: domain.JoinPolicyApproval, MemberLimit: 20,
		InvitedUserIDs: []uint64{adminOneID, adminTwoID, memberOneID, memberTwoID},
	})
	if err != nil || created.ID == 0 || created.OwnerID != ownerID {
		t.Fatalf("CreateGroup() = %#v, public=%v private=%v", created, err, errors.Unwrap(err))
	}
	ownerMember, err := repository.GetMember(context.Background(), created.ID, ownerID)
	if err != nil || ownerMember.Role != domain.GroupRoleOwner {
		t.Fatalf("owner membership = %#v, %v", ownerMember, err)
	}
	if member, err := groups.IsMember(context.Background(), created.ID, ownerID); err != nil || !member {
		t.Fatalf("IsMember(owner) = %t, %v", member, err)
	}
	if role, err := groups.GetMemberRole(context.Background(), created.ID, ownerID); err != nil || role != domain.GroupRoleOwner {
		t.Fatalf("GetMemberRole(owner) = %d, %v", role, err)
	}

	if err := groups.UpdateMemberRole(context.Background(), ownerID, created.ID, adminOneID, domain.GroupRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := groups.UpdateMemberRole(context.Background(), ownerID, created.ID, adminTwoID, domain.GroupRoleAdmin); err != nil {
		t.Fatal(err)
	}
	assertBusinessCode(t, groups.LeaveGroup(context.Background(), ownerID, created.ID), groupservice.CodeOwnerCannotLeave)
	assertBusinessCode(t, groups.KickMember(context.Background(), memberOneID, created.ID, memberTwoID), groupservice.CodePermissionDenied)
	assertBusinessCode(t, groups.KickMember(context.Background(), adminOneID, created.ID, adminTwoID), groupservice.CodePermissionDenied)
	if err := groups.KickMember(context.Background(), adminOneID, created.ID, memberOneID); err != nil {
		t.Fatalf("admin kick member: %v", err)
	}
	if err := groups.LeaveGroup(context.Background(), memberTwoID, created.ID); err != nil {
		t.Fatalf("member leave: %v", err)
	}

	request, err := groups.ApplyToJoin(context.Background(), applicantID, created.ID, "please approve")
	if err != nil || request.Status != domain.JoinRequestPending {
		t.Fatalf("ApplyToJoin() = %#v, %v", request, err)
	}
	assertBusinessCode(t, func() error {
		_, duplicateErr := groups.ApplyToJoin(context.Background(), applicantID, created.ID, "duplicate")
		return duplicateErr
	}(), groupservice.CodeDuplicateRequest)

	const reviewers = 12
	var accepted atomic.Int32
	results := make(chan error, reviewers)
	var wait sync.WaitGroup
	for range reviewers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, reviewErr := groups.ReviewJoinRequest(context.Background(), ownerID, request.ID, true)
			if reviewErr == nil {
				accepted.Add(1)
				return
			}
			_, code, _ := apperror.PublicFields(reviewErr)
			if code != groupservice.CodeInvalidRequestState {
				results <- reviewErr
			}
		}()
	}
	wait.Wait()
	close(results)
	for reviewErr := range results {
		t.Fatalf("concurrent approve returned unexpected error: %v private=%v", reviewErr, errors.Unwrap(reviewErr))
	}
	if accepted.Load() != 1 {
		t.Fatalf("successful concurrent approvals = %d, want 1", accepted.Load())
	}
	if countRows(t, rawDB, "group_members", "group_id = ? AND user_id = ?", created.ID, applicantID) != 1 {
		t.Fatal("concurrent approval did not converge to one membership")
	}

	rejected, err := groups.ApplyToJoin(context.Background(), rejectedID, created.ID, "reject me")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := groups.ReviewJoinRequest(context.Background(), adminOneID, rejected.ID, false); err != nil {
		t.Fatal(err)
	}
	assertBusinessCode(t, func() error {
		_, repeatErr := groups.ReviewJoinRequest(context.Background(), ownerID, rejected.ID, true)
		return repeatErr
	}(), groupservice.CodeInvalidRequestState)
	if countRows(t, rawDB, "group_members", "group_id = ? AND user_id = ?", created.ID, rejectedID) != 0 {
		t.Fatal("rejected request created a membership")
	}
	if err := groups.InviteMembers(context.Background(), applicantID, created.ID, []uint64{inviteID}); err != nil {
		t.Fatalf("member invite allowed by group setting: %v", err)
	}
	assertBusinessCode(t, groups.InviteMembers(context.Background(), ownerID, created.ID, []uint64{inviteID}), groupservice.CodeAlreadyMember)

	failureRequest, err := groups.ApplyToJoin(context.Background(), failureApplicantID, created.ID, "inject failure")
	if err != nil {
		t.Fatal(err)
	}
	registerCreateFailure(t, database, "group_members", "group_approve_failure")
	_, approveErr := groups.ReviewJoinRequest(context.Background(), ownerID, failureRequest.ID, true)
	removeCreateFailure(t, database, "group_approve_failure")
	if approveErr == nil {
		t.Fatal("approve failure injection unexpectedly succeeded")
	}
	storedFailureRequest, err := repository.GetJoinRequest(context.Background(), failureRequest.ID)
	if err != nil || storedFailureRequest.Status != domain.JoinRequestPending {
		t.Fatalf("failed approval request state = %#v, %v", storedFailureRequest, err)
	}
	if countRows(t, rawDB, "group_members", "group_id = ? AND user_id = ?", created.ID, failureApplicantID) != 0 {
		t.Fatal("failed approval left a membership")
	}

	registerCreateFailure(t, database, "group_members", "group_create_failure")
	_, createErr := groups.CreateGroup(context.Background(), failureOwnerID, groupservice.CreateGroupInput{Name: "must rollback", JoinPolicy: domain.JoinPolicyApproval})
	removeCreateFailure(t, database, "group_create_failure")
	if createErr == nil || countRows(t, rawDB, "groups", "name = ?", "must rollback") != 0 {
		t.Fatalf("create failure did not rollback group: err=%v", createErr)
	}
	openGroup, err := groups.CreateGroup(context.Background(), failureOwnerID, groupservice.CreateGroupInput{Name: "open group", JoinPolicy: domain.JoinPolicyOpen})
	if err != nil {
		t.Fatal(err)
	}
	openRequest, err := groups.ApplyToJoin(context.Background(), openUserID, openGroup.ID, "open join")
	if err != nil || openRequest.Status != domain.JoinRequestAccepted || countRows(t, rawDB, "group_members", "group_id = ? AND user_id = ?", openGroup.ID, openUserID) != 1 {
		t.Fatalf("open join = %#v, %v", openRequest, err)
	}

	const duplicateAttempts = 16
	var memberCreates atomic.Int32
	wait = sync.WaitGroup{}
	for range duplicateAttempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			member := domain.GroupMember{GroupID: created.ID, UserID: concurrentID, Role: domain.GroupRoleMember}
			if addErr := repository.AddMember(context.Background(), &member); addErr == nil {
				memberCreates.Add(1)
			}
		}()
	}
	wait.Wait()
	if memberCreates.Load() != 1 || countRows(t, rawDB, "group_members", "group_id = ? AND user_id = ?", created.ID, concurrentID) != 1 {
		t.Fatalf("concurrent member creates = %d", memberCreates.Load())
	}

	users.batchCalls.Store(0)
	views, memberCount, err := groups.ListMembers(context.Background(), ownerID, created.ID, 100)
	if err != nil || len(views) != int(memberCount) || users.batchCalls.Load() != 1 {
		t.Fatalf("ListMembers() count=%d len=%d batchCalls=%d err=%v", memberCount, len(views), users.batchCalls.Load(), err)
	}
	myGroups, err := groups.MyGroups(context.Background(), applicantID, 100)
	if err != nil || len(myGroups) != 1 || myGroups[0].Group.ID != created.ID || myGroups[0].Role != domain.GroupRoleMember {
		t.Fatalf("MyGroups() = %#v, public=%v private=%v", myGroups, err, errors.Unwrap(err))
	}

	presenceBefore := redisKeys(t, redisClient.Raw(), "gim:presence:*")
	applicantMember, err := repository.GetMember(context.Background(), created.ID, applicantID)
	if err != nil {
		t.Fatal(err)
	}
	if err := groups.MuteMember(context.Background(), ownerID, created.ID, applicantID, 180*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ttl, err := redisClient.Raw().PTTL(context.Background(), rediskeys.GroupProhibition(applicantMember.ID)).Result()
	if err != nil || ttl <= 0 {
		t.Fatalf("mute TTL = %v, %v", ttl, err)
	}
	if muted, err := groups.IsMuted(context.Background(), created.ID, applicantID); err != nil || !muted {
		t.Fatalf("IsMuted() = %t, %v", muted, err)
	}
	if muted, err := groups.IsMuted(context.Background(), created.ID, concurrentID); err != nil || muted {
		t.Fatalf("isolated IsMuted() = %t, %v", muted, err)
	}
	if err := groups.UnmuteMember(context.Background(), ownerID, created.ID, applicantID); err != nil {
		t.Fatal(err)
	}
	if muted, err := groups.IsMuted(context.Background(), created.ID, applicantID); err != nil || muted {
		t.Fatalf("unmuted state = %t, %v", muted, err)
	}
	if err := groups.InviteMembers(context.Background(), failureOwnerID, openGroup.ID, []uint64{applicantID}); err != nil {
		t.Fatal(err)
	}
	openApplicantMember, err := repository.GetMember(context.Background(), openGroup.ID, applicantID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = redisClient.Raw().Del(context.Background(), rediskeys.GroupProhibition(applicantMember.ID), rediskeys.GroupProhibition(openApplicantMember.ID)).Err()
	})
	if err := groups.MuteMember(context.Background(), failureOwnerID, openGroup.ID, applicantID, time.Second); err != nil {
		t.Fatal(err)
	}
	if muted, err := groups.IsMuted(context.Background(), created.ID, applicantID); err != nil || muted {
		t.Fatalf("cross-group mute leaked to original membership: %t, %v", muted, err)
	}
	if muted, err := groups.IsMuted(context.Background(), openGroup.ID, applicantID); err != nil || !muted {
		t.Fatalf("second group mute missing: %t, %v", muted, err)
	}
	if err := groups.MuteMember(context.Background(), ownerID, created.ID, applicantID, 80*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		muted, lookupErr := groups.IsMuted(context.Background(), created.ID, applicantID)
		if lookupErr != nil {
			t.Fatal(lookupErr)
		}
		if !muted {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if muted, err := groups.IsMuted(context.Background(), created.ID, applicantID); err != nil || muted {
		t.Fatalf("expired mute state = %t, %v", muted, err)
	}
	presenceAfter := redisKeys(t, redisClient.Raw(), "gim:presence:*")
	if strings.Join(presenceBefore, ",") != strings.Join(presenceAfter, ",") {
		t.Fatalf("group mute changed Presence keys: before=%v after=%v", presenceBefore, presenceAfter)
	}

	badRedis := redisv9.NewClient(&redisv9.Options{Addr: "127.0.0.1:1", DialTimeout: 20 * time.Millisecond, ReadTimeout: 20 * time.Millisecond, WriteTimeout: 20 * time.Millisecond, MaxRetries: 0, Protocol: 2})
	t.Cleanup(func() { _ = badRedis.Close() })
	badMuteStore, _ := mute.NewRedisStore(badRedis, 50*time.Millisecond)
	badService, _ := groupservice.New(repository, users, badMuteStore, groupservice.DefaultConfig())
	_, unavailableErr := badService.IsMuted(context.Background(), created.ID, applicantID)
	status, code, message := apperror.PublicFields(unavailableErr)
	if unavailableErr == nil || status != 503 || code != groupservice.CodeMuteStateError || message != "群禁言状态暂不可用" {
		t.Fatalf("Redis failure public error: status=%d code=%d message=%q err=%v", status, code, message, unavailableErr)
	}

	assertGroupExplain(t, rawDB, created.ID, applicantID)
	if err := groups.DissolveGroup(context.Background(), failureOwnerID, openGroup.ID); err != nil {
		t.Fatal(err)
	}
	assertBusinessCode(t, func() error {
		_, dissolvedErr := groups.GetGroupByID(context.Background(), openGroup.ID)
		return dissolvedErr
	}(), groupservice.CodeGroupNotFound)
	if countRows(t, rawDB, "group_members", "group_id = ?", openGroup.ID) != 0 {
		t.Fatal("dissolved group retained memberships")
	}
}

func registerCreateFailure(t *testing.T, database *gorm.DB, table, name string) {
	t.Helper()
	if err := database.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			tx.AddError(errors.New("injected group write failure"))
		}
	}); err != nil {
		t.Fatalf("register failure callback: %v", err)
	}
}

func removeCreateFailure(t *testing.T, database *gorm.DB, name string) {
	t.Helper()
	if err := database.Callback().Create().Remove(name); err != nil {
		t.Fatalf("remove failure callback: %v", err)
	}
}

func countRows(t *testing.T, database *sql.DB, table, condition string, args ...any) int64 {
	t.Helper()
	var count int64
	query := "SELECT COUNT(*) FROM `" + table + "` WHERE " + condition
	if err := database.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func assertBusinessCode(t *testing.T, err error, want uint32) {
	t.Helper()
	_, code, _ := apperror.PublicFields(err)
	if code != want {
		t.Fatalf("business code = %d, want %d; err=%v", code, want, err)
	}
}

func redisKeys(t *testing.T, client *redisv9.Client, pattern string) []string {
	t.Helper()
	keys, err := client.Keys(context.Background(), pattern).Result()
	if err != nil {
		t.Fatalf("list Redis keys: %v", err)
	}
	sort.Strings(keys)
	return keys
}

func assertGroupExplain(t *testing.T, database *sql.DB, groupID, userID uint64) {
	t.Helper()
	queries := []struct {
		name  string
		query string
		args  []any
		index string
	}{
		{"membership", `EXPLAIN FORMAT=JSON SELECT id, role FROM group_members WHERE group_id = ? AND user_id = ?`, []any{groupID, userID}, "ux_group_members_group_user"},
		{"member list", `EXPLAIN FORMAT=JSON SELECT id, user_id, role FROM group_members WHERE group_id = ? ORDER BY role, user_id LIMIT 100`, []any{groupID}, "ix_group_members_group_role_user"},
		{"my groups", "EXPLAIN FORMAT=JSON SELECT `groups`.id FROM group_members mine JOIN `groups` ON `groups`.id = mine.group_id WHERE mine.user_id = ?", []any{userID}, "ix_group_members_user_group"},
		{"pending requests", `EXPLAIN FORMAT=JSON SELECT id FROM group_verifies WHERE group_id = ? AND status = 0 ORDER BY created_at, id`, []any{groupID}, "ix_group_verifies_group_status_created"},
	}
	for _, test := range queries {
		var plan string
		if err := database.QueryRow(test.query, test.args...).Scan(&plan); err != nil {
			t.Fatalf("EXPLAIN %s: %v", test.name, err)
		}
		if !strings.Contains(plan, test.index) {
			t.Fatalf("EXPLAIN %s did not expose designed index %s: %s", test.name, test.index, plan)
		}
	}
}
