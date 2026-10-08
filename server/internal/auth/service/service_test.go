package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/auth/credential"
	"github.com/kanhai447/GIM/server/internal/auth/revocation"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	"github.com/kanhai447/GIM/server/internal/auth/userclient"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

var testNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

type fakeUsers struct {
	create func(context.Context, userclient.CreateInput) (userclient.User, error)
	get    func(context.Context, string) (userclient.User, error)
}

func (users fakeUsers) Create(ctx context.Context, input userclient.CreateInput) (userclient.User, error) {
	return users.create(ctx, input)
}

func (users fakeUsers) GetByAccount(ctx context.Context, account string) (userclient.User, error) {
	return users.get(ctx, account)
}

type fakeRevocations struct {
	revoke    func(context.Context, string, time.Duration) error
	isRevoked func(context.Context, string) (bool, error)
}

func (store fakeRevocations) Revoke(ctx context.Context, fingerprint string, ttl time.Duration) error {
	return store.revoke(ctx, fingerprint, ttl)
}

func (store fakeRevocations) IsRevoked(ctx context.Context, fingerprint string) (bool, error) {
	return store.isRevoked(ctx, fingerprint)
}

func TestRegisterHashesPasswordAndReturnsPublicUser(t *testing.T) {
	plaintext := "correct-password"
	type traceKey string
	ctx := context.WithValue(context.Background(), traceKey("id"), "request-context")
	users := fakeUsers{create: func(received context.Context, input userclient.CreateInput) (userclient.User, error) {
		if received.Value(traceKey("id")) != "request-context" {
			t.Fatal("Register did not propagate caller context")
		}
		if input.PasswordHash == plaintext || credential.NewPasswords(4).Compare(input.PasswordHash, plaintext) != nil {
			t.Fatal("Register did not pass a bcrypt hash to User RPC")
		}
		if input.Role != userclient.MemberRole || input.Status != userclient.ActiveStatus {
			t.Fatalf("Create input role/status = %d/%d", input.Role, input.Status)
		}
		return userclient.User{ID: 8, Account: input.Account, Nickname: input.Nickname, Role: input.Role, Status: input.Status}, nil
	}}
	service := testService(t, users, emptyRevocations())
	result, err := service.Register(ctx, RegisterInput{Account: "gim-user", Nickname: "GIM", Password: plaintext, Repeat: plaintext})
	if err != nil || result.UserID != 8 {
		t.Fatalf("Register() = %#v, %v", result, err)
	}
	payload, _ := json.Marshal(result)
	if strings.Contains(string(payload), plaintext) || strings.Contains(strings.ToLower(string(payload)), "hash") || strings.Contains(strings.ToLower(string(payload)), "password") {
		t.Fatalf("Register response exposed password material: %s", payload)
	}
}

func TestRegisterDuplicateAccount(t *testing.T) {
	users := fakeUsers{create: func(context.Context, userclient.CreateInput) (userclient.User, error) {
		return userclient.User{}, userclient.ErrDuplicateAccount
	}}
	_, err := testService(t, users, emptyRevocations()).Register(context.Background(), RegisterInput{
		Account: "gim-user", Nickname: "GIM", Password: "correct-password", Repeat: "correct-password",
	})
	assertCode(t, err, CodeDuplicateAccount)
}

func TestLoginSuccessAndClaims(t *testing.T) {
	hash, err := credential.NewPasswords(4).Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	users := fakeUsers{get: func(context.Context, string) (userclient.User, error) {
		return userclient.User{ID: 9, Account: "gim-user", Nickname: "GIM", PasswordHash: hash, Role: 2, Status: 1}, nil
	}}
	service := testService(t, users, emptyRevocations())
	result, err := service.Login(context.Background(), LoginInput{Account: "gim-user", Password: "correct-password"})
	if err != nil || result.Token == "" || result.User.UserID != 9 {
		t.Fatalf("Login() = %#v, %v", result, err)
	}
	claims, err := service.tokens.Parse(result.Token)
	if err != nil || claims.UserID != 9 || claims.Role != 2 || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		t.Fatalf("login claims = %#v, %v", claims, err)
	}
}

func TestLoginFailureCases(t *testing.T) {
	hash, err := credential.NewPasswords(4).Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	tests := []struct {
		name string
		user userclient.User
		err  error
		want uint32
	}{
		{name: "wrong password", user: userclient.User{ID: 9, PasswordHash: hash, Role: 2, Status: 1}, want: CodeInvalidCredential},
		{name: "unknown account", err: userclient.ErrNotFound, want: CodeInvalidCredential},
		{name: "disabled user", user: userclient.User{ID: 9, PasswordHash: hash, Role: 2, Status: userclient.Disabled}, want: CodeUserDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := fakeUsers{get: func(context.Context, string) (userclient.User, error) { return test.user, test.err }}
			password := "correct-password"
			if test.name == "wrong password" {
				password = "incorrect-password"
			}
			_, gotErr := testService(t, users, emptyRevocations()).Login(context.Background(), LoginInput{Account: "gim-user", Password: password})
			assertCode(t, gotErr, test.want)
			if test.name == "unknown account" {
				_, _, message := apperror.PublicFields(gotErr)
				if message != "账号或密码错误" {
					t.Fatalf("unknown-account message = %q", message)
				}
			}
		})
	}
}

func TestLogoutRevokesFingerprintForRemainingLifetime(t *testing.T) {
	var storedFingerprint string
	var storedTTL time.Duration
	type traceKey string
	ctx := context.WithValue(context.Background(), traceKey("id"), "logout-context")
	store := fakeRevocations{
		revoke: func(received context.Context, fingerprint string, ttl time.Duration) error {
			if received.Value(traceKey("id")) != "logout-context" {
				t.Fatal("Logout did not propagate caller context")
			}
			storedFingerprint, storedTTL = fingerprint, ttl
			return nil
		},
		isRevoked: func(context.Context, string) (bool, error) { return false, nil },
	}
	service := testService(t, fakeUsers{}, store)
	raw, _, err := service.tokens.Issue(10, 2)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	before, err := service.Authenticate(context.Background(), raw, "/api/user/user_info")
	if err != nil || !before.Authenticated {
		t.Fatalf("Authenticate() before logout = %#v, %v", before, err)
	}
	if err := service.Logout(ctx, raw); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if storedFingerprint != revocation.Fingerprint(raw) || strings.Contains(storedFingerprint, raw) {
		t.Fatalf("stored fingerprint = %q", storedFingerprint)
	}
	if storedTTL != time.Hour {
		t.Fatalf("stored TTL = %v, want %v", storedTTL, time.Hour)
	}
}

func TestAuthenticationPublicProtectedAndRevoked(t *testing.T) {
	revoked := false
	store := fakeRevocations{
		revoke:    func(context.Context, string, time.Duration) error { revoked = true; return nil },
		isRevoked: func(context.Context, string) (bool, error) { return revoked, nil },
	}
	service := testService(t, fakeUsers{}, store)
	public, err := service.Authenticate(context.Background(), "", "/api/auth/login")
	if err != nil || !public.Public || public.Authenticated {
		t.Fatalf("public Authenticate() = %#v, %v", public, err)
	}
	_, err = service.Authenticate(context.Background(), "", "/api/user/user_info")
	assertCode(t, err, CodeTokenMissing)
	raw, _, err := service.tokens.Issue(12, 1)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	valid, err := service.Authenticate(context.Background(), raw, "/api/user/user_info")
	if err != nil || !valid.Authenticated || valid.UserID != 12 || valid.Role != 1 {
		t.Fatalf("valid Authenticate() = %#v, %v", valid, err)
	}
	if err := service.Logout(context.Background(), raw); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	_, err = service.Authenticate(context.Background(), raw, "/api/user/user_info")
	assertCode(t, err, CodeTokenRevoked)
}

func TestAuthenticationMapsRedisFailureToInternal(t *testing.T) {
	privateDetail := "redis password=private-test-value"
	store := fakeRevocations{
		revoke:    func(context.Context, string, time.Duration) error { return nil },
		isRevoked: func(context.Context, string) (bool, error) { return false, errors.New(privateDetail) },
	}
	service := testService(t, fakeUsers{}, store)
	raw, _, _ := service.tokens.Issue(12, 2)
	_, err := service.Authenticate(context.Background(), raw, "/api/user/user_info")
	_, code, message := apperror.PublicFields(err)
	if code != apperror.CodeInternal || strings.Contains(message, privateDetail) {
		t.Fatalf("Authenticate() leaked Redis error: %q", message)
	}
}

func testService(t *testing.T, users Users, store Revocations) *Service {
	t.Helper()
	manager, err := token.NewManagerWithClock([]byte(strings.Repeat("t", 32)), time.Hour, func() time.Time { return testNow })
	if err != nil {
		t.Fatalf("NewManagerWithClock() error = %v", err)
	}
	return NewWithClock(users, credential.NewPasswords(4), manager, store, DefaultAllowlist(), func() time.Time { return testNow })
}

func emptyRevocations() fakeRevocations {
	return fakeRevocations{
		revoke:    func(context.Context, string, time.Duration) error { return nil },
		isRevoked: func(context.Context, string) (bool, error) { return false, nil },
	}
}

func assertCode(t *testing.T, err error, want uint32) {
	t.Helper()
	_, got, _ := apperror.PublicFields(err)
	if got != want {
		t.Fatalf("error code = %d, want %d (err=%v)", got, want, err)
	}
}
