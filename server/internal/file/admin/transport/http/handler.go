package http

import (
	"context"
	"errors"
	"log"
	stdhttp "net/http"
	"strconv"
	"strings"

	fileadmin "github.com/kanhai447/GIM/server/internal/file/admin"
	"github.com/kanhai447/GIM/server/internal/file/domain"
	fileservice "github.com/kanhai447/GIM/server/internal/file/service"
	"github.com/kanhai447/GIM/server/internal/file/storage"
	"github.com/kanhai447/GIM/server/internal/platform/adminapi"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

const (
	codeInvalid   uint32 = 2520
	codeForbidden uint32 = 2521
	codeNotFound  uint32 = 2522
)

type Application interface {
	List(context.Context, fileadmin.Query) (fileadmin.Page, error)
	Get(context.Context, uint64) (fileadmin.File, error)
	Delete(context.Context, uint64) error
}
type Opener interface {
	Open(context.Context, domain.FileMetadata) (storage.Object, error)
}
type Handler struct {
	application Application
	opener      Opener
	auditor     audit.Publisher
	logger      *log.Logger
}

func RegisterRoutes(server *rest.Server, application Application, opener Opener, publishers ...audit.Publisher) {
	var publisher audit.Publisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	h := &Handler{application: application, opener: opener, auditor: publisher, logger: log.Default()}
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodGet, Path: "/api/file/files", Handler: h.List},
		{Method: stdhttp.MethodGet, Path: "/api/file/files/:id", Handler: h.Download},
		{Method: stdhttp.MethodDelete, Path: "/api/file/files", Handler: h.Delete},
	})
}
func (h *Handler) List(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if _, ok := adminapi.Identity(r); !ok {
		forbidden(w)
		return
	}
	before, limit, ok := adminapi.CursorLimit(r.URL.Query(), 50, 100)
	userID := uint64(0)
	if text := strings.TrimSpace(r.URL.Query().Get("userId")); text != "" {
		parsed, err := strconv.ParseUint(text, 10, 64)
		if err != nil || parsed == 0 {
			ok = false
		} else {
			userID = parsed
		}
	}
	if !ok {
		writeError(w, apperror.Business(codeInvalid, "文件查询参数无效"))
		return
	}
	page, err := h.application.List(r.Context(), fileadmin.Query{BeforeID: before, Limit: limit, UserID: userID, Search: r.URL.Query().Get("search")})
	if err != nil {
		handlerError(w, err)
		return
	}
	_ = httpresponse.Success(w, page)
}
func (h *Handler) Download(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if _, ok := adminapi.Identity(r); !ok {
		forbidden(w)
		return
	}
	id, err := strconv.ParseUint(pathvar.Vars(r)["id"], 10, 64)
	if err != nil || id == 0 {
		writeError(w, apperror.Business(codeInvalid, "文件参数无效"))
		return
	}
	value, err := h.application.Get(r.Context(), id)
	if err != nil {
		handlerError(w, err)
		return
	}
	object, err := h.opener.Open(r.Context(), value.Metadata)
	if err != nil {
		handlerError(w, err)
		return
	}
	defer object.Close()
	w.Header().Set("Content-Type", value.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(value.Size, 10))
	w.Header().Set("Content-Disposition", fileservice.ContentDisposition(value.FileName))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	stdhttp.ServeContent(w, r, value.FileName, value.Metadata.Object.UpdatedAt, object)
}
func (h *Handler) Delete(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := adminapi.Identity(r)
	if !ok {
		forbidden(w)
		return
	}
	id, err := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("id")), 10, 64)
	if err != nil || id == 0 {
		writeError(w, apperror.Business(codeInvalid, "文件参数无效"))
		return
	}
	if err := h.application.Delete(r.Context(), id); err != nil {
		handlerError(w, err)
		return
	}
	metadata, _ := audit.Metadata(map[string]any{"user_file_id": id})
	event := audit.NewEvent(audit.EventAdminFileDeleted, "file", "admin_delete", audit.StatusSuccess)
	event.UserID, event.Role, event.ResourceType, event.ResourceID, event.Metadata = audit.Uint64(actorID), audit.Int32(1), "user_file", strconv.FormatUint(id, 10), metadata
	audit.EnrichHTTP(&event, r)
	audit.EmitBestEffort(audit.HTTPContext(r), h.auditor, event, h.logger)
	_ = httpresponse.Success(w, map[string]bool{"deleted": true})
}
func forbidden(w stdhttp.ResponseWriter) {
	writeError(w, apperror.HTTP(stdhttp.StatusForbidden, codeForbidden, "无管理员权限"))
}
func handlerError(w stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fileadmin.ErrInvalid):
		writeError(w, apperror.Business(codeInvalid, "文件管理参数无效"))
	case errors.Is(err, fileadmin.ErrNotFound):
		writeError(w, apperror.HTTP(stdhttp.StatusNotFound, codeNotFound, "文件不存在"))
	default:
		writeError(w, apperror.HTTP(stdhttp.StatusInternalServerError, apperror.CodeInternal, "服务器错误"))
	}
}
func writeError(w stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(w, err) }
