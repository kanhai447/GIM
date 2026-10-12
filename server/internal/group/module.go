// Package group assembles Group foundation dependencies explicitly.
package group

import (
	"time"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/group/mute"
	groupmysql "github.com/kanhai447/GIM/server/internal/group/repository/mysql"
	groupservice "github.com/kanhai447/GIM/server/internal/group/service"
	"github.com/kanhai447/GIM/server/internal/group/userclient"
	redisv9 "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Module struct{ Service *groupservice.Service }

func New(database *gorm.DB, redis redisv9.Cmdable, users userv1.UserServiceClient, dependencyTimeout, redisTimeout time.Duration) (*Module, error) {
	mutes, err := mute.NewRedisStore(redis, redisTimeout)
	if err != nil {
		return nil, err
	}
	service, err := groupservice.New(groupmysql.New(database), userclient.New(users), mutes, groupservice.Config{DependencyTimeout: dependencyTimeout})
	if err != nil {
		return nil, err
	}
	return &Module{Service: service}, nil
}
