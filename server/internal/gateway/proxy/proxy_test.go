package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestForwarderPreservesHTTPExchange(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Method != http.MethodPut || request.URL.Path != "/api/user/user_info" || request.URL.RawQuery != "view=full" || string(body) != "request-body" || request.Header.Get("X-Test") != "request-header" {
			t.Fatalf("upstream request = %s %s?%s body=%q headers=%#v", request.Method, request.URL.Path, request.URL.RawQuery, body, request.Header)
		}
		writer.Header().Set("X-Upstream", "response-header")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("response-body"))
	}))
	defer upstream.Close()
	endpoint, _ := url.Parse(upstream.URL)
	request := httptest.NewRequest(http.MethodPut, "/api/user/user_info?view=full", strings.NewReader("request-body"))
	request.Header.Set("X-Test", "request-header")
	recorder := httptest.NewRecorder()
	New(upstream.Client().Transport, time.Second).ServeHTTP(recorder, request, *endpoint)
	if recorder.Code != http.StatusCreated || recorder.Header().Get("X-Upstream") != "response-header" || recorder.Body.String() != "response-body" {
		t.Fatalf("proxy response status=%d headers=%#v body=%q", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestForwarderMapsUnavailableAndTimeout(t *testing.T) {
	unavailable, _ := url.Parse("http://127.0.0.1:1")
	recorder := httptest.NewRecorder()
	New(http.DefaultTransport, 100*time.Millisecond).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/user_info", nil), *unavailable)
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), `"code":1305`) || strings.Contains(strings.ToLower(recorder.Body.String()), "dial") {
		t.Fatalf("unavailable response status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	slow := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer slow.Close()
	endpoint, _ := url.Parse(slow.URL)
	recorder = httptest.NewRecorder()
	New(slow.Client().Transport, 20*time.Millisecond).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/user/user_info", nil), *endpoint)
	if recorder.Code != http.StatusGatewayTimeout || !strings.Contains(recorder.Body.String(), `"code":1306`) {
		t.Fatalf("timeout response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUpgradeDetection(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/chat/ws/chat", nil)
	request.Header.Set("Connection", "keep-alive, Upgrade")
	request.Header.Set("Upgrade", "websocket")
	if !isUpgrade(request) {
		t.Fatal("WebSocket Upgrade was not recognized")
	}
}
