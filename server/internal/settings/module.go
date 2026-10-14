package settings

import (
	"github.com/kanhai447/GIM/server/internal/settings/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/settings/service"
	"gorm.io/gorm"
)

type Module struct{ Service *service.Service }

func New(db *gorm.DB) *Module { return &Module{Service: service.New(mysql.New(db))} }
