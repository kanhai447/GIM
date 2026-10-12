// Package http exposes authenticated Group foundation APIs.
package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/domain"
	groupservice "github.com/kanhai447/GIM/server/internal/group/service"
	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/platform/httpresponse"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

type Handler struct{ groups *groupservice.Service }

func RegisterRoutes(server *rest.Server, groups *groupservice.Service) {
	handler := &Handler{groups: groups}
	server.AddRoutes([]rest.Route{
		{Method: stdhttp.MethodPost, Path: "/api/group/group", Handler: handler.Create},
		{Method: stdhttp.MethodGet, Path: "/api/group/group/:id", Handler: handler.Info},
		{Method: stdhttp.MethodDelete, Path: "/api/group/group/:id", Handler: handler.Dissolve},
		{Method: stdhttp.MethodGet, Path: "/api/group/my", Handler: handler.MyGroups},
		{Method: stdhttp.MethodGet, Path: "/api/group/member", Handler: handler.Members},
		{Method: stdhttp.MethodPost, Path: "/api/group/member", Handler: handler.Invite},
		{Method: stdhttp.MethodDelete, Path: "/api/group/member", Handler: handler.RemoveMember},
		{Method: stdhttp.MethodPut, Path: "/api/group/member/role", Handler: handler.UpdateRole},
		{Method: stdhttp.MethodPost, Path: "/api/group/valid", Handler: handler.Apply},
		{Method: stdhttp.MethodPut, Path: "/api/group/valid/status", Handler: handler.Review},
		{Method: stdhttp.MethodPut, Path: "/api/group/member/prohibition", Handler: handler.Mute},
	})
}

type createRequest struct {
	Mode         int8     `json:"mode"`
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Avatar       string   `json:"avatar"`
	Abstract     string   `json:"abstract"`
	IsSearch     bool     `json:"isSearch"`
	JoinPolicy   *uint8   `json:"joinPolicy"`
	Verification *uint8   `json:"verification"`
	Size         uint32   `json:"size"`
	UserIDList   []uint64 `json:"userIdList"`
}

type inviteRequest struct {
	GroupID      uint64   `json:"id"`
	MemberIDList []uint64 `json:"memberIdList"`
}

type roleRequest struct {
	GroupID  uint64 `json:"id"`
	MemberID uint64 `json:"memberId"`
	Role     uint8  `json:"role"`
}

type applyRequest struct {
	GroupID uint64 `json:"groupID"`
	Verify  string `json:"verify"`
}

type reviewRequest struct {
	RequestID uint64 `json:"validId"`
	Status    uint8  `json:"status"`
}

type muteRequest struct {
	GroupID            uint64 `json:"groupId"`
	MemberID           uint64 `json:"memberId"`
	ProhibitionMinutes *int64 `json:"prohibitionTime"`
}

type groupResponse struct {
	GroupID       uint64 `json:"groupId"`
	Title         string `json:"title"`
	Abstract      string `json:"abstract"`
	Avatar        string `json:"avatar"`
	OwnerID       uint64 `json:"ownerId"`
	Status        uint8  `json:"status"`
	Role          uint8  `json:"role,omitempty"`
	MemberCount   uint64 `json:"memberCount"`
	IsSearch      bool   `json:"isSearch"`
	JoinPolicy    uint8  `json:"joinPolicy"`
	IsProhibition bool   `json:"isProhibition"`
	MemberLimit   uint32 `json:"memberLimit"`
}

type memberResponse struct {
	UserID         uint64 `json:"userId"`
	UserNickname   string `json:"userNickname"`
	Avatar         string `json:"avatar"`
	Role           uint8  `json:"role"`
	MemberNickname string `json:"memberNickname"`
	CreatedAt      string `json:"createdAt"`
}

func (handler *Handler) Create(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	if !ok {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "身份信息无效"))
		return
	}
	var input createRequest
	if !decodeJSON(request, &input) {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群参数无效"))
		return
	}
	name := input.Name
	if name == "" {
		name = input.Title
	}
	joinPolicy := uint8(domain.JoinPolicyApproval)
	if input.JoinPolicy != nil {
		joinPolicy = *input.JoinPolicy
	}
	if input.Verification != nil {
		if input.JoinPolicy != nil && *input.JoinPolicy != *input.Verification {
			writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群参数无效"))
			return
		}
		joinPolicy = *input.Verification
	}
	group, err := handler.groups.CreateGroup(request.Context(), operatorID, groupservice.CreateGroupInput{
		Name: name, Avatar: input.Avatar, Description: input.Abstract, IsSearch: input.IsSearch,
		JoinPolicy: domain.JoinPolicy(joinPolicy), MemberLimit: input.Size, InvitedUserIDs: input.UserIDList,
	})
	if err != nil {
		writeError(writer, err)
		return
	}
	info, err := handler.groups.GroupInfo(request.Context(), operatorID, group.ID)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, groupDTO(info.Group, info.CurrentRole, info.MemberCount))
}

func (handler *Handler) Info(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	groupID, pathOK := pathID(request)
	if !ok || !pathOK {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群参数无效"))
		return
	}
	info, err := handler.groups.GroupInfo(request.Context(), operatorID, groupID)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, groupDTO(info.Group, info.CurrentRole, info.MemberCount))
}

func (handler *Handler) Dissolve(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	groupID, pathOK := pathID(request)
	if !ok || !pathOK {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群参数无效"))
		return
	}
	if err := handler.groups.DissolveGroup(request.Context(), operatorID, groupID); err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"dissolved": true})
}

func (handler *Handler) MyGroups(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	limit, limitOK := queryInt(request, "limit")
	if !ok || !limitOK {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群列表参数无效"))
		return
	}
	groups, err := handler.groups.MyGroups(request.Context(), operatorID, limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	items := make([]groupResponse, 0, len(groups))
	for _, summary := range groups {
		items = append(items, groupDTO(summary.Group, summary.Role, summary.MemberCount))
	}
	_ = httpresponse.Success(writer, map[string]any{"list": items, "count": len(items)})
}

func (handler *Handler) Members(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	groupID, groupOK := queryUint64(request, "id")
	limit, limitOK := queryInt(request, "limit")
	if !ok || !groupOK || !limitOK {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群成员参数无效"))
		return
	}
	views, count, err := handler.groups.ListMembers(request.Context(), operatorID, groupID, limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	items := make([]memberResponse, 0, len(views))
	for _, view := range views {
		items = append(items, memberResponse{UserID: view.User.ID, UserNickname: view.User.Nickname, Avatar: view.User.Avatar, Role: uint8(view.Member.Role), MemberNickname: view.Member.MemberNickname, CreatedAt: view.Member.JoinedAt.Format(time.RFC3339Nano)})
	}
	_ = httpresponse.Success(writer, map[string]any{"list": items, "count": count})
}

func (handler *Handler) Invite(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	var input inviteRequest
	if !ok || !decodeJSON(request, &input) {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "邀请成员参数无效"))
		return
	}
	if err := handler.groups.InviteMembers(request.Context(), operatorID, input.GroupID, input.MemberIDList); err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"invited": true})
}

func (handler *Handler) RemoveMember(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	groupID, groupOK := queryUint64(request, "id")
	targetID, targetOK := optionalQueryUint64(request, "memberId")
	if !ok || !groupOK || !targetOK {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "移除成员参数无效"))
		return
	}
	if targetID == 0 {
		targetID = operatorID
	}
	var err error
	if targetID == operatorID {
		err = handler.groups.LeaveGroup(request.Context(), operatorID, groupID)
	} else {
		err = handler.groups.KickMember(request.Context(), operatorID, groupID, targetID)
	}
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"removed": true})
}

func (handler *Handler) UpdateRole(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	var input roleRequest
	if !ok || !decodeJSON(request, &input) {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群角色参数无效"))
		return
	}
	if err := handler.groups.UpdateMemberRole(request.Context(), operatorID, input.GroupID, input.MemberID, domain.GroupRole(input.Role)); err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"updated": true})
}

func (handler *Handler) Apply(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	var input applyRequest
	if !ok || !decodeJSON(request, &input) {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "加群申请参数无效"))
		return
	}
	result, err := handler.groups.ApplyToJoin(request.Context(), operatorID, input.GroupID, input.Verify)
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]any{"validId": result.ID, "status": result.Status})
}

func (handler *Handler) Review(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	var input reviewRequest
	if !ok || !decodeJSON(request, &input) || input.Status != uint8(domain.JoinRequestAccepted) && input.Status != uint8(domain.JoinRequestRejected) {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "加群审批参数无效"))
		return
	}
	result, err := handler.groups.ReviewJoinRequest(request.Context(), operatorID, input.RequestID, input.Status == uint8(domain.JoinRequestAccepted))
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]any{"validId": result.ID, "status": result.Status})
}

func (handler *Handler) Mute(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	operatorID, ok := trustedUserID(request)
	var input muteRequest
	if !ok || !decodeJSON(request, &input) {
		writeError(writer, apperror.Business(groupservice.CodeInvalidArgument, "群禁言参数无效"))
		return
	}
	var err error
	if input.ProhibitionMinutes == nil {
		err = handler.groups.UnmuteMember(request.Context(), operatorID, input.GroupID, input.MemberID)
	} else {
		err = handler.groups.MuteMember(request.Context(), operatorID, input.GroupID, input.MemberID, time.Duration(*input.ProhibitionMinutes)*time.Minute)
	}
	if err != nil {
		writeError(writer, err)
		return
	}
	_ = httpresponse.Success(writer, map[string]bool{"updated": true})
}

func groupDTO(group domain.Group, role domain.GroupRole, count uint64) groupResponse {
	return groupResponse{GroupID: group.ID, Title: group.Name, Abstract: group.Description, Avatar: group.Avatar, OwnerID: group.OwnerID, Status: uint8(group.Status), Role: uint8(role), MemberCount: count, IsSearch: group.IsSearch, JoinPolicy: uint8(group.JoinPolicy), IsProhibition: group.IsAllMuted, MemberLimit: group.MemberLimit}
}

func trustedUserID(request *stdhttp.Request) (uint64, bool) {
	if request == nil {
		return 0, false
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	if err != nil || userID == 0 {
		return 0, false
	}
	role, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("Role")), 10, 8)
	return userID, err == nil && (role == 1 || role == 2)
}

func pathID(request *stdhttp.Request) (uint64, bool) {
	value := pathvar.Vars(request)["id"]
	id, err := strconv.ParseUint(value, 10, 64)
	return id, err == nil && id != 0
}

func queryUint64(request *stdhttp.Request, key string) (uint64, bool) {
	id, err := strconv.ParseUint(request.URL.Query().Get(key), 10, 64)
	return id, err == nil && id != 0
}

func optionalQueryUint64(request *stdhttp.Request, key string) (uint64, bool) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseUint(value, 10, 64)
	return id, err == nil && id != 0
}

func queryInt(request *stdhttp.Request, key string) (int, bool) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return 0, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func decodeJSON(request *stdhttp.Request, target any) bool {
	if request == nil || request.Body == nil {
		return false
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, 64*1024+1))
	if err != nil || len(payload) == 0 || len(payload) > 64*1024 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return errors.Is(decoder.Decode(&struct{}{}), io.EOF)
}

func writeError(writer stdhttp.ResponseWriter, err error) { _ = httpresponse.Failure(writer, err) }
