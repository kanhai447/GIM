package httpresponse

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
)

func TestSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := Success(recorder, map[string]any{"userID": float64(42)}); err != nil {
		t.Fatalf("Success() error = %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body Body
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Code != apperror.CodeOK || body.Msg != successMessage {
		t.Fatalf("body = %#v", body)
	}
}

func TestBusinessFailureKeepsHTTP200(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := Failure(recorder, apperror.Business(1001, "参数错误")); err != nil {
		t.Fatalf("Failure() error = %v", err)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestUnknownFailureDoesNotLeakError(t *testing.T) {
	recorder := httptest.NewRecorder()
	secret := "sensitive internal detail"
	if err := Failure(recorder, errors.New(secret)); err != nil {
		t.Fatalf("Failure() error = %v", err)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatal("response leaked an internal error")
	}
}

func TestSuccessFallsBackWhenDataCannotBeMarshaled(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := Success(recorder, make(chan int))
	if err == nil {
		t.Fatal("Success() error = nil, want marshal error")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
