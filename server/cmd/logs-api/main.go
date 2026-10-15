package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kanhai447/GIM/server/internal/logs/consumer"
	logsmysql "github.com/kanhai447/GIM/server/internal/logs/repository/mysql"
	logsservice "github.com/kanhai447/GIM/server/internal/logs/service"
	logshttp "github.com/kanhai447/GIM/server/internal/logs/transport/http"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	"github.com/kanhai447/GIM/server/internal/platform/kafkaaudit"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("logs api stopped: %v", err)
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
	kafkaConfig, err := kafkaaudit.FromValues(values)
	if err != nil {
		return err
	}
	registrationConfig, err := discovery.RegistrationFromValues(values, "LOGS", "logs_api")
	if err != nil {
		return err
	}
	restConfig, err := serviceruntime.RESTConfig(values, registrationConfig, "gim-logs-api")
	if err != nil {
		return err
	}

	startupContext, startupCancel := context.WithTimeout(context.Background(), runtimeConfig.StartupTimeout)
	defer startupCancel()
	mysqlClient, err := platformmysql.Open(startupContext, mysqlConfig)
	if err != nil {
		return err
	}
	defer mysqlClient.Close()
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	repository := logsmysql.New(mysqlClient.DB())
	logService := logsservice.New(repository)
	auditConsumer, err := consumer.New(kafkaConfig, repository, log.Default())
	if err != nil {
		return err
	}
	if auditConsumer != nil {
		defer auditConsumer.Close()
	}
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	serviceruntime.RegisterHealthz(server)
	logshttp.RegisterRoutes(server, logService)

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
		cleanupContext, cancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cancel()
		if closeErr := registration.Close(cleanupContext); closeErr != nil {
			log.Printf("logs api registration cleanup failed")
		}
	}()

	consumerDone := make(chan error, 1)
	if auditConsumer != nil {
		go func() { consumerDone <- auditConsumer.Run(lifecycleContext) }()
	} else {
		close(consumerDone)
	}
	startupCancel()
	serveErr := serviceruntime.ServeREST(lifecycleContext, server, runtimeConfig.ShutdownTimeout)
	lifecycleCancel()
	if auditConsumer != nil {
		_ = auditConsumer.Close()
	}
	var consumerErr error
	select {
	case consumerErr = <-consumerDone:
	case <-time.After(runtimeConfig.ShutdownTimeout):
		consumerErr = context.DeadlineExceeded
	}
	return errors.Join(serveErr, consumerErr)
}
