package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	files "github.com/kanhai447/GIM/server/internal/file"
	fileadmin "github.com/kanhai447/GIM/server/internal/file/admin"
	fileadminmysql "github.com/kanhai447/GIM/server/internal/file/admin/mysql"
	fileadminhttp "github.com/kanhai447/GIM/server/internal/file/admin/transport/http"
	fileconfig "github.com/kanhai447/GIM/server/internal/file/config"
	filehttp "github.com/kanhai447/GIM/server/internal/file/transport/http"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	"github.com/kanhai447/GIM/server/internal/platform/kafkaaudit"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("file api stopped: %v", err)
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
			log.Printf("file audit producer shutdown incomplete")
		}
	}()
	fileConfig, err := fileconfig.FromValues(values)
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
	registrationConfig, err := discovery.RegistrationFromValues(values, "FILE", "file_api")
	if err != nil {
		return err
	}
	restConfig, err := serviceruntime.RESTConfig(values, registrationConfig, "gim-file-api")
	if err != nil {
		return err
	}
	restConfig.MaxBytes = fileConfig.MaxUploadBytes + (1 << 20)
	startupContext, startupCancel := context.WithTimeout(context.Background(), runtimeConfig.StartupTimeout)
	defer startupCancel()
	mysqlClient, err := platformmysql.Open(startupContext, mysqlConfig)
	if err != nil {
		return err
	}
	defer mysqlClient.Close()
	module, err := files.New(mysqlClient.DB(), fileConfig.StorageRoot, fileConfig.MaxUploadBytes)
	if err != nil {
		return err
	}
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	serviceruntime.RegisterHealthz(server)
	filehttp.RegisterRoutes(server, module.Service, fileConfig.MaxUploadBytes, auditProducer)
	fileadminhttp.RegisterRoutes(server, fileadmin.New(fileadminmysql.New(mysqlClient.DB())), module.Service, auditProducer)
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	if err != nil {
		return err
	}
	defer etcdClient.Close()
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
		cleanup, cancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cancel()
		if err := registration.Close(cleanup); err != nil {
			log.Printf("file api registration cleanup failed: %v", err)
		}
	}()
	return errors.Join(serviceruntime.ServeREST(lifecycleContext, server, runtimeConfig.ShutdownTimeout))
}
