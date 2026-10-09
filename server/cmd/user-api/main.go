package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/kanhai447/GIM/server/internal/user"
	userhttp "github.com/kanhai447/GIM/server/internal/user/transport/http"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("user api stopped: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	values, err := serviceruntime.LoadValues(args)
	if err != nil {
		return err
	}
	runtimeConfig, err := serviceruntime.FromValues(values)
	if err != nil {
		return err
	}
	mysqlConfig, err := platformmysql.FromValues(values)
	if err != nil {
		return err
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		return err
	}
	registrationConfig, err := discovery.RegistrationFromValues(values, "USER", "user_api")
	if err != nil {
		return err
	}
	restConfig, err := serviceruntime.RESTConfig(registrationConfig, "gim-user-api")
	if err != nil {
		return err
	}

	startupContext, startupCancel := context.WithTimeout(context.Background(), runtimeConfig.StartupTimeout)
	mysqlClient, err := platformmysql.Open(startupContext, mysqlConfig)
	if err != nil {
		startupCancel()
		return err
	}
	defer mysqlClient.Close()
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	startupCancel()
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	users := user.New(mysqlClient.DB())
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	userhttp.RegisterRoutes(server, users.Service)

	registry, err := discovery.NewRegistry(etcdClient.Raw(), registrationConfig.LeaseTTL)
	if err != nil {
		return err
	}
	lifecycleContext, lifecycleCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	registration, err := registry.Register(
		lifecycleContext,
		registrationConfig.Service,
		registrationConfig.InstanceID,
		registrationConfig.Endpoint,
	)
	if err != nil {
		lifecycleCancel()
		return err
	}
	defer func() {
		lifecycleCancel()
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cleanupCancel()
		if err := registration.Close(cleanupContext); err != nil {
			log.Printf("user api registration cleanup failed: %v", err)
		}
	}()

	return serviceruntime.ServeREST(lifecycleContext, server, runtimeConfig.ShutdownTimeout)
}
