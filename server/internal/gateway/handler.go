// Package gateway implements the GIM authentication-aware service proxy.
package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/kanhai447/GIM/server/internal/gateway/authclient"
	"github.com/kanhai447/GIM/server/internal/gateway/proxy"
	"github.com/kanhai447/GIM/server/internal/gateway/route"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
)

const (
	CodeInvalidPath        uint32 = 1301
	CodeUnsupportedService uint32 = 1302
	CodeServiceUnavailable uint32 = 1303
	CodeAuthUnavailable    uint32 = 1304
)

type Authenticator interface {
	Authenticate(context.Context, string, string) (authclient.Result, error)
}

type Resolver interface {
	Resolve(context.Context, string) ([]discovery.Endpoint, error)
}

type Forwarder interface {
	ServeHTTP(http.ResponseWriter, *http.Request, url.URL)
}

type Handler struct {
	auth             Authenticator
	resolver         Resolver
	forwarder        Forwarder
	discoveryTimeout time.Duration
}

func NewHandler(auth Authenticator, resolver Resolver, forwarder Forwarder, discoveryTimeout time.Duration) *Handler {
	return &Handler{auth: auth, resolver: resolver, forwarder: forwarder, discoveryTimeout: discoveryTimeout}
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	scrubIdentityHeaders(request.Header)
	if request.URL.RawPath != "" {
		writeFailure(writer, apperror.HTTP(http.StatusBadRequest, CodeInvalidPath, "非法 API 路径"))
		return
	}
	target, err := route.Parse(request.URL.Path)
	if err != nil {
		switch {
		case errors.Is(err, route.ErrUnsupportedService):
			writeFailure(writer, apperror.HTTP(http.StatusNotFound, CodeUnsupportedService, "不支持的服务"))
		default:
			writeFailure(writer, apperror.HTTP(http.StatusBadRequest, CodeInvalidPath, "非法 API 路径"))
		}
		return
	}

	result, err := handler.auth.Authenticate(request.Context(), requestToken(request), request.URL.Path)
	if err != nil {
		writeFailure(writer, authError(err))
		return
	}
	if result.Authenticated {
		request.Header.Set("User-ID", strconv.FormatUint(result.UserID, 10))
		request.Header.Set("Role", strconv.FormatInt(int64(result.Role), 10))
	}

	lookupContext, cancel := context.WithTimeout(request.Context(), handler.discoveryTimeout)
	endpoints, err := handler.resolver.Resolve(lookupContext, target.DiscoveryService)
	cancel()
	if err != nil {
		writeFailure(writer, discoveryError(err))
		return
	}
	if len(endpoints) == 0 {
		writeFailure(writer, discoveryError(discovery.ErrServiceNotFound))
		return
	}
	handler.forwarder.ServeHTTP(writer, request, endpoints[0].URL)
}

func scrubIdentityHeaders(header http.Header) {
	header.Del("User-ID")
	header.Del("Role")
	header.Del("ValidPath")
}

func requestToken(request *http.Request) string {
	if token := request.Header.Get("token"); token != "" {
		return token
	}
	return request.URL.Query().Get("token")
}

func authError(err error) error {
	var publicError *apperror.Error
	if errors.As(err, &publicError) {
		return err
	}
	if errors.Is(err, authclient.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
		return apperror.HTTP(http.StatusGatewayTimeout, CodeAuthUnavailable, "认证服务超时")
	}
	if errors.Is(err, context.Canceled) {
		return apperror.HTTP(http.StatusRequestTimeout, CodeAuthUnavailable, "请求已取消")
	}
	return apperror.HTTP(http.StatusServiceUnavailable, CodeAuthUnavailable, "认证服务不可用")
}

func discoveryError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.HTTP(http.StatusGatewayTimeout, CodeServiceUnavailable, "服务发现超时")
	}
	if errors.Is(err, context.Canceled) {
		return apperror.HTTP(http.StatusRequestTimeout, CodeServiceUnavailable, "请求已取消")
	}
	if errors.Is(err, discovery.ErrServiceNotFound) {
		return apperror.HTTP(http.StatusServiceUnavailable, CodeServiceUnavailable, "服务未注册")
	}
	return apperror.HTTP(http.StatusServiceUnavailable, CodeServiceUnavailable, "服务发现不可用")
}

func writeFailure(writer http.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }

var _ Forwarder = (*proxy.Forwarder)(nil)
