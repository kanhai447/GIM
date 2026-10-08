package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanhai447/GIM/server/internal/auth/service"
)

type fakeApplication struct {
	register     func(context.Context, service.RegisterInput) (service.PublicUser, error)
	login        func(context.Context, service.LoginInput) (service.LoginResult, error)
	authenticate func(context.Context, string, string) (service.AuthenticationResult, error)
	logout       func(context.Context, string) error
}

func (app fakeApplication) Register(ctx context.Context, input service.RegisterInput) (service.PublicUser, error) {
	return app.register(ctx, input)
}
func (app fakeApplication) Login(ctx context.Context, input service.LoginInput) (service.LoginResult, error) {
	return app.login(ctx, input)
}
func (app fakeApplication) Authenticate(ctx context.Context, token, path string) (service.AuthenticationResult, error) {
	return app.authenticate(ctx, token, path)
}
func (app fakeApplication) Logout(ctx context.Context, token string) error {
	return app.logout(ctx, token)
}

func TestRegisterResponseDoesNotExposePasswordMaterial(t *testing.T) {
	plaintext := "correct-password"
	handler := NewHandler(fakeApplication{register: func(_ context.Context, input service.RegisterInput) (service.PublicUser, error) {
		if input.Password != plaintext || input.Repeat != plaintext {
			t.Fatalf("Register input = %#v", input)
		}
		return service.PublicUser{UserID: 7, Account: input.Account, Nickname: input.Nickname, Role: 2, Status: 1}, nil
	}})
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/auth/register", strings.NewReader(`{"account":"gim-user","nickname":"GIM","pwd":"correct-password","rePwd":"correct-password"}`))
	recorder := httptest.NewRecorder()
	handler.Register(recorder, request)
	body := recorder.Body.String()
	if recorder.Code != stdhttp.StatusOK || !strings.Contains(body, `"userID":7`) {
		t.Fatalf("register response status=%d body=%s", recorder.Code, body)
	}
	if strings.Contains(body, plaintext) || strings.Contains(strings.ToLower(body), "password") || strings.Contains(strings.ToLower(body), "hash") {
		t.Fatalf("register response exposed password material: %s", body)
	}
}

func TestLoginAcceptsAccountAndLegacyUserName(t *testing.T) {
	for _, body := range []string{
		`{"account":"gim-user","password":"correct-password"}`,
		`{"userName":"gim-user","password":"correct-password"}`,
	} {
		handler := NewHandler(fakeApplication{login: func(_ context.Context, input service.LoginInput) (service.LoginResult, error) {
			if input.Account != "gim-user" {
				t.Fatalf("Login account = %q", input.Account)
			}
			return service.LoginResult{Token: "test-token", User: service.PublicUser{UserID: 7}}, nil
		}})
		recorder := httptest.NewRecorder()
		handler.Login(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/auth/login", strings.NewReader(body)))
		if recorder.Code != stdhttp.StatusOK || !strings.Contains(recorder.Body.String(), `"token":"test-token"`) {
			t.Fatalf("login response status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func TestAuthenticationMapsCompatibilityHeaders(t *testing.T) {
	handler := NewHandler(fakeApplication{authenticate: func(_ context.Context, token, path string) (service.AuthenticationResult, error) {
		if token != "test-token" || path != "/api/user/user_info" {
			t.Fatalf("Authenticate headers = %q, %q", token, path)
		}
		return service.AuthenticationResult{UserID: 7, Role: 2, Authenticated: true}, nil
	}})
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/auth/authentication", nil)
	request.Header.Set("Token", "test-token")
	request.Header.Set("ValidPath", "/api/user/user_info")
	recorder := httptest.NewRecorder()
	handler.Authentication(recorder, request)
	if !strings.Contains(recorder.Body.String(), `"authenticated":true`) {
		t.Fatalf("authentication response = %s", recorder.Body.String())
	}
}

func TestLogoutUsesTokenHeaderAndDoesNotEchoToken(t *testing.T) {
	raw := "header.payload.signature"
	handler := NewHandler(fakeApplication{logout: func(_ context.Context, token string) error {
		if token != raw {
			t.Fatalf("Logout token = %q", token)
		}
		return nil
	}})
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("token", raw)
	recorder := httptest.NewRecorder()
	handler.Logout(recorder, request)
	if strings.Contains(recorder.Body.String(), raw) || !strings.Contains(recorder.Body.String(), `"loggedOut":true`) {
		t.Fatalf("logout response = %s", recorder.Body.String())
	}
}

func TestHandlerDoesNotLeakPrivateError(t *testing.T) {
	private := "private user rpc credential detail"
	handler := NewHandler(fakeApplication{login: func(context.Context, service.LoginInput) (service.LoginResult, error) {
		return service.LoginResult{}, errors.New(private)
	}})
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/auth/login", strings.NewReader(`{"account":"gim-user","password":"correct-password"}`)))
	if strings.Contains(recorder.Body.String(), private) {
		t.Fatalf("handler leaked private error: %s", recorder.Body.String())
	}
}
