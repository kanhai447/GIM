package http

import (
	"context"
	stdhttp "net/http"

	"github.com/kanhai447/GIM/server/internal/platform/adminapi"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/kanhai447/GIM/server/internal/settings/overview"
	"github.com/zeromicro/go-zero/rest"
)

const codeForbidden uint32 = 2713

type Application interface {
	Get(context.Context) (overview.Summary, error)
}

type Handler struct{ application Application }

func RegisterRoutes(server *rest.Server, application Application) {
	handler := &Handler{application: application}
	server.AddRoute(rest.Route{Method: stdhttp.MethodGet, Path: "/api/settings/admin/overview", Handler: handler.Get})
}

func (handler *Handler) Get(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if _, ok := adminapi.Identity(request); !ok {
		_ = httpresponse.Failure(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	summary, err := handler.application.Get(request.Context())
	if err != nil {
		_ = httpresponse.Failure(writer, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
		return
	}
	_ = httpresponse.Success(writer, summary)
}
