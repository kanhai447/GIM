// Package http exposes Auth through the GIM V1 go-zero HTTP contract.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strings"

	"github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
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

type Handler struct{ auth Application }

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

func NewHandler(auth Application) *Handler { return &Handler{auth: auth} }

func RegisterRoutes(server *rest.Server, auth Application) {
	handler := NewHandler(auth)
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
	user, err := handler.auth.Register(request.Context(), service.RegisterInput{
		Account: input.Account, Nickname: input.Nickname, Password: input.Password, Repeat: input.Repeat,
	})
	if err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
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
	result, err := handler.auth.Login(request.Context(), service.LoginInput{Account: account, Password: input.Password})
	if err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
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
	if err := handler.auth.Logout(request.Context(), request.Header.Get("token")); err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"loggedOut": true})
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
