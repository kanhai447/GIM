// Package http exposes Auth through the GIM V1 go-zero HTTP contract.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
)

const maxRequestBodyBytes = 1 << 20

type Application interface {
	Register(context.Context, service.RegisterInput) (service.PublicUser, error)
	Login(context.Context, service.LoginInput) (service.LoginResult, error)
	Authenticate(context.Context, string, string) (service.AuthenticationResult, error)
	Logout(context.Context, string) error
}

type Handler struct {
	auth    Application
	auditor audit.Publisher
	logger  *log.Logger
}

type registerRequest struct {
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	Password string `json:"pwd"`
	Repeat   string `json:"rePwd"`
}

type loginRequest struct {
	Account  string `json:"account"`
	UserName string `json:"userName"`
	Password string `json:"password"`
}

func NewHandler(auth Application, publishers ...audit.Publisher) *Handler {
	var publisher audit.Publisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	return &Handler{auth: auth, auditor: publisher, logger: log.Default()}
}

func RegisterRoutes(server *rest.Server, auth Application, publishers ...audit.Publisher) {
	handler := NewHandler(auth, publishers...)
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodPost, Path: "/api/auth/register", Handler: handler.Register},
		{Method: stdhttp.MethodPost, Path: "/api/auth/login", Handler: handler.Login},
		{Method: stdhttp.MethodPost, Path: "/api/auth/authentication", Handler: handler.Authentication},
		{Method: stdhttp.MethodPost, Path: "/api/auth/logout", Handler: handler.Logout},
	})
}

func (handler *Handler) Register(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	var input registerRequest
	if err := decodeJSON(writer, request, &input); err != nil {
		writeInvalidArgument(writer)
		return
	}
	ctx := audit.HTTPContext(request)
	user, err := handler.auth.Register(ctx, service.RegisterInput{
		Account: input.Account, Nickname: input.Nickname, Password: input.Password, Repeat: input.Repeat,
	})
	if err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
	event := audit.NewEvent(audit.EventUserRegistered, "auth", "register", audit.StatusSuccess)
	event.UserID, event.Role, event.ResourceType, event.ResourceID = audit.Uint64(user.UserID), audit.Int32(user.Role), "user", strconv.FormatUint(user.UserID, 10)
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(ctx, handler.auditor, event, handler.logger)
	_ = httpresponse.Success(writer, user)
}

func (handler *Handler) Login(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	var input loginRequest
	if err := decodeJSON(writer, request, &input); err != nil {
		writeInvalidArgument(writer)
		return
	}
	account := strings.TrimSpace(input.Account)
	legacyAccount := strings.TrimSpace(input.UserName)
	if account == "" {
		account = legacyAccount
	} else if legacyAccount != "" && legacyAccount != account {
		writeInvalidArgument(writer)
		return
	}
	ctx := audit.HTTPContext(request)
	result, err := handler.auth.Login(ctx, service.LoginInput{Account: account, Password: input.Password})
	if err != nil {
		handler.emitLoginFailure(ctx, request, err)
		_ = httpresponse.Failure(writer, err)
		return
	}
	event := audit.NewEvent(audit.EventAuthLoginSuccess, "auth", "login", audit.StatusSuccess)
	event.UserID, event.Role, event.ResourceType, event.ResourceID = audit.Uint64(result.User.UserID), audit.Int32(result.User.Role), "user", strconv.FormatUint(result.User.UserID, 10)
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(ctx, handler.auditor, event, handler.logger)
	_ = httpresponse.Success(writer, result)
}

func (handler *Handler) Authentication(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	result, err := handler.auth.Authenticate(request.Context(), request.Header.Get("Token"), request.Header.Get("ValidPath"))
	if err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
	_ = httpresponse.Success(writer, result)
}

func (handler *Handler) Logout(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	ctx := audit.HTTPContext(request)
	if err := handler.auth.Logout(ctx, request.Header.Get("token")); err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
	event := audit.NewEvent(audit.EventAuthLogout, "auth", "logout", audit.StatusSuccess)
	if userID, parseErr := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64); parseErr == nil && userID > 0 {
		event.UserID = audit.Uint64(userID)
	}
	if role, parseErr := strconv.ParseInt(strings.TrimSpace(request.Header.Get("Role")), 10, 32); parseErr == nil {
		event.Role = audit.Int32(int32(role))
	}
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(ctx, handler.auditor, event, handler.logger)
	_ = httpresponse.Success(writer, map[string]bool{"loggedOut": true})
}

func (handler *Handler) emitLoginFailure(ctx context.Context, request *stdhttp.Request, failure error) {
	_, code, _ := apperror.PublicFields(failure)
	category := "internal"
	switch code {
	case service.CodeInvalidCredential:
		category = "invalid_credentials"
	case service.CodeUserDisabled:
		category = "user_disabled"
	case service.CodeInvalidArgument:
		category = "invalid_request"
	}
	metadata, metadataErr := audit.Metadata(map[string]any{"failure_category": category})
	if metadataErr != nil {
		return
	}
	event := audit.NewEvent(audit.EventAuthLoginFailed, "auth", "login", audit.StatusFailed)
	event.ErrorCode, event.Metadata = code, metadata
	audit.EnrichHTTP(&event, request)
	audit.EmitBestEffort(ctx, handler.auditor, event, handler.logger)
}

func decodeJSON(writer stdhttp.ResponseWriter, request *stdhttp.Request, destination any) error {
	request.Body = stdhttp.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeInvalidArgument(writer stdhttp.ResponseWriter) {
	_ = httpresponse.Failure(writer, apperror.Business(service.CodeInvalidArgument, "参数错误"))
}
