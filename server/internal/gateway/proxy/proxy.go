// Package proxy forwards authenticated Gateway requests to discovered services.
package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
)

const (
	CodeUpstreamUnavailable uint32 = 1305
	CodeGatewayTimeout      uint32 = 1306
)

type Forwarder struct {
	transport http.RoundTripper
	timeout   time.Duration
}

func New(transport http.RoundTripper, timeout time.Duration) *Forwarder {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Forwarder{transport: transport, timeout: timeout}
}

func (forwarder *Forwarder) ServeHTTP(writer http.ResponseWriter, request *http.Request, endpoint url.URL) {
	proxiedRequest := request
	var cancel context.CancelFunc
	if !isUpgrade(request) {
		var proxyContext context.Context
		proxyContext, cancel = context.WithTimeout(request.Context(), forwarder.timeout)
		defer cancel()
		proxiedRequest = request.Clone(proxyContext)
	}
	reverseProxy := &httputil.ReverseProxy{
		Transport:     forwarder.transport,
		FlushInterval: -1,
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			proxyRequest.Out.URL.Scheme = endpoint.Scheme
			proxyRequest.Out.URL.Host = endpoint.Host
			proxyRequest.Out.Host = endpoint.Host
			proxyRequest.SetXForwarded()
		},
		ErrorHandler: func(responseWriter http.ResponseWriter, _ *http.Request, err error) {
			if errors.Is(err, context.DeadlineExceeded) {
				_ = httpresponse.Failure(responseWriter, apperror.HTTP(http.StatusGatewayTimeout, CodeGatewayTimeout, "上游请求超时"))
				return
			}
			if errors.Is(err, context.Canceled) {
				_ = httpresponse.Failure(responseWriter, apperror.HTTP(http.StatusRequestTimeout, CodeGatewayTimeout, "请求已取消"))
				return
			}
			_ = httpresponse.Failure(responseWriter, apperror.HTTP(http.StatusBadGateway, CodeUpstreamUnavailable, "上游服务不可用"))
		},
	}
	reverseProxy.ServeHTTP(writer, proxiedRequest)
}

func isUpgrade(request *http.Request) bool {
	return headerHasToken(request.Header.Get("Connection"), "upgrade") && request.Header.Get("Upgrade") != ""
}

func headerHasToken(value, expected string) bool {
	for _, item := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(item), expected) {
			return true
		}
	}
	return false
}
