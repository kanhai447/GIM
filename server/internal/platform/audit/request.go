package audit

import (
	"context"
	"net/http"
	"strings"
)

const RequestIDHeader = "X-Request-ID"

type requestIDKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

// HTTPContext copies only the Gateway-generated request ID. It deliberately
// does not trust forwarded client IP headers because no trusted-proxy list is
// configured in V1.
func HTTPContext(request *http.Request) context.Context {
	if request == nil {
		return context.Background()
	}
	return WithRequestID(request.Context(), request.Header.Get(RequestIDHeader))
}

func EnrichHTTP(event *Event, request *http.Request) {
	if event == nil || request == nil {
		return
	}
	event.RequestID = strings.TrimSpace(request.Header.Get(RequestIDHeader))
	event.Method = request.Method
	event.Path = request.URL.Path
}
