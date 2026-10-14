package http

import (
	"context"
	"errors"
	"log"
	stdhttp "net/http"
	"strconv"
	"strings"

	groupadmin "github.com/kanhai447/GIM/server/internal/group/admin"
	"github.com/kanhai447/GIM/server/internal/platform/adminapi"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

const (
	codeInvalid   uint32 = 2320
	codeForbidden uint32 = 2321
	codeNotFound  uint32 = 2322
)

type Application interface {
	List(context.Context, groupadmin.Query) (groupadmin.Page, error)
	ListMessages(context.Context, groupadmin.MessageQuery) (groupadmin.MessagePage, error)
	Dissolve(context.Context, uint64) error
	DeleteMessage(context.Context, uint64) error
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
		{Method: stdhttp.MethodGet, Path: "/api/group/groups", Handler: handler.List},
		{Method: stdhttp.MethodDelete, Path: "/api/group/groups", Handler: handler.Dissolve},
		{Method: stdhttp.MethodGet, Path: "/api/group/messages/:id", Handler: handler.Messages},
		{Method: stdhttp.MethodDelete, Path: "/api/group/messages", Handler: handler.DeleteMessage},
	})
}

func (handler *Handler) List(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if _, ok := adminapi.Identity(request); !ok {
		forbidden(writer)
		return
	}
	before, limit, ok := adminapi.CursorLimit(request.URL.Query(), 30, 100)
	status := uint8(0)
	if text := strings.TrimSpace(request.URL.Query().Get("status")); text != "" {
		parsed, err := strconv.ParseUint(text, 10, 8)
		if err != nil {
			ok = false
		} else {
			status = uint8(parsed)
		}
	}
	if !ok {
		writeError(writer, apperror.Business(codeInvalid, "群查询参数无效"))
		return
	}
	page, err := handler.application.List(request.Context(), groupadmin.Query{BeforeID: before, Limit: limit, Search: request.URL.Query().Get("search"), Status: status})
	if err != nil {
		handlerError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, page)
}

func (handler *Handler) Messages(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if _, ok := adminapi.Identity(request); !ok {
		forbidden(writer)
		return
	}
	groupID, err := strconv.ParseUint(pathvar.Vars(request)["id"], 10, 64)
	before, limit, ok := adminapi.CursorLimit(request.URL.Query(), 50, 100)
	if err != nil || groupID == 0 || !ok {
		writeError(writer, apperror.Business(codeInvalid, "群消息查询参数无效"))
		return
	}
	page, err := handler.application.ListMessages(request.Context(), groupadmin.MessageQuery{GroupID: groupID, BeforeID: before, Limit: limit})
	if err != nil {
		handlerError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, page)
}

func (handler *Handler) Dissolve(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	actorID, ok := adminapi.Identity(request)
	if !ok {
		forbidden(writer)
		return
	}
	groupID, err := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("groupId")), 10, 64)
	if err != nil || groupID == 0 {
		writeError(writer, apperror.Business(codeInvalid, "群参数无效"))
		return
	}
	if err := handler.application.Dissolve(request.Context(), groupID); err != nil {
		handlerError(writer, err)
		return
	}
	handler.emit(request, audit.EventAdminGroupDissolved, "dissolve", actorID, "group", groupID, "group_id")
	_ = httpresponse.Success(writer, map[string]bool{"dissolved": true})
}

func (handler *Handler) DeleteMessage(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	actorID, ok := adminapi.Identity(request)
	if !ok {
		forbidden(writer)
		return
	}
	messageID, err := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("messageId")), 10, 64)
	if err != nil || messageID == 0 {
		writeError(writer, apperror.Business(codeInvalid, "群消息参数无效"))
		return
	}
	if err := handler.application.DeleteMessage(request.Context(), messageID); err != nil {
		handlerError(writer, err)
		return
	}
	handler.emit(request, audit.EventAdminGroupMessageDeleted, "delete_message", actorID, "group_message", messageID, "message_id")
	_ = httpresponse.Success(writer, map[string]bool{"deleted": true})
}

func forbidden(writer stdhttp.ResponseWriter) {
	writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
}
func handlerError(writer stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, groupadmin.ErrInvalid):
		writeError(writer, apperror.Business(codeInvalid, "群管理参数无效"))
	case errors.Is(err, groupadmin.ErrNotFound):
		writeError(writer, apperror.HTTP(stdhttp.StatusNotFound, codeNotFound, "群或消息不存在"))
	default:
		writeError(writer, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
	}
}
func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }
func (handler *Handler) emit(request *stdhttp.Request, eventType, action string, actorID uint64, resourceType string, resourceID uint64, metadataKey string) {
	metadata, _ := audit.Metadata(map[string]any{metadataKey: resourceID})
	event := audit.NewEvent(eventType, "group", action, audit.StatusSuccess)
	event.UserID = audit.Uint64(actorID)
	event.Role = audit.Int32(1)
	event.ResourceType = resourceType
	event.ResourceID = strconv.FormatUint(resourceID, 10)
	event.Metadata = metadata
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(audit.HTTPContext(request), handler.auditor, event, handler.logger)
}
