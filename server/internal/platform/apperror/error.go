// Package apperror defines errors that are safe to translate to public API
// responses while retaining an optional private cause for server-side handling.
package apperror

import (
	"errors"
	"net/http"
)

const (
	CodeOK       uint32 = 0
	CodeInternal uint32 = 7
)

const internalMessage = "服务器错误"

// Error carries the public response fields separately from its private cause.
type Error struct {
	code       uint32
	message    string
	httpStatus int
	cause      error
}

// Business creates a reference-compatible HTTP 200 business error.
func Business(code uint32, message string) *Error {
	return newError(nil, http.StatusOK, code, message)
}

// HTTP creates an error that also uses a meaningful HTTP status.
func HTTP(status int, code uint32, message string) *Error {
	return newError(nil, status, code, message)
}

// Wrap retains a private cause but exposes only the supplied public message.
func Wrap(cause error, status int, code uint32, message string) *Error {
	return newError(cause, status, code, message)
}

func newError(cause error, status int, code uint32, message string) *Error {
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	if code == CodeOK {
		code = CodeInternal
	}
	if message == "" {
		message = internalMessage
	}
	return &Error{code: code, message: message, httpStatus: status, cause: cause}
}

func (e *Error) Error() string {
	return e.message
}

func (e *Error) Unwrap() error {
	return e.cause
}

// PublicFields resolves an error into fields safe for an HTTP response.
func PublicFields(err error) (status int, code uint32, message string) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.httpStatus, appErr.code, appErr.message
	}
	return http.StatusInternalServerError, CodeInternal, internalMessage
}
