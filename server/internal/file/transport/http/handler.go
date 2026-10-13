// Package http exposes authenticated File APIs without leaking storage paths.
package http

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/file/domain"
	fileservice "github.com/kanhai447/GIM/server/internal/file/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

const multipartOverhead int64 = 1 << 20

type Handler struct {
	files    *fileservice.Service
	maxBytes int64
	auditor  audit.Publisher
	logger   *log.Logger
}

func NewHandler(files *fileservice.Service, maxBytes int64, publishers ...audit.Publisher) *Handler {
	var publisher audit.Publisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	return &Handler{files: files, maxBytes: maxBytes, auditor: publisher, logger: log.Default()}
}

func RegisterRoutes(server *rest.Server, files *fileservice.Service, maxBytes int64, publishers ...audit.Publisher) {
	handler := NewHandler(files, maxBytes, publishers...)
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodPost, Path: "/api/file/file", Handler: handler.UploadFile},
		{Method: stdhttp.MethodPost, Path: "/api/file/image", Handler: handler.UploadImage},
		{Method: stdhttp.MethodGet, Path: "/api/file/user-files", Handler: handler.List},
		{Method: stdhttp.MethodGet, Path: "/api/file/user-files/:id", Handler: handler.Detail},
		{Method: stdhttp.MethodDelete, Path: "/api/file/user-files/:id", Handler: handler.Delete},
		{Method: stdhttp.MethodGet, Path: "/api/file/:id", Handler: handler.Download},
	})
}

func (handler *Handler) UploadFile(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	handler.upload(writer, request, "file")
}
func (handler *Handler) UploadImage(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	handler.upload(writer, request, "image")
}

func (handler *Handler) upload(writer stdhttp.ResponseWriter, request *stdhttp.Request, field string) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, apperror.Business(fileservice.CodeInvalid, "身份信息无效"))
		return
	}
	if request.ContentLength > handler.maxBytes+multipartOverhead {
		writeError(writer, apperror.Business(fileservice.CodeTooLarge, "文件超过大小限制"))
		return
	}
	mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		writeError(writer, apperror.Business(fileservice.CodeInvalid, "上传格式无效"))
		return
	}
	request.Body = stdhttp.MaxBytesReader(writer, request.Body, handler.maxBytes+multipartOverhead)
	reader := multipart.NewReader(request.Body, params["boundary"])
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			writeError(writer, apperror.Business(fileservice.CodeInvalid, "缺少上传文件"))
			return
		}
		if nextErr != nil {
			writeError(writer, uploadReadError(nextErr))
			return
		}
		if part.FormName() != field || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		result, uploadErr := handler.files.Upload(request.Context(), userID, part.FileName(), part)
		_ = part.Close()
		if uploadErr != nil {
			writeError(writer, uploadErr)
			return
		}
		metadata, metadataErr := audit.Metadata(map[string]any{
			"user_file_id": result.Metadata.UserFile.ID, "file_object_uid": result.Metadata.Object.UID,
			"size_bytes": result.Metadata.Object.Size, "mime_type": result.Metadata.Object.MIMEType,
			"object_created": result.ObjectCreated, "reference_created": result.ReferenceNew,
		})
		if metadataErr == nil {
			event := audit.NewEvent(audit.EventFileUploaded, "file", "upload", audit.StatusSuccess)
			event.UserID, event.ResourceType, event.ResourceID, event.Metadata = audit.Uint64(userID), "user_file", strconv.FormatUint(result.Metadata.UserFile.ID, 10), metadata
			audit.EnrichHTTP(&event, request)
			audit.EmitBestEffort(audit.HTTPContext(request), handler.auditor, event, handler.logger)
		}
		_ = httpresponse.Success(writer, uploadDTO(result.Metadata, result.ObjectCreated, result.ReferenceNew))
		return
	}
}

func (handler *Handler) List(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	before, beforeOK := queryUint(request, "beforeId")
	limit, limitOK := queryInt(request, "limit")
	if !ok || !beforeOK || !limitOK {
		writeError(writer, apperror.Business(fileservice.CodeInvalid, "文件列表参数无效"))
		return
	}
	items, err := handler.files.List(request.Context(), userID, before, limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	dtos := make([]metadataResponse, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, metadataDTO(item))
	}
	var next uint64
	if len(items) > 0 {
		next = items[len(items)-1].UserFile.ID
	}
	_ = httpresponse.Success(writer, map[string]any{"list": dtos, "nextBeforeId": next})
}

func (handler *Handler) Detail(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	id := pathvar.Vars(request)["id"]
	if !ok {
		writeError(writer, apperror.Business(fileservice.CodeInvalid, "身份信息无效"))
		return
	}
	metadata, err := handler.files.Get(request.Context(), userID, id)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, metadataDTO(metadata))
}

func (handler *Handler) Delete(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	id, parseErr := strconv.ParseUint(pathvar.Vars(request)["id"], 10, 64)
	if !ok || parseErr != nil || id == 0 {
		writeError(writer, apperror.Business(fileservice.CodeInvalid, "文件参数无效"))
		return
	}
	if err := handler.files.Delete(request.Context(), userID, id); err != nil {
		writeError(writer, err)
		return
	}
	metadata, metadataErr := audit.Metadata(map[string]any{"user_file_id": id})
	if metadataErr == nil {
		event := audit.NewEvent(audit.EventFileDeleted, "file", "delete", audit.StatusSuccess)
		event.UserID, event.ResourceType, event.ResourceID, event.Metadata = audit.Uint64(userID), "user_file", strconv.FormatUint(id, 10), metadata
		audit.EnrichHTTP(&event, request)
		audit.EmitBestEffort(audit.HTTPContext(request), handler.auditor, event, handler.logger)
	}
	_ = httpresponse.Success(writer, map[string]bool{"deleted": true})
}

func (handler *Handler) Download(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, apperror.Business(fileservice.CodeInvalid, "身份信息无效"))
		return
	}
	metadata, err := handler.files.Get(request.Context(), userID, pathvar.Vars(request)["id"])
	if err != nil {
		writeError(writer, err)
		return
	}
	object, err := handler.files.Open(request.Context(), metadata)
	if err != nil {
		writeError(writer, err)
		return
	}
	defer object.Close()
	writer.Header().Set("Content-Type", metadata.Object.MIMEType)
	writer.Header().Set("Content-Length", strconv.FormatInt(metadata.Object.Size, 10))
	writer.Header().Set("Content-Disposition", fileservice.ContentDisposition(metadata.UserFile.OriginalName))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	stdhttp.ServeContent(writer, request, metadata.UserFile.OriginalName, metadata.Object.UpdatedAt, object)
}

type metadataResponse struct {
	ID        uint64 `json:"id"`
	UID       string `json:"uid"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	MIMEType  string `json:"mimeType"`
	SHA256    string `json:"sha256"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}
type uploadResponse struct {
	metadataResponse
	ObjectCreated    bool   `json:"objectCreated"`
	ReferenceCreated bool   `json:"referenceCreated"`
	Src              string `json:"src"`
}

func metadataDTO(item domain.FileMetadata) metadataResponse {
	return metadataResponse{ID: item.UserFile.ID, UID: item.Object.UID, FileName: item.UserFile.OriginalName, Size: item.Object.Size, MIMEType: item.Object.MIMEType, SHA256: item.Object.SHA256, URL: fmt.Sprintf("/api/file/%d", item.UserFile.ID), CreatedAt: item.UserFile.CreatedAt.Format(time.RFC3339Nano)}
}
func uploadDTO(item domain.FileMetadata, objectCreated, referenceCreated bool) uploadResponse {
	dto := metadataDTO(item)
	return uploadResponse{metadataResponse: dto, ObjectCreated: objectCreated, ReferenceCreated: referenceCreated, Src: dto.URL}
}

func trustedUserID(request *stdhttp.Request) (uint64, bool) {
	if request == nil {
		return 0, false
	}
	id, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	role, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("Role")), 10, 8)
	return id, err == nil && (role == 1 || role == 2)
}
func queryUint(request *stdhttp.Request, key string) (uint64, bool) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil && parsed != 0
}
func queryInt(request *stdhttp.Request, key string) (int, bool) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}
func uploadReadError(err error) error {
	var maxErr *stdhttp.MaxBytesError
	if errors.As(err, &maxErr) {
		return apperror.Business(fileservice.CodeTooLarge, "文件超过大小限制")
	}
	return apperror.Business(fileservice.CodeInvalid, "上传格式无效")
}
func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }
