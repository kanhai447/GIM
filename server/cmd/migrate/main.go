package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kanhai447/GIM/server/internal/platform/config"
	mysqlclient "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Printf("migration command failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env", "", "path to the ignored GIM environment file")
	command := flag.String("command", "status", "migration command: status, up, down")
	steps := flag.Int("steps", 1, "number of down migrations; use 0 for all")
	flag.Parse()
	if *envFile == "" {
		*envFile = os.Getenv("GIM_ENV_FILE")
	}
	if *envFile == "" {
		return errors.New("migration environment file is required")
	}
	values, err := config.LoadFile(*envFile)
	if err != nil {
		return err
	}
	cfg, err := mysqlclient.FromValues(values)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := mysqlclient.OpenSQL(ctx, cfg, true)
	if err != nil {
		return err
	}
	defer db.Close()
	runner, err := migrations.New(db)
	if err != nil {
		return err
	}
	switch *command {
	case "status":
		statuses, err := runner.Status(ctx)
		if err != nil {
			return err
		}
		for _, status := range statuses {
			state := "pending"
			if status.Applied {
				state = "applied"
			}
			if status.Dirty {
				state = "dirty"
			}
			fmt.Printf("%03d_%s %s\n", status.Version, status.Name, state)
		}
		return nil
	case "up":
		return runner.Up(ctx)
	case "down":
		if *steps == 0 {
			return runner.DownAll(ctx)
		}
		return runner.Down(ctx, *steps)
	default:
		return errors.New("migration command must be status, up, or down")
	}
}
