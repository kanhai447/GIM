// Package overview provides the read-only Admin dashboard aggregation model.
package overview

import "context"

type Summary struct {
	UsersTotal      uint64 `json:"usersTotal"`
	UsersActive     uint64 `json:"usersActive"`
	UsersDisabled   uint64 `json:"usersDisabled"`
	PrivateMessages uint64 `json:"privateMessages"`
	GroupsActive    uint64 `json:"groupsActive"`
	GroupMessages   uint64 `json:"groupMessages"`
	UserFiles       uint64 `json:"userFiles"`
	AuditEvents     uint64 `json:"auditEvents"`
}

type Repository interface {
	Summary(context.Context) (Summary, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) Get(ctx context.Context) (Summary, error) {
	return service.repository.Summary(ctx)
}
