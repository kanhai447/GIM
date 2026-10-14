package http

import (
	"context"
	"errors"
	"log"
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/kanhai447/GIM/server/internal/platform/adminapi"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	useradmin "github.com/kanhai447/GIM/server/internal/user/admin"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	"github.com/zeromicro/go-zero/rest"
)

const (
	codeInvalid   uint32 = 1110
	codeForbidden uint32 = 1111
	codeNotFound  uint32 = 1112
)

type Application interface {
	List(context.Context, useradmin.Query) (useradmin.Page, error)
	UpdateRestrictions(context.Context, useradmin.Restrictions) error
	Disable(context.Context, uint64, uint64) error
	Enable(context.Context, uint64, uint64) error
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
	handler := &Handler{application: application, auditor: publisher, logger: log.Default()}
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodGet, Path: "/api/user/users", Handler: handler.List},
		{Method: stdhttp.MethodPost, Path: "/api/user/curtail", Handler: handler.UpdateRestrictions},
		{Method: stdhttp.MethodDelete, Path: "/api/user/users", Handler: handler.Disable},
		{Method: stdhttp.MethodPut, Path: "/api/user/users", Handler: handler.Enable},
	})
}

func (handler *Handler) List(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if _, ok := adminapi.Identity(request); !ok {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	before, limit, ok := adminapi.CursorLimit(request.URL.Query(), 50, 100)
	status := domain.Status(0)
	if text := strings.TrimSpace(request.URL.Query().Get("status")); text != "" {
		parsed, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			ok = false
		} else {
			status = domain.Status(parsed)
		}
	}
	if !ok {
		writeError(writer, apperror.Business(codeInvalid, "用户查询参数无效"))
		return
	}
	page, err := handler.application.List(request.Context(), useradmin.Query{BeforeID: before, Limit: limit, Search: request.URL.Query().Get("search"), Status: status})
	if err != nil {
		handlerError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, page)
}

func (handler *Handler) UpdateRestrictions(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	actorID, ok := adminapi.Identity(request)
	if !ok {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	var input useradmin.Restrictions
	if !adminapi.DecodeJSON(request, &input) {
		writeError(writer, apperror.Business(codeInvalid, "用户限制参数无效"))
		return
	}
	if err := handler.application.UpdateRestrictions(request.Context(), input); err != nil {
		handlerError(writer, err)
		return
	}
	handler.emit(request, audit.EventAdminUserRestrictionsUpdated, "update_restrictions", actorID, "user", input.UserID, "target_user_id")
	_ = httpresponse.Success(writer, map[string]bool{"updated": true})
}

func (handler *Handler) Disable(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	actorID, ok := adminapi.Identity(request)
	targetID, err := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("userId")), 10, 64)
	if !ok {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	if err != nil || targetID == 0 {
		writeError(writer, apperror.Business(codeInvalid, "用户参数无效"))
		return
	}
	if err := handler.application.Disable(request.Context(), actorID, targetID); err != nil {
		handlerError(writer, err)
		return
	}
	handler.emit(request, audit.EventAdminUserDisabled, "disable", actorID, "user", targetID, "target_user_id")
	_ = httpresponse.Success(writer, map[string]any{"disabled": true, "status": domain.StatusDisabled})
}

func (handler *Handler) Enable(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	actorID, ok := adminapi.Identity(request)
	targetID, err := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("userId")), 10, 64)
	if !ok {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	if err != nil || targetID == 0 {
		writeError(writer, apperror.Business(codeInvalid, "用户参数无效"))
		return
	}
	if err := handler.application.Enable(request.Context(), actorID, targetID); err != nil {
		handlerError(writer, err)
		return
	}
	handler.emit(request, audit.EventAdminUserEnabled, "enable", actorID, "user", targetID, "target_user_id")
	_ = httpresponse.Success(writer, map[string]any{"enabled": true, "status": domain.StatusActive})
}

func handlerError(writer stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, useradmin.ErrInvalid):
		writeError(writer, apperror.Business(codeInvalid, "用户管理参数无效"))
	case errors.Is(err, useradmin.ErrNotFound):
		writeError(writer, apperror.HTTP(stdhttp.StatusNotFound, codeNotFound, "用户不存在"))
	case errors.Is(err, useradmin.ErrSelfAction):
		writeError(writer, apperror.Business(codeInvalid, "不能禁用当前管理员账号"))
	default:
		writeError(writer, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
	}
}

func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }

func (handler *Handler) emit(request *stdhttp.Request, eventType, action string, actorID uint64, resourceType string, resourceID uint64, metadataKey string) {
	metadata, err := audit.Metadata(map[string]any{metadataKey: resourceID})
	if err != nil {
		return
	}
	event := audit.NewEvent(eventType, "user", action, audit.StatusSuccess)
	event.UserID = audit.Uint64(actorID)
	event.Role = audit.Int32(1)
	event.ResourceType = resourceType
	event.ResourceID = strconv.FormatUint(resourceID, 10)
	event.Metadata = metadata
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(audit.HTTPContext(request), handler.auditor, event, handler.logger)
}
