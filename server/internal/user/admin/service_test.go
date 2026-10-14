package admin

import (
	"context"
	"testing"

	"github.com/kanhai447/GIM/server/internal/user/domain"
)

type statusRepository struct {
	userID uint64
	status domain.Status
	err    error
}

func (repository *statusRepository) List(context.Context, Query) ([]User, error) {
	return nil, nil
}

func (repository *statusRepository) UpdateRestrictions(context.Context, Restrictions) error {
	return nil
}

func (repository *statusRepository) UpdateStatus(_ context.Context, userID uint64, status domain.Status) error {
	repository.userID = userID
	repository.status = status
	return repository.err
}

func TestDisableUsesFormalDisabledStatus(t *testing.T) {
	repository := &statusRepository{}
	service := New(repository, nil)
	if err := service.Disable(context.Background(), 1, 7); err != nil {
		t.Fatal(err)
	}
	if repository.userID != 7 || repository.status != domain.StatusDisabled {
		t.Fatalf("status update = user %d status %d", repository.userID, repository.status)
	}
}

func TestEnableUsesFormalActiveStatus(t *testing.T) {
	repository := &statusRepository{}
	service := New(repository, nil)
	if err := service.Enable(context.Background(), 1, 7); err != nil {
		t.Fatal(err)
	}
	if repository.userID != 7 || repository.status != domain.StatusActive {
		t.Fatalf("status update = user %d status %d", repository.userID, repository.status)
	}
}

func TestDisableRejectsSelfActionWithoutUpdatingStatus(t *testing.T) {
	repository := &statusRepository{}
	service := New(repository, nil)
	if err := service.Disable(context.Background(), 1, 1); err != ErrSelfAction {
		t.Fatalf("disable self error = %v", err)
	}
	if repository.userID != 0 {
		t.Fatalf("repository called for user %d", repository.userID)
	}
}
