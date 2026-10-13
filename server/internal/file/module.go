// Package file assembles File service dependencies explicitly.
package file

import (
	filemysql "github.com/kanhai447/GIM/server/internal/file/repository/mysql"
	fileservice "github.com/kanhai447/GIM/server/internal/file/service"
	filestorage "github.com/kanhai447/GIM/server/internal/file/storage"
	"gorm.io/gorm"
)

type Module struct{ Service *fileservice.Service }

func New(database *gorm.DB, root string, maxUploadBytes int64) (*Module, error) {
	local, err := filestorage.NewLocal(root)
	if err != nil {
		return nil, err
	}
	service, err := fileservice.New(filemysql.New(database), local, fileservice.Config{MaxUploadBytes: maxUploadBytes})
	if err != nil {
		return nil, err
	}
	return &Module{Service: service}, nil
}
