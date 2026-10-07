package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestPublicFieldsPreservesAppError(t *testing.T) {
	err := Business(1001, "参数错误")
	status, code, message := PublicFields(err)
	if status != http.StatusOK || code != 1001 || message != "参数错误" {
		t.Fatalf("PublicFields() = %d, %d, %q", status, code, message)
	}
}

func TestPublicFieldsHidesUnknownError(t *testing.T) {
	status, code, message := PublicFields(errors.New("database password leaked here"))
	if status != http.StatusInternalServerError || code != CodeInternal || message != internalMessage {
		t.Fatalf("PublicFields() = %d, %d, %q", status, code, message)
	}
}

func TestWrapRetainsPrivateCause(t *testing.T) {
	cause := errors.New("private cause")
	err := Wrap(cause, http.StatusBadGateway, CodeInternal, "上游服务不可用")
	if !errors.Is(err, cause) {
		t.Fatal("Wrap() did not retain the private cause")
	}
}
