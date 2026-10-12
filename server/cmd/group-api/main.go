package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/group"
	groupconfig "github.com/kanhai447/GIM/server/internal/group/config"
	grouphttp "github.com/kanhai447/GIM/server/internal/group/transport/http"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	"github.com/zeromicro/go-zero/rest"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("group api stopped: %v", err)
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
	groupConfig, err := groupconfig.FromValues(values)
	if err != nil {
		return err
	}
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
	registrationConfig, err := discovery.RegistrationFromValues(values, "GROUP", "group_api")
	if err != nil {
		return err
	}
	restConfig, err := serviceruntime.RESTConfig(registrationConfig, "gim-group-api")
	if err != nil {
		return err
	}
	userRPCAddress, err := serviceruntime.RPCAddress(values, "USER")
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
	redisClient, err := platformredis.Open(startupContext, redisConfig)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	userConnection, err := googlegrpc.NewClient(userRPCAddress, googlegrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create user rpc client: %w", err)
	}
	defer userConnection.Close()
	if _, err := healthv1.NewHealthClient(userConnection).Check(startupContext, &healthv1.HealthCheckRequest{}); err != nil {
		return fmt.Errorf("user rpc health check failed")
	}
	groupModule, err := group.New(mysqlClient.DB(), redisClient.Raw(), userv1.NewUserServiceClient(userConnection), groupConfig.DependencyTimeout, groupConfig.RedisTimeout)
	if err != nil {
		return err
	}
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	grouphttp.RegisterRoutes(server, groupModule.Service)
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
		cleanupContext, cancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cancel()
		if err := registration.Close(cleanupContext); err != nil {
			log.Printf("group api registration cleanup failed: %v", err)
		}
	}()
	return serviceruntime.ServeREST(lifecycleContext, server, runtimeConfig.ShutdownTimeout)
}
