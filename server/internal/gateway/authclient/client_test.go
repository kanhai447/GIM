package authclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
)

type fakeResolver struct {
	endpoints []discovery.Endpoint
	err       error
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (body *trackedBody) Close() error {
	body.closed = true
	return nil
}

func (resolver fakeResolver) Resolve(context.Context, string) ([]discovery.Endpoint, error) {
	return resolver.endpoints, resolver.err
}

func TestAuthenticateUsesCompatibilityHeadersAndDecodesIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/auth/authentication" || request.Header.Get("Token") != "opaque" || request.Header.Get("ValidPath") != "/api/user/user_info" {
			t.Fatalf("Auth request method/path/headers = %s %s %#v", request.Method, request.URL.Path, request.Header)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":0,"msg":"成功","data":{"userID":7,"role":2,"authenticated":true,"public":false}}`))
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	client := New(fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}, server.Client(), time.Second)
	result, err := client.Authenticate(context.Background(), "opaque", "/api/user/user_info")
	if err != nil || !result.Authenticated || result.UserID != 7 || result.Role != 2 {
		t.Fatalf("Authenticate() = %#v, %v", result, err)
	}
}

func TestAuthenticatePropagatesSafeAuthRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"code":1204,"msg":"认证令牌无效","data":null}`))
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	_, err := New(fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}, server.Client(), time.Second).Authenticate(context.Background(), "opaque", "/api/user/user_info")
	status, code, message := apperror.PublicFields(err)
	if status != http.StatusUnauthorized || code != 1204 || message != "认证令牌无效" {
		t.Fatalf("Auth rejection = status %d code %d message %q", status, code, message)
	}
}

func TestAuthenticateClosesBodyAndRejectsInvalidResponse(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader(`{"code":0,"data":null}`)}
	httpClient := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
	})}
	endpoint, _ := url.Parse("http://auth.internal")
	_, err := New(fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}, httpClient, time.Second).Authenticate(context.Background(), "opaque", "/api/user/user_info")
	if err == nil || strings.Contains(err.Error(), endpoint.String()) || !body.closed {
		t.Fatalf("invalid response error = %v", err)
	}
}

func TestAuthenticateTimeoutAndUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = writer.Write([]byte(`{"code":0,"data":{"public":true}}`))
	}))
	defer server.Close()
	endpoint, _ := url.Parse(server.URL)
	_, err := New(fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}, server.Client(), 20*time.Millisecond).Authenticate(context.Background(), "", "/api/auth/login")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("timeout error = %v", err)
	}
	_, err = New(fakeResolver{err: discovery.ErrServiceNotFound}, server.Client(), time.Second).Authenticate(context.Background(), "", "/api/auth/login")
	if err == nil || err.Error() != ErrUnavailable.Error() {
		t.Fatalf("unavailable error = %v", err)
	}
}
