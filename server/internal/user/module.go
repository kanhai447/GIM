// Package user assembles the User domain through explicit dependency injection.
package user

import (
	userrepo "github.com/kanhai447/GIM/server/internal/user/repository/mysql"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
	"gorm.io/gorm"
)

type Module struct {
	Service *userservice.Service
}

func New(database *gorm.DB) *Module {
	repository := userrepo.New(database)
	return &Module{Service: userservice.New(repository)}
}
