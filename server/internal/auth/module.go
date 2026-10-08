// Package auth assembles the Auth module through explicit dependency injection.
package auth

import (
	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	authconfig "github.com/kanhai447/GIM/server/internal/auth/config"
	"github.com/kanhai447/GIM/server/internal/auth/credential"
	"github.com/kanhai447/GIM/server/internal/auth/revocation"
	authservice "github.com/kanhai447/GIM/server/internal/auth/service"
	"github.com/kanhai447/GIM/server/internal/auth/token"
	"github.com/kanhai447/GIM/server/internal/auth/userclient"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
	redisv9 "github.com/redis/go-redis/v9"
)

type Module struct{ Service *authservice.Service }

func New(values platformconfig.Values, users userv1.UserServiceClient, redisClient *redisv9.Client) (*Module, error) {
	cfg, err := authconfig.FromValues(values)
	if err != nil {
		return nil, err
	}
	tokens, err := token.NewManager(cfg.JWTSecret, cfg.JWTExpiry)
	if err != nil {
		return nil, err
	}
	service := authservice.New(
		userclient.New(users),
		credential.NewPasswords(credential.DefaultBcryptCost),
		tokens,
		revocation.NewRedisStore(redisClient),
		authservice.DefaultAllowlist(),
	)
	return &Module{Service: service}, nil
}
