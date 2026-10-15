package process

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzHandlerReturnsMinimalResponse(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	HealthzHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("health response = %d %q", recorder.Code, recorder.Body.String())
	}
}
