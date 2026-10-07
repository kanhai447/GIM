// Package http provides User HTTP handlers for the go-zero REST server.
package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	"github.com/zeromicro/go-zero/rest"
)

type UserReader interface {
	GetUserByID(ctx context.Context, userID uint64) (domain.User, error)
}

type ProfileHandler struct {
	users UserReader
}

type profileResponse struct {
	UserID   uint64        `json:"userID"`
	Account  string        `json:"account"`
	Nickname string        `json:"nickname"`
	Avatar   string        `json:"avatar"`
	Role     domain.Role   `json:"role"`
	Status   domain.Status `json:"status"`
}

func NewProfileHandler(users UserReader) *ProfileHandler {
	return &ProfileHandler{users: users}
}

func RegisterRoutes(server *rest.Server, users UserReader) {
	handler := NewProfileHandler(users)
	server.AddRoute(rest.Route{
		Method:  stdhttp.MethodGet,
		Path:    "/api/user/user_info",
		Handler: handler.ServeHTTP,
	})
}

func (handler *ProfileHandler) ServeHTTP(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	userID, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	if err != nil || userID == 0 {
		_ = httpresponse.Failure(writer, apperror.Business(userservice.CodeInvalidArgument, "参数错误"))
		return
	}

	user, err := handler.users.GetUserByID(request.Context(), userID)
	if err != nil {
		_ = httpresponse.Failure(writer, err)
		return
	}
	_ = httpresponse.Success(writer, profileResponse{
		UserID:   user.ID,
		Account:  user.Account,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Role:     user.Role,
		Status:   user.Status,
	})
}
