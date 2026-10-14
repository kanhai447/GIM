package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kanhai447/GIM/server/internal/chat/presence"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	"github.com/kanhai447/GIM/server/internal/platform/kafkaaudit"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/kanhai447/GIM/server/internal/user"
	useradmin "github.com/kanhai447/GIM/server/internal/user/admin"
	useradminmysql "github.com/kanhai447/GIM/server/internal/user/admin/mysql"
	useradminhttp "github.com/kanhai447/GIM/server/internal/user/admin/transport/http"
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
			log.Printf("user audit producer shutdown incomplete")
		}
	}()
	mysqlConfig, err := platformmysql.FromValues(values)
	if err != nil {
		return err
	}
	redisConfig, err := platformredis.FromValues(values)
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
	redisClient, err := platformredis.Open(startupContext, redisConfig)
	if err != nil {
		startupCancel()
		return err
	}
	defer redisClient.Close()
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	startupCancel()
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	users := user.New(mysqlClient.DB())
	adminUsers := useradmin.New(useradminmysql.New(mysqlClient.DB()), presence.NewRedisStore(redisClient.Raw()))
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	userhttp.RegisterRoutes(server, users.Service)
	useradminhttp.RegisterRoutes(server, adminUsers, auditProducer)

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
