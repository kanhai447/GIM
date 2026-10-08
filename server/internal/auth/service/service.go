// Package service implements transport-independent Auth behavior.
package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/auth/revocation"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	"github.com/kanhai447/GIM/server/internal/auth/userclient"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

const (
	CodeInvalidArgument   uint32 = 1001
	CodeDuplicateAccount  uint32 = 1102
	CodeInvalidCredential uint32 = 1201
	CodeUserDisabled      uint32 = 1202
	CodeTokenMissing      uint32 = 1203
	CodeTokenInvalid      uint32 = 1204
	CodeTokenExpired      uint32 = 1205
	CodeTokenRevoked      uint32 = 1206
)

type Users interface {
	Create(context.Context, userclient.CreateInput) (userclient.User, error)
	GetByAccount(context.Context, string) (userclient.User, error)
}

type Passwords interface {
	Hash(string) (string, error)
	Compare(string, string) error
}

type Tokens interface {
	Issue(uint64, int32) (string, token.Claims, error)
	Parse(string) (token.Claims, error)
}

type Revocations interface {
	Revoke(context.Context, string, time.Duration) error
	IsRevoked(context.Context, string) (bool, error)
}

type Service struct {
	users       Users
	passwords   Passwords
	tokens      Tokens
	revocations Revocations
	allowlist   Allowlist
	now         func() time.Time
}

type RegisterInput struct {
	Account  string
	Nickname string
	Password string
	Repeat   string
}

type LoginInput struct {
	Account  string
	Password string
}

type PublicUser struct {
	UserID   uint64 `json:"userID"`
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Role     int32  `json:"role"`
	Status   int32  `json:"status"`
}

type LoginResult struct {
	Token string     `json:"token"`
	User  PublicUser `json:"user"`
}

type AuthenticationResult struct {
	UserID        uint64 `json:"userID"`
	Role          int32  `json:"role"`
	Authenticated bool   `json:"authenticated"`
	Public        bool   `json:"public"`
}

func New(users Users, passwords Passwords, tokens Tokens, revocations Revocations, allowlist Allowlist) *Service {
	return NewWithClock(users, passwords, tokens, revocations, allowlist, time.Now)
}

func NewWithClock(users Users, passwords Passwords, tokens Tokens, revocations Revocations, allowlist Allowlist, now func() time.Time) *Service {
	return &Service{users: users, passwords: passwords, tokens: tokens, revocations: revocations, allowlist: allowlist, now: now}
}

func (service *Service) Register(ctx context.Context, input RegisterInput) (PublicUser, error) {
	if err := ctx.Err(); err != nil {
		return PublicUser{}, err
	}
	input.Account = strings.TrimSpace(input.Account)
	input.Nickname = strings.TrimSpace(input.Nickname)
	if !validAccount(input.Account) || utf8.RuneCountInString(input.Nickname) < 1 || utf8.RuneCountInString(input.Nickname) > 32 || !validPassword(input.Password) || input.Password != input.Repeat {
		return PublicUser{}, apperror.Business(CodeInvalidArgument, "参数错误")
	}
	hash, err := service.passwords.Hash(input.Password)
	if err != nil {
		return PublicUser{}, internal(err)
	}
	user, err := service.users.Create(ctx, userclient.CreateInput{
		Account: input.Account, Nickname: input.Nickname, PasswordHash: hash,
		Role: userclient.MemberRole, Status: userclient.ActiveStatus,
	})
	if err != nil {
		return PublicUser{}, userError(err, false)
	}
	return publicUser(user), nil
}

func (service *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if err := ctx.Err(); err != nil {
		return LoginResult{}, err
	}
	input.Account = strings.TrimSpace(input.Account)
	if input.Account == "" || input.Password == "" {
		return LoginResult{}, apperror.Business(CodeInvalidArgument, "参数错误")
	}
	if !validAccount(input.Account) || !validPassword(input.Password) {
		return LoginResult{}, invalidCredentials()
	}
	user, err := service.users.GetByAccount(ctx, input.Account)
	if err != nil {
		return LoginResult{}, userError(err, true)
	}
	if err := service.passwords.Compare(user.PasswordHash, input.Password); err != nil {
		return LoginResult{}, invalidCredentials()
	}
	if user.Status == userclient.Disabled {
		return LoginResult{}, apperror.Business(CodeUserDisabled, "用户已禁用")
	}
	if user.Status != userclient.ActiveStatus || user.ID == 0 || user.Role != 1 && user.Role != 2 {
		return LoginResult{}, internal(errors.New("user rpc returned invalid auth state"))
	}
	rawToken, _, err := service.tokens.Issue(user.ID, user.Role)
	if err != nil {
		return LoginResult{}, internal(err)
	}
	return LoginResult{Token: rawToken, User: publicUser(user)}, nil
}

func (service *Service) Authenticate(ctx context.Context, rawToken, validPath string) (AuthenticationResult, error) {
	if err := ctx.Err(); err != nil {
		return AuthenticationResult{}, err
	}
	if service.allowlist.IsPublic(validPath) {
		return AuthenticationResult{Public: true}, nil
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return AuthenticationResult{}, apperror.HTTP(http.StatusUnauthorized, CodeTokenMissing, "缺少认证令牌")
	}
	claims, err := service.tokens.Parse(rawToken)
	if err != nil {
		return AuthenticationResult{}, tokenError(err)
	}
	revoked, err := service.revocations.IsRevoked(ctx, revocation.Fingerprint(rawToken))
	if err != nil {
		return AuthenticationResult{}, internal(err)
	}
	if revoked {
		return AuthenticationResult{}, apperror.HTTP(http.StatusUnauthorized, CodeTokenRevoked, "认证令牌已注销")
	}
	return AuthenticationResult{UserID: claims.UserID, Role: claims.Role, Authenticated: true}, nil
}

func (service *Service) Logout(ctx context.Context, rawToken string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return apperror.HTTP(http.StatusUnauthorized, CodeTokenMissing, "缺少认证令牌")
	}
	claims, err := service.tokens.Parse(rawToken)
	if err != nil {
		return tokenError(err)
	}
	ttl := claims.ExpiresAt.Time.Sub(service.now().UTC())
	if ttl <= 0 {
		return tokenError(token.ErrExpired)
	}
	if err := service.revocations.Revoke(ctx, revocation.Fingerprint(rawToken), ttl); err != nil {
		return internal(err)
	}
	return nil
}

func publicUser(user userclient.User) PublicUser {
	return PublicUser{UserID: user.ID, Account: user.Account, Nickname: user.Nickname, Avatar: user.Avatar, Role: user.Role, Status: user.Status}
}

func validAccount(account string) bool {
	if len(account) < 3 || len(account) > 64 {
		return false
	}
	for _, character := range account {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validPassword(password string) bool {
	return len(password) >= 8 && len(password) <= 72 && utf8.ValidString(password)
}

func invalidCredentials() error {
	return apperror.Business(CodeInvalidCredential, "账号或密码错误")
}

func userError(err error, hideNotFound bool) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, userclient.ErrNotFound) && hideNotFound:
		return invalidCredentials()
	case errors.Is(err, userclient.ErrDuplicateAccount):
		return apperror.Business(CodeDuplicateAccount, "账号已存在")
	default:
		return internal(err)
	}
}

func tokenError(err error) error {
	if errors.Is(err, token.ErrExpired) {
		return apperror.HTTP(http.StatusUnauthorized, CodeTokenExpired, "认证令牌已过期")
	}
	return apperror.HTTP(http.StatusUnauthorized, CodeTokenInvalid, "认证令牌无效")
}

func internal(cause error) error {
	return apperror.Wrap(cause, http.StatusInternalServerError, apperror.CodeInternal, "服务器错误")
}
