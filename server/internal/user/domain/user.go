// Package domain defines the GIM user domain without transport or database dependencies.
package domain

import "time"

type Role int32

const (
	RoleAdmin  Role = 1
	RoleMember Role = 2
)

func (role Role) Valid() bool {
	return role == RoleAdmin || role == RoleMember
}

type Status int32

const (
	StatusActive   Status = 1
	StatusDisabled Status = 2
)

func (status Status) Valid() bool {
	return status == StatusActive || status == StatusDisabled
}

// User is the internal aggregate. PasswordHash is intentionally excluded from JSON.
type User struct {
	ID             uint64    `json:"id"`
	Account        string    `json:"account"`
	PasswordHash   string    `json:"-"`
	Nickname       string    `json:"nickname"`
	Abstract       string    `json:"abstract"`
	Avatar         string    `json:"avatar"`
	IP             string    `json:"ip"`
	Address        string    `json:"address"`
	OpenID         string    `json:"-"`
	RegisterSource string    `json:"registerSource"`
	Role           Role      `json:"role"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// UserInfo is safe for public HTTP and general business RPC responses.
type UserInfo struct {
	ID       uint64 `json:"id"`
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Role     Role   `json:"role"`
	Status   Status `json:"status"`
}

func (user User) PublicInfo() UserInfo {
	return UserInfo{
		ID:       user.ID,
		Account:  user.Account,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Role:     user.Role,
		Status:   user.Status,
	}
}
