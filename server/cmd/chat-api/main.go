package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/platform/discovery"
	platformetcd "github.com/kanhai447/GIM/server/internal/platform/etcd"
	serviceruntime "github.com/kanhai447/GIM/server/internal/platform/process"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("chat api stopped: %v", err)
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
	etcdConfig, err := platformetcd.FromValues(values)
	if err != nil {
		return err
	}
	registrationConfig, err := discovery.RegistrationFromValues(values, "CHAT", "chat_api")
	if err != nil {
		return err
	}
	restConfig, err := serviceruntime.RESTConfig(registrationConfig, "gim-chat-api")
	if err != nil {
		return err
	}
	chatModule, err := chat.New(values)
	if err != nil {
		return err
	}
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	chat.RegisterRoute(server, chatModule.Config.Path, chatModule.Handler)

	startupContext, startupCancel := context.WithTimeout(context.Background(), runtimeConfig.StartupTimeout)
	etcdClient, err := platformetcd.Open(startupContext, etcdConfig)
	startupCancel()
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
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cleanupCancel()
		if err := registration.Close(cleanupContext); err != nil {
			log.Printf("chat api registration cleanup failed: %v", err)
		}
	}()

	hubDone := make(chan struct{})
	go func() {
		chatModule.Hub.Run(lifecycleContext)
		close(hubDone)
	}()
	serveErr := serviceruntime.ServeREST(lifecycleContext, server, runtimeConfig.ShutdownTimeout)
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
	defer shutdownCancel()
	hubErr := chatModule.Hub.Shutdown(shutdownContext)
	select {
	case <-hubDone:
	case <-shutdownContext.Done():
		if hubErr == nil {
			hubErr = shutdownContext.Err()
		}
	}
	return errors.Join(serveErr, hubErr)
}
