// Package http exposes admin-only structured audit log queries.
package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/kanhai447/GIM/server/internal/logs/domain"
	"github.com/kanhai447/GIM/server/internal/logs/repository"
	logsservice "github.com/kanhai447/GIM/server/internal/logs/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

const (
	CodeInvalidQuery uint32 = 2601
	CodeForbidden    uint32 = 2603
	CodeNotFound     uint32 = 2604
)

type Application interface {
	List(context.Context, domain.Query) (domain.Page, error)
	Get(context.Context, uint64) (domain.Log, error)
}

type Handler struct{ logs Application }

func RegisterRoutes(server *rest.Server, logs Application) {
	handler := &Handler{logs: logs}
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodGet, Path: "/api/logs/logs", Handler: handler.List},
		{Method: stdhttp.MethodGet, Path: "/api/logs/logs/:id", Handler: handler.Detail},
	})
}

func (handler *Handler) List(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if !isAdmin(request) {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, CodeForbidden, "无管理员权限"))
		return
	}
	query, err := parseQuery(request)
	if err != nil {
		writeError(writer, apperror.Business(CodeInvalidQuery, "日志查询参数无效"))
		return
	}
	page, err := handler.logs.List(request.Context(), query)
	if err != nil {
		writeServiceError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, page)
}

func (handler *Handler) Detail(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if !isAdmin(request) {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, CodeForbidden, "无管理员权限"))
		return
	}
	id, err := strconv.ParseUint(pathvar.Vars(request)["id"], 10, 64)
	if err != nil || id == 0 {
		writeError(writer, apperror.Business(CodeInvalidQuery, "日志参数无效"))
		return
	}
	value, err := handler.logs.Get(request.Context(), id)
	if err != nil {
		writeServiceError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, value)
}

func parseQuery(request *stdhttp.Request) (domain.Query, error) {
	values := request.URL.Query()
	query := domain.Query{Service: strings.TrimSpace(values.Get("service")), EventType: strings.TrimSpace(values.Get("eventType")), Status: strings.TrimSpace(values.Get("status")), RequestID: strings.TrimSpace(values.Get("requestId"))}
	var err error
	if query.BeforeID, err = optionalUint(values.Get("beforeId")); err != nil {
		return domain.Query{}, err
	}
	if limit := values.Get("limit"); limit != "" {
		query.Limit, err = strconv.Atoi(limit)
		if err != nil {
			return domain.Query{}, err
		}
	}
	if userID, parseErr := optionalUint(values.Get("userId")); parseErr != nil {
		return domain.Query{}, parseErr
	} else if userID > 0 {
		query.UserID = &userID
	}
	if query.From, err = logsservice.ParseTime(values.Get("from")); err != nil {
		return domain.Query{}, err
	}
	if query.To, err = logsservice.ParseTime(values.Get("to")); err != nil {
		return domain.Query{}, err
	}
	return query, nil
}

func optionalUint(value string) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, logsservice.ErrInvalidQuery
	}
	return parsed, nil
}

func isAdmin(request *stdhttp.Request) bool {
	if request == nil || strings.TrimSpace(request.Header.Get("Role")) != "1" {
		return false
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	return err == nil && userID > 0
}

func writeServiceError(writer stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, logsservice.ErrInvalidQuery):
		writeError(writer, apperror.Business(CodeInvalidQuery, "日志查询参数无效"))
	case errors.Is(err, repository.ErrNotFound):
		writeError(writer, apperror.HTTP(stdhttp.StatusNotFound, CodeNotFound, "日志不存在"))
	default:
		writeError(writer, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
	}
}
func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }
