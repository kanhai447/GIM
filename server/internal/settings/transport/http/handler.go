package http

import (
	"context"
	"errors"
	"log"
	stdhttp "net/http"

	"github.com/kanhai447/GIM/server/internal/platform/adminapi"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/kanhai447/GIM/server/internal/settings/domain"
	settingsservice "github.com/kanhai447/GIM/server/internal/settings/service"
	"github.com/zeromicro/go-zero/rest"
)

const (
	codeInvalid   uint32 = 2701
	codeForbidden uint32 = 2703
)

type Application interface {
	Get(context.Context) (domain.Info, error)
	Update(context.Context, domain.Info) (domain.Info, error)
}
type Handler struct {
	application Application
	auditor     audit.Publisher
	logger      *log.Logger
}

func RegisterRoutes(server *rest.Server, application Application, publishers ...audit.Publisher) {
	var publisher audit.Publisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	h := &Handler{application: application, auditor: publisher, logger: log.Default()}
	server.AddRoutes([]rest.Route{{Method: stdhttp.MethodGet, Path: "/api/settings/info", Handler: h.Get}, {Method: stdhttp.MethodGet, Path: "/api/settings/open_login", Handler: h.OpenLogin}, {Method: stdhttp.MethodPut, Path: "/api/settings/admin/info", Handler: h.Update}})
}
func (h *Handler) Get(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	value, err := h.application.Get(r.Context())
	if err != nil {
		internal(w)
		return
	}
	_ = httpresponse.Success(w, value)
}
func (h *Handler) OpenLogin(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	value, err := h.application.Get(r.Context())
	if err != nil {
		internal(w)
		return
	}
	_ = httpresponse.Success(w, value.OpenLogin)
}
func (h *Handler) Update(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := adminapi.Identity(r)
	if !ok {
		_ = httpresponse.Failure(w, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	var input domain.Info
	if !adminapi.DecodeJSON(r, &input) {
		_ = httpresponse.Failure(w, apperror.Business(codeInvalid, "设置参数无效"))
		return
	}
	value, err := h.application.Update(r.Context(), input)
	if errors.Is(err, settingsservice.ErrInvalid) {
		_ = httpresponse.Failure(w, apperror.Business(codeInvalid, "设置参数无效"))
		return
	}
	if err != nil {
		internal(w)
		return
	}
	event := audit.NewEvent(audit.EventAdminSettingsUpdated, "settings", "update", audit.StatusSuccess)
	event.UserID = audit.Uint64(actorID)
	event.Role = audit.Int32(1)
	event.ResourceType = "settings"
	event.ResourceID = "public"
	audit.EnrichHTTP(&event, r)
	audit.EmitBestEffort(audit.HTTPContext(r), h.auditor, event, h.logger)
	_ = httpresponse.Success(w, value)
}
func internal(w stdhttp.ResponseWriter) {
	_ = httpresponse.Failure(w, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
}
