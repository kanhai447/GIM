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

	platformmysql "github.com/kanhai447/GIM/server/internal/platform/database/mysql"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/kanhai447/GIM/server/internal/user"
	usergrpc "github.com/kanhai447/GIM/server/internal/user/transport/grpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("user rpc stopped: %v", err)
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
	address, err := serviceruntime.RPCAddress(values, "USER")
	if err != nil {
		return err
	}

	startupContext, startupCancel := context.WithTimeout(context.Background(), runtimeConfig.StartupTimeout)
	mysqlClient, err := platformmysql.Open(startupContext, mysqlConfig)
	startupCancel()
	if err != nil {
		return err
	}
	defer mysqlClient.Close()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen user rpc: %w", err)
	}
	defer listener.Close()

	users := user.New(mysqlClient.DB())
	server := googlegrpc.NewServer()
	usergrpc.Register(users.Service)(server)
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
		return fmt.Errorf("serve user rpc: %w", err)
	case <-lifecycleContext.Done():
		healthServer.SetServingStatus("", healthv1.HealthCheckResponse_NOT_SERVING)
		gracefulStop(server, runtimeConfig.ShutdownTimeout)
		return nil
	}
}

func gracefulStop(server *googlegrpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		server.Stop()
	}
}
