// Package authclient calls Auth authentication on behalf of Gateway.
package authclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
)

const maxAuthResponseBytes = 1 << 20

var (
	ErrUnavailable = errors.New("authentication service unavailable")
	ErrTimeout     = errors.New("authentication request timed out")
)

type Resolver interface {
	Resolve(context.Context, string) ([]discovery.Endpoint, error)
}

type Result struct {
	UserID        uint64
	Role          int32
	Authenticated bool
	Public        bool
}

type Client struct {
	resolver Resolver
	http     *http.Client
	timeout  time.Duration
}

func New(resolver Resolver, httpClient *http.Client, timeout time.Duration) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{resolver: resolver, http: httpClient, timeout: timeout}
}

func (client *Client) Authenticate(ctx context.Context, rawToken, validPath string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	callContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	endpoints, err := client.resolver.Resolve(callContext, "auth_api")
	if err != nil {
		return Result{}, classify(callContext, err)
	}
	var lastErr error
	for _, endpoint := range endpoints {
		result, requestErr := client.authenticateEndpoint(callContext, endpoint.URL, rawToken, validPath)
		if requestErr == nil {
			return result, nil
		}
		var publicError *apperror.Error
		if errors.As(requestErr, &publicError) || errors.Is(requestErr, context.Canceled) || errors.Is(requestErr, context.DeadlineExceeded) {
			return Result{}, classify(callContext, requestErr)
		}
		lastErr = requestErr
	}
	return Result{}, classify(callContext, lastErr)
}

func (client *Client) authenticateEndpoint(ctx context.Context, endpoint url.URL, rawToken, validPath string) (Result, error) {
	endpoint.Path = "/api/auth/authentication"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if rawToken != "" {
		request.Header.Set("Token", rawToken)
	}
	request.Header.Set("ValidPath", validPath)
	response, err := client.http.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()

	var envelope struct {
		Code uint32 `json:"code"`
		Msg  string `json:"msg"`
		Data *struct {
			UserID        uint64 `json:"userID"`
			Role          int32  `json:"role"`
			Authenticated bool   `json:"authenticated"`
			Public        bool   `json:"public"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxAuthResponseBytes))
	if err := decoder.Decode(&envelope); err != nil {
		return Result{}, ErrUnavailable
	}
	if envelope.Code != 0 {
		status := response.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusUnauthorized
		}
		return Result{}, apperror.HTTP(status, envelope.Code, envelope.Msg)
	}
	if envelope.Data == nil {
		return Result{}, ErrUnavailable
	}
	result := Result{
		UserID: envelope.Data.UserID, Role: envelope.Data.Role, Authenticated: envelope.Data.Authenticated, Public: envelope.Data.Public,
	}
	if result.Public && !result.Authenticated {
		return result, nil
	}
	if result.Authenticated && !result.Public && result.UserID > 0 && (result.Role == 1 || result.Role == 2) {
		return result, nil
	}
	return Result{}, ErrUnavailable
}

func classify(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	var publicError *apperror.Error
	if errors.As(err, &publicError) {
		return err
	}
	return &clientError{cause: err}
}

type clientError struct{ cause error }

func (err *clientError) Error() string { return ErrUnavailable.Error() }
func (err *clientError) Unwrap() error { return err.cause }

func (result Result) String() string {
	return fmt.Sprintf("authenticated=%t public=%t user=%d role=%d", result.Authenticated, result.Public, result.UserID, result.Role)
}
