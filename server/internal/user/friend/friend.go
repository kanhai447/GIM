// Package friend implements the GIM V1 friendship lifecycle: user search,
// friend requests, acceptance/rejection, remark updates and deletion.
//
// The package owns no tables of its own: it builds on the existing `friends`
// and `friend_verifies` schema created by migration 002, and reads the Redis
// Presence contribution maintained by the Chat WebSocket.
package friend

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/platform/apperror"
	"github.com/kanhai447/GIM/server/internal/user/domain"
)

const (
	CodeInvalid          uint32 = 1001
	CodeUserNotFound     uint32 = 1101
	CodeSelfTarget       uint32 = 1113
	CodeAlreadyFriend    uint32 = 1114
	CodeNotFriend        uint32 = 1115
	CodeDuplicateRequest uint32 = 1116
	CodeRequestNotFound  uint32 = 1117
	CodeRequestHandled   uint32 = 1118
	CodeNotReceiver      uint32 = 1119

	StatusPending  = 0
	StatusAccepted = 1
	StatusRejected = 2

	DefaultLimit = 20
	MaxLimit     = 50

	maxKeywordRunes = 32
	maxMessageRunes = 64
	maxRemarkRunes  = 32

	presenceTimeout = 500 * time.Millisecond
)

var (
	ErrSelfTarget       = errors.New("friend: cannot target self")
	ErrAlreadyFriend    = errors.New("friend: users are already friends")
	ErrNotFriend        = errors.New("friend: users are not friends")
	ErrDuplicateRequest = errors.New("friend: a pending request already exists")
	ErrRequestNotFound  = errors.New("friend: friend request not found")
	ErrRequestHandled   = errors.New("friend: friend request already handled")
	ErrNotReceiver      = errors.New("friend: friend request belongs to another user")
)

// Friend is one row of the current user's contact list.
type Friend struct {
	UserID    uint64    `json:"userID"`
	Account   string    `json:"account"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Abstract  string    `json:"abstract"`
	Remark    string    `json:"remark"`
	IsOnline  bool      `json:"isOnline"`
	CreatedAt time.Time `json:"createdAt"`
}

// Request is a friend verification request. Status uses the reference
// compatible values 0=pending, 1=accepted, 2=rejected.
type Request struct {
	ID         uint64    `json:"id"`
	Requester  Brief     `json:"requester"`
	ReceiverID uint64    `json:"receiverID"`
	Status     int       `json:"status"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Brief struct {
	UserID   uint64 `json:"userID"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// SearchResult is a searchable public profile plus the current user's
// relationship to it.
type SearchResult struct {
	UserID   uint64 `json:"userID"`
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Abstract string `json:"abstract"`
	IsFriend bool   `json:"isFriend"`
}

// FriendRow is the joined friends/users projection returned by the repository.
type FriendRow struct {
	FriendID  uint64
	Remark    string
	Account   string
	Nickname  string
	Avatar    string
	Abstract  string
	CreatedAt time.Time
}

// VerificationRow is the joined friend_verifies/users projection.
type VerificationRow struct {
	ID                uint64
	RequesterID       uint64
	ReceiverID        uint64
	Status            int
	Message           string
	RequesterNickname string
	RequesterAvatar   string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Repository persists friendship state. Implementations must treat
// (user_id, friend_id) and the pending-pair unique key as the final guards.
type Repository interface {
	SearchUsers(ctx context.Context, userID uint64, keyword string, limit int) ([]domain.User, error)
	ListFriends(ctx context.Context, userID uint64, limit int) ([]FriendRow, error)
	GetFriend(ctx context.Context, userID, friendID uint64) (FriendRow, error)
	FriendshipFlags(ctx context.Context, userID uint64, otherIDs []uint64) (map[uint64]bool, error)
	CreateRequest(ctx context.Context, requesterID, receiverID uint64, message string) (VerificationRow, error)
	ListRequests(ctx context.Context, receiverID uint64, status int, limit int) ([]VerificationRow, error)
	Respond(ctx context.Context, requestID, reviewerID uint64, accept bool) (VerificationRow, error)
	UpdateRemark(ctx context.Context, userID, friendID uint64, remark string) error
	DeleteFriend(ctx context.Context, userID, friendID uint64) error
}

// Directory resolves public user profiles for friend targets.
type Directory interface {
	GetUserByID(ctx context.Context, userID uint64) (domain.User, error)
}

// Presence reports the authoritative Redis Presence maintained by Chat WS.
type Presence interface {
	IsOnline(ctx context.Context, userID uint64, now time.Time) (bool, error)
}

type Service struct {
	repository Repository
	directory  Directory
	presence   Presence
	now        func() time.Time
}

func New(repository Repository, directory Directory, presence Presence) *Service {
	return &Service{repository: repository, directory: directory, presence: presence, now: time.Now}
}

// Search finds active users by account/nickname (or exact numeric ID), excluding
// the caller, and marks which results are already friends.
func (service *Service) Search(ctx context.Context, userID uint64, keyword string, limit int) ([]SearchResult, error) {
	if userID == 0 {
		return nil, apperror.Business(CodeInvalid, "参数错误")
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > maxKeywordRunes || !utf8.ValidString(keyword) {
		return nil, apperror.Business(CodeInvalid, "搜索关键词无效")
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}
	users, err := service.repository.SearchUsers(ctx, userID, keyword, limit)
	if err != nil {
		return nil, translate(err)
	}
	ids := make([]uint64, 0, len(users))
	for _, user := range users {
		if user.ID != userID {
			ids = append(ids, user.ID)
		}
	}
	flags, err := service.repository.FriendshipFlags(ctx, userID, ids)
	if err != nil {
		return nil, translate(err)
	}
	results := make([]SearchResult, 0, len(ids))
	for _, user := range users {
		if user.ID == userID {
			continue
		}
		results = append(results, SearchResult{
			UserID: user.ID, Account: user.Account, Nickname: user.Nickname,
			Avatar: user.Avatar, Abstract: user.Abstract, IsFriend: flags[user.ID],
		})
	}
	return results, nil
}

// SendRequest creates one pending friend verification. The pending-pair unique
// key rejects a second pending request in either direction.
func (service *Service) SendRequest(ctx context.Context, requesterID, friendID uint64, message string) (Request, error) {
	if err := validatePair(requesterID, friendID); err != nil {
		return Request{}, err
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > maxMessageRunes || !utf8.ValidString(message) {
		return Request{}, apperror.Business(CodeInvalid, "验证消息过长")
	}
	if _, err := service.directory.GetUserByID(ctx, friendID); err != nil {
		return Request{}, translate(err)
	}
	row, err := service.repository.CreateRequest(ctx, requesterID, friendID, message)
	if err != nil {
		return Request{}, translate(err)
	}
	return requestDTO(row), nil
}

// ListRequests returns requests received by the current user.
func (service *Service) ListRequests(ctx context.Context, userID uint64, status int, limit int) ([]Request, error) {
	if userID == 0 {
		return nil, apperror.Business(CodeInvalid, "参数错误")
	}
	if status != -1 && status != StatusPending && status != StatusAccepted && status != StatusRejected {
		return nil, apperror.Business(CodeInvalid, "申请状态无效")
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}
	rows, err := service.repository.ListRequests(ctx, userID, status, limit)
	if err != nil {
		return nil, translate(err)
	}
	items := make([]Request, 0, len(rows))
	for _, row := range rows {
		items = append(items, requestDTO(row))
	}
	return items, nil
}

// Respond accepts or rejects one received request. Acceptance and the two
// friendship rows commit in one transaction.
func (service *Service) Respond(ctx context.Context, reviewerID, requestID uint64, status int) (Request, error) {
	if reviewerID == 0 || requestID == 0 || (status != StatusAccepted && status != StatusRejected) {
		return Request{}, apperror.Business(CodeInvalid, "申请处理参数无效")
	}
	row, err := service.repository.Respond(ctx, requestID, reviewerID, status == StatusAccepted)
	if err != nil {
		return Request{}, translate(err)
	}
	return requestDTO(row), nil
}

// ListFriends returns the current user's contacts with Redis Presence.
func (service *Service) ListFriends(ctx context.Context, userID uint64, limit int) ([]Friend, error) {
	if userID == 0 {
		return nil, apperror.Business(CodeInvalid, "参数错误")
	}
	if limit < 1 || limit > MaxLimit {
		limit = MaxLimit
	}
	rows, err := service.repository.ListFriends(ctx, userID, limit)
	if err != nil {
		return nil, translate(err)
	}
	items := make([]Friend, 0, len(rows))
	for _, row := range rows {
		items = append(items, service.friendDTO(ctx, row))
	}
	return items, nil
}

// FriendInfo returns one contact; non-friends are indistinguishable from
// non-existent users to avoid enumeration.
func (service *Service) FriendInfo(ctx context.Context, userID, friendID uint64) (Friend, error) {
	if err := validatePair(userID, friendID); err != nil {
		return Friend{}, err
	}
	row, err := service.repository.GetFriend(ctx, userID, friendID)
	if err != nil {
		return Friend{}, translate(err)
	}
	return service.friendDTO(ctx, row), nil
}

// UpdateRemark changes only the current user's directional remark.
func (service *Service) UpdateRemark(ctx context.Context, userID, friendID uint64, remark string) (Friend, error) {
	if err := validatePair(userID, friendID); err != nil {
		return Friend{}, err
	}
	remark = strings.TrimSpace(remark)
	if utf8.RuneCountInString(remark) > maxRemarkRunes || !utf8.ValidString(remark) {
		return Friend{}, apperror.Business(CodeInvalid, "备注过长")
	}
	if err := service.repository.UpdateRemark(ctx, userID, friendID, remark); err != nil {
		return Friend{}, translate(err)
	}
	row, err := service.repository.GetFriend(ctx, userID, friendID)
	if err != nil {
		return Friend{}, translate(err)
	}
	return service.friendDTO(ctx, row), nil
}

// DeleteFriend removes both directions of the relationship.
func (service *Service) DeleteFriend(ctx context.Context, userID, friendID uint64) error {
	if err := validatePair(userID, friendID); err != nil {
		return err
	}
	return translate(service.repository.DeleteFriend(ctx, userID, friendID))
}

func (service *Service) friendDTO(ctx context.Context, row FriendRow) Friend {
	return Friend{
		UserID: row.FriendID, Account: row.Account, Nickname: row.Nickname, Avatar: row.Avatar,
		Abstract: row.Abstract, Remark: row.Remark, IsOnline: service.online(ctx, row.FriendID), CreatedAt: row.CreatedAt,
	}
}

// online degrades to offline when Presence is unavailable or unconfigured: a
// Redis hiccup must not make the contact list unusable.
func (service *Service) online(ctx context.Context, userID uint64) bool {
	if service.presence == nil {
		return false
	}
	presenceContext, cancel := context.WithTimeout(ctx, presenceTimeout)
	defer cancel()
	online, err := service.presence.IsOnline(presenceContext, userID, service.now())
	if err != nil {
		return false
	}
	return online
}

func requestDTO(row VerificationRow) Request {
	return Request{
		ID: row.ID, Requester: Brief{UserID: row.RequesterID, Nickname: row.RequesterNickname, Avatar: row.RequesterAvatar},
		ReceiverID: row.ReceiverID, Status: row.Status, Message: row.Message,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func validatePair(userID, friendID uint64) error {
	if userID == 0 || friendID == 0 {
		return apperror.Business(CodeInvalid, "参数错误")
	}
	if userID == friendID {
		return apperror.Business(CodeSelfTarget, "不能添加自己为好友")
	}
	return nil
}

func translate(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	switch {
	case errors.Is(err, ErrSelfTarget):
		return apperror.Business(CodeSelfTarget, "不能添加自己为好友")
	case errors.Is(err, ErrAlreadyFriend):
		return apperror.Business(CodeAlreadyFriend, "对方已经是好友")
	case errors.Is(err, ErrNotFriend):
		return apperror.Business(CodeNotFriend, "对方不是好友")
	case errors.Is(err, ErrDuplicateRequest):
		return apperror.Business(CodeDuplicateRequest, "已有待处理的好友申请")
	case errors.Is(err, ErrRequestNotFound):
		return apperror.Business(CodeRequestNotFound, "好友申请不存在")
	case errors.Is(err, ErrRequestHandled):
		return apperror.Business(CodeRequestHandled, "好友申请已处理")
	case errors.Is(err, ErrNotReceiver):
		return apperror.Business(CodeNotReceiver, "无权处理该好友申请")
	default:
		return apperror.Wrap(err, http.StatusInternalServerError, apperror.CodeInternal, "服务器错误")
	}
}
