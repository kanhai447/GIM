// Command admin-bootstrap creates the first administrator for a fresh GIM
// installation. It is intentionally opt-in and never updates an existing
// account or prints credential material.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kanhai447/GIM/server/internal/auth/credential"
	mysqlclient "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/kanhai447/GIM/server/internal/user/domain"
	userrepository "github.com/kanhai447/GIM/server/internal/user/repository/mysql"
	userservice "github.com/kanhai447/GIM/server/internal/user/service"
)

func main() {
	if err := run(); err != nil {
		log.Printf("admin bootstrap failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	account := strings.TrimSpace(os.Getenv("GIM_ADMIN_ACCOUNT"))
	nickname := strings.TrimSpace(os.Getenv("GIM_ADMIN_NICKNAME"))
	password := os.Getenv("GIM_ADMIN_PASSWORD")
	if account == "" || nickname == "" || len(password) < 8 || len(password) > 72 || !utf8.ValidString(password) {
		return errors.New("GIM_ADMIN_ACCOUNT, GIM_ADMIN_NICKNAME, and a valid GIM_ADMIN_PASSWORD are required")
	}

	values, err := process.LoadValues(os.Args[1:])
	if err != nil {
		return err
	}
	databaseConfig, err := mysqlclient.FromValues(values)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mysqlclient.Open(ctx, databaseConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	hash, err := credential.NewPasswords(credential.DefaultBcryptCost).Hash(password)
	if err != nil {
		return errors.New("password hashing failed")
	}
	users := userservice.New(userrepository.New(client.DB()))
	user, err := users.CreateUser(ctx, userservice.CreateUserInput{
		Account: account, Nickname: nickname, PasswordHash: hash,
		Role: domain.RoleAdmin, Status: domain.StatusActive,
	})
	if err != nil {
		return err
	}
	fmt.Printf("administrator %s created with role admin\n", user.Account)
	return nil
}
