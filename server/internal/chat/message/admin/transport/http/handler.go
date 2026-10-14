package http

import (
	"context"
	"errors"
	"log"
	stdhttp "net/http"
	"strconv"
	"strings"

	chatadmin "github.com/kanhai447/GIM/server/internal/chat/message/admin"
	"github.com/kanhai447/GIM/server/internal/platform/adminapi"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
)

const (
	codeInvalid   uint32 = 2220
	codeForbidden uint32 = 2221
	codeNotFound  uint32 = 2222
)

type Application interface {
	List(context.Context, chatadmin.Query) (chatadmin.Page, error)
	Delete(context.Context, uint64) error
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
		{Method: stdhttp.MethodGet, Path: "/api/chat/admin/history", Handler: handler.List},
		{Method: stdhttp.MethodDelete, Path: "/api/chat/admin/history", Handler: handler.Delete},
	})
}

func (handler *Handler) List(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if _, ok := adminapi.Identity(request); !ok {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	before, limit, ok := adminapi.CursorLimit(request.URL.Query(), 50, 100)
	query := chatadmin.Query{BeforeID: before, Limit: limit}
	if !ok || !parseOptionalID(request, "userId", &query.UserID) || !parseOptionalID(request, "sendUserId", &query.SenderID) || !parseOptionalID(request, "revUserId", &query.ReceiverID) {
		writeError(writer, apperror.Business(codeInvalid, "私聊查询参数无效"))
		return
	}
	page, err := handler.application.List(request.Context(), query)
	if err != nil {
		handlerError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, page)
}

func (handler *Handler) Delete(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	actorID, ok := adminapi.Identity(request)
	if !ok {
		writeError(writer, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
		return
	}
	id, err := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("messageId")), 10, 64)
	if err != nil || id == 0 {
		writeError(writer, apperror.Business(codeInvalid, "消息参数无效"))
		return
	}
	if err := handler.application.Delete(request.Context(), id); err != nil {
		handlerError(writer, err)
		return
	}
	metadata, _ := audit.Metadata(map[string]any{"message_id": id})
	event := audit.NewEvent(audit.EventAdminChatMessageDeleted, "chat", "delete_message", audit.StatusSuccess)
	event.UserID = audit.Uint64(actorID)
	event.Role = audit.Int32(1)
	event.ResourceType = "chat_message"
	event.ResourceID = strconv.FormatUint(id, 10)
	event.Metadata = metadata
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(audit.HTTPContext(request), handler.auditor, event, handler.logger)
	_ = httpresponse.Success(writer, map[string]bool{"deleted": true})
}

func parseOptionalID(request *stdhttp.Request, key string, target *uint64) bool {
	value := strings.TrimSpace(request.URL.Query().Get(key))
	if value == "" {
		return true
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return false
	}
	*target = parsed
	return true
}

func handlerError(writer stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chatadmin.ErrInvalid):
		writeError(writer, apperror.Business(codeInvalid, "私聊管理参数无效"))
	case errors.Is(err, chatadmin.ErrNotFound):
		writeError(writer, apperror.HTTP(stdhttp.StatusNotFound, codeNotFound, "消息不存在"))
	default:
		writeError(writer, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
	}
}

func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }
