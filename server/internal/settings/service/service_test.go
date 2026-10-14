package service

import (
	"context"
	"encoding/json"
	"github.com/kanhai447/GIM/server/internal/settings/domain"
	"github.com/kanhai447/GIM/server/internal/settings/repository"
	"testing"
)

type memoryRepo struct{ values map[string]json.RawMessage }

func (r *memoryRepo) ListPublic(context.Context, []string) (map[string]json.RawMessage, error) {
	return r.values, nil
}
func (r *memoryRepo) UpsertPublic(_ context.Context, values []repository.Value) error {
	if r.values == nil {
		r.values = map[string]json.RawMessage{}
	}
	for _, v := range values {
		r.values[v.Key] = v.JSON
	}
	return nil
}
func TestSafeSettingsRoundTrip(t *testing.T) {
	repo := &memoryRepo{}
	service := New(repo)
	input := domain.Info{Site: domain.Site{Name: "GIM", Version: "v1", Notice: "safe", RegistrationEnabled: true}, OpenLogin: domain.OpenLogin{Enabled: false}}
	if _, err := service.Update(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	value, err := service.Get(context.Background())
	if err != nil || value.Site.Notice != "safe" {
		t.Fatalf("settings=%+v err=%v", value, err)
	}
	for key := range repo.values {
		if key == "jwt_secret" || key == "db_password" {
			t.Fatalf("unsafe key %q", key)
		}
	}
}
