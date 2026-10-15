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
	"github.com/kanhai447/GIM/server/internal/platform/kafkaaudit"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/kanhai447/GIM/server/internal/settings"
	"github.com/kanhai447/GIM/server/internal/settings/overview"
	overviewmysql "github.com/kanhai447/GIM/server/internal/settings/overview/mysql"
	overviewhttp "github.com/kanhai447/GIM/server/internal/settings/overview/transport/http"
	settingshttp "github.com/kanhai447/GIM/server/internal/settings/transport/http"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("settings api stopped: %v", err)
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
	auditConfig, err := kafkaaudit.FromValues(values)
	if err != nil {
		return err
	}
	auditProducer, err := kafkaaudit.NewProducer(auditConfig, log.Default())
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cancel()
		if closeErr := auditProducer.Close(ctx); closeErr != nil {
			log.Printf("settings audit producer shutdown incomplete")
		}
	}()
	mysqlConfig, err := platformmysql.FromValues(values)
	if err != nil {
		return err
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		return err
	}
	registrationConfig, err := discovery.RegistrationFromValues(values, "SETTINGS", "settings_api")
	if err != nil {
		return err
	}
	restConfig, err := serviceruntime.RESTConfig(values, registrationConfig, "gim-settings-api")
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
	module := settings.New(mysqlClient.DB())
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	serviceruntime.RegisterHealthz(server)
	settingshttp.RegisterRoutes(server, module.Service, auditProducer)
	overviewhttp.RegisterRoutes(server, overview.New(overviewmysql.New(mysqlClient.DB())))
	registry, err := discovery.NewRegistry(etcdClient.Raw(), registrationConfig.LeaseTTL)
	if err != nil {
		return err
	}
	lifecycleContext, lifecycleCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	registration, err := registry.Register(lifecycleContext, registrationConfig.Service, registrationConfig.InstanceID, registrationConfig.Endpoint)
	if err != nil {
		lifecycleCancel()
		return err
	}
	defer func() {
		lifecycleCancel()
		ctx, cancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cancel()
		if closeErr := registration.Close(ctx); closeErr != nil {
			log.Printf("settings api registration cleanup failed")
		}
	}()
	return serviceruntime.ServeREST(lifecycleContext, server, runtimeConfig.ShutdownTimeout)
}
