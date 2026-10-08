package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
)

type fakeAuthenticator struct {
	result authclient.Result
	err    error
	token  string
	path   string
	called bool
}

func (auth *fakeAuthenticator) Authenticate(_ context.Context, token, path string) (authclient.Result, error) {
	auth.called, auth.token, auth.path = true, token, path
	return auth.result, auth.err
}

type fakeResolver struct {
	endpoints []discovery.Endpoint
	err       error
	service   string
	wait      bool
}

func (resolver *fakeResolver) Resolve(ctx context.Context, service string) ([]discovery.Endpoint, error) {
	resolver.service = service
	if resolver.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return resolver.endpoints, resolver.err
}

type fakeForwarder struct {
	called   bool
	headers  http.Header
	endpoint url.URL
}

func (forwarder *fakeForwarder) ServeHTTP(writer http.ResponseWriter, request *http.Request, endpoint url.URL) {
	forwarder.called, forwarder.headers, forwarder.endpoint = true, request.Header.Clone(), endpoint
	writer.WriteHeader(http.StatusNoContent)
}

func TestHandlerOverwritesForgedIdentityHeaders(t *testing.T) {
	endpoint, _ := url.Parse("http://127.0.0.1:9001")
	auth := &fakeAuthenticator{result: authclient.Result{UserID: 42, Role: 2, Authenticated: true}}
	resolver := &fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}
	forwarder := &fakeForwarder{}
	handler := NewHandler(auth, resolver, forwarder, time.Second)
	request := httptest.NewRequest(http.MethodGet, "/api/user/user_info?view=full", nil)
	request.Header.Set("token", "opaque-test-credential")
	request.Header.Set("User-ID", "1")
	request.Header.Set("Role", "1")
	request.Header.Set("ValidPath", "/api/auth/login")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || !forwarder.called {
		t.Fatalf("gateway status=%d forwarded=%t", recorder.Code, forwarder.called)
	}
	if auth.token != "opaque-test-credential" || auth.path != "/api/user/user_info" || resolver.service != "user_api" {
		t.Fatalf("auth/discovery inputs token=%q path=%q service=%q", auth.token, auth.path, resolver.service)
	}
	if forwarder.headers.Get("User-ID") != "42" || forwarder.headers.Get("Role") != "2" || forwarder.headers.Get("ValidPath") != "" {
		t.Fatalf("forwarded identity headers = %#v", forwarder.headers)
	}
}

func TestHandlerPublicRequestRemovesForgedIdentity(t *testing.T) {
	endpoint, _ := url.Parse("http://127.0.0.1:9002")
	auth := &fakeAuthenticator{result: authclient.Result{Public: true}}
	forwarder := &fakeForwarder{}
	handler := NewHandler(auth, &fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}, forwarder, time.Second)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{}"))
	request.Header.Set("User-ID", "1")
	request.Header.Set("Role", "1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if forwarder.headers.Get("User-ID") != "" || forwarder.headers.Get("Role") != "" {
		t.Fatalf("public request retained forged identity: %#v", forwarder.headers)
	}
}

func TestHandlerRejectsPathAndDiscoveryFailures(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		resolver   *fakeResolver
		wantStatus int
		wantCode   string
	}{
		{name: "invalid", path: "/not-api", resolver: &fakeResolver{}, wantStatus: http.StatusBadRequest, wantCode: `"code":1301`},
		{name: "unsupported", path: "/api/admin/info", resolver: &fakeResolver{}, wantStatus: http.StatusNotFound, wantCode: `"code":1302`},
		{name: "empty", path: "/api/user/user_info", resolver: &fakeResolver{}, wantStatus: http.StatusServiceUnavailable, wantCode: `"code":1303`},
		{name: "missing", path: "/api/chat/history", resolver: &fakeResolver{err: discovery.ErrServiceNotFound}, wantStatus: http.StatusServiceUnavailable, wantCode: `"code":1303`},
		{name: "timeout", path: "/api/user/user_info", resolver: &fakeResolver{wait: true}, wantStatus: http.StatusGatewayTimeout, wantCode: `"code":1303`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auth := &fakeAuthenticator{result: authclient.Result{Public: true}}
			forwarder := &fakeForwarder{}
			handler := NewHandler(auth, test.resolver, forwarder, 20*time.Millisecond)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantCode) || forwarder.called {
				t.Fatalf("status=%d body=%s forwarded=%t", recorder.Code, recorder.Body.String(), forwarder.called)
			}
			if (test.name == "invalid" || test.name == "unsupported") && auth.called {
				t.Fatal("invalid route reached Auth")
			}
		})
	}
}

func TestHandlerMapsAuthFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unavailable", err: authclient.ErrUnavailable, wantStatus: http.StatusServiceUnavailable},
		{name: "timeout", err: authclient.ErrTimeout, wantStatus: http.StatusGatewayTimeout},
		{name: "canceled", err: context.Canceled, wantStatus: http.StatusRequestTimeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(&fakeAuthenticator{err: test.err}, &fakeResolver{}, &fakeForwarder{}, time.Second)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/user_info", nil))
			if recorder.Code != test.wantStatus || strings.Contains(recorder.Body.String(), test.err.Error()) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHandlerUsesQueryTokenForFutureUpgradeCompatibility(t *testing.T) {
	endpoint, _ := url.Parse("http://127.0.0.1:9001")
	auth := &fakeAuthenticator{result: authclient.Result{UserID: 8, Role: 2, Authenticated: true}}
	handler := NewHandler(auth, &fakeResolver{endpoints: []discovery.Endpoint{{URL: *endpoint}}}, &fakeForwarder{}, time.Second)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/chat/ws/chat?token=query-credential", nil))
	if auth.token != "query-credential" {
		t.Fatalf("query token = %q", auth.token)
	}
}

func TestDiscoveryErrorDoesNotExposeCause(t *testing.T) {
	private := errors.New("private etcd endpoint detail")
	err := discoveryError(private)
	if strings.Contains(err.Error(), private.Error()) {
		t.Fatalf("discoveryError leaked cause: %v", err)
	}
}
