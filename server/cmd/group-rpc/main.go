package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/group"
	groupconfig "github.com/kanhai447/GIM/server/internal/group/config"
	groupgrpc "github.com/kanhai447/GIM/server/internal/group/transport/grpc"
	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	platformredis "github.com/kanhai447/GIM/server/internal/platform/redis"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("group rpc stopped: %v", err)
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
	address, err := serviceruntime.RPCListenAddress(values, "GROUP")
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
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen group rpc: %w", err)
	}
	defer listener.Close()
	server := googlegrpc.NewServer()
	groupgrpc.Register(groupModule.Service)(server)
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	lifecycleContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve(listener) }()
	select {
	case err := <-serveError:
		if errors.Is(err, googlegrpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve group rpc: %w", err)
	case <-lifecycleContext.Done():
		healthServer.SetServingStatus("", healthv1.HealthCheckResponse_NOT_SERVING)
		gracefulStop(server, runtimeConfig.ShutdownTimeout)
		return nil
	}
}

func gracefulStop(server *googlegrpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() { server.GracefulStop(); close(done) }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		server.Stop()
	}
}
