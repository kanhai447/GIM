package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/settings/domain"
	"github.com/kanhai447/GIM/server/internal/settings/repository"
)

var ErrInvalid = errors.New("invalid public settings")

const (
	keySiteName     = "site.name"
	keySiteVersion  = "site.version"
	keySiteNotice   = "site.notice"
	keyRegistration = "site.registration_enabled"
	keyOpenLogin    = "open_login.enabled"
)

var keys = []string{keySiteName, keySiteVersion, keySiteNotice, keyRegistration, keyOpenLogin}

type Service struct{ repository repository.Repository }

func New(repository repository.Repository) *Service { return &Service{repository: repository} }
func (s *Service) Get(ctx context.Context) (domain.Info, error) {
	values, err := s.repository.ListPublic(ctx, keys)
	if err != nil {
		return domain.Info{}, err
	}
	result := domain.Defaults()
	decodeString(values[keySiteName], &result.Site.Name)
	decodeString(values[keySiteVersion], &result.Site.Version)
	decodeString(values[keySiteNotice], &result.Site.Notice)
	decodeBool(values[keyRegistration], &result.Site.RegistrationEnabled)
	decodeBool(values[keyOpenLogin], &result.OpenLogin.Enabled)
	return result, nil
}
func (s *Service) Update(ctx context.Context, value domain.Info) (domain.Info, error) {
	value.Site.Name = strings.TrimSpace(value.Site.Name)
	value.Site.Version = strings.TrimSpace(value.Site.Version)
	value.Site.Notice = strings.TrimSpace(value.Site.Notice)
	if !between(value.Site.Name, 1, 64) || !between(value.Site.Version, 1, 32) || utf8.RuneCountInString(value.Site.Notice) > 500 {
		return domain.Info{}, ErrInvalid
	}
	entries := []repository.Value{{Key: keySiteName, JSON: encode(value.Site.Name)}, {Key: keySiteVersion, JSON: encode(value.Site.Version)}, {Key: keySiteNotice, JSON: encode(value.Site.Notice)}, {Key: keyRegistration, JSON: encode(value.Site.RegistrationEnabled)}, {Key: keyOpenLogin, JSON: encode(value.OpenLogin.Enabled)}}
	if err := s.repository.UpsertPublic(ctx, entries); err != nil {
		return domain.Info{}, err
	}
	return value, nil
}
func between(value string, min, max int) bool {
	size := utf8.RuneCountInString(value)
	return size >= min && size <= max
}
func encode(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
func decodeString(data json.RawMessage, target *string) {
	if len(data) != 0 {
		_ = json.Unmarshal(data, target)
	}
}
func decodeBool(data json.RawMessage, target *bool) {
	if len(data) != 0 {
		_ = json.Unmarshal(data, target)
	}
}
