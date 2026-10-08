package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kanhai447/GIM/server/internal/gateway"
	"github.com/kanhai447/GIM/server/internal/platform/config"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
)

func main() {
	if err := run(); err != nil {
		log.Printf("gateway stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env", "", "path to the ignored GIM environment file")
	flag.Parse()
	if *envFile == "" {
		*envFile = os.Getenv("GIM_ENV_FILE")
	}
	if *envFile == "" {
		return errors.New("gateway environment file is required")
	}
	values, err := config.LoadFile(*envFile)
	if err != nil {
		return err
	}
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		return err
	}
	startupContext, startupCancel := context.WithTimeout(context.Background(), etcdConfig.DialTimeout)
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	startupCancel()
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	module, err := gateway.New(values, etcdClient)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              module.Config.ListenAddress,
		Handler:           module.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	lifecycleContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveError := make(chan error, 1)
	go func() {
		serveError <- server.ListenAndServe()
	}()

	select {
	case err := <-serveError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve gateway: %w", err)
	case <-lifecycleContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown gateway: %w", err)
		}
		return nil
	}
}
