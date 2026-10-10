package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	userv1 "github.com/kanhai447/GIM/server/api/user/v1"
	"github.com/kanhai447/GIM/server/internal/chat"
	chatconfig "github.com/kanhai447/GIM/server/internal/chat/config"
	"github.com/kanhai447/GIM/server/internal/chat/delivery"
	"github.com/kanhai447/GIM/server/internal/chat/message"
	chatmysql "github.com/kanhai447/GIM/server/internal/chat/message/repository/mysql"
	"github.com/kanhai447/GIM/server/internal/chat/presence"
	chatuserclient "github.com/kanhai447/GIM/server/internal/chat/userclient"
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
	redisConfig, err := platformredis.FromValues(values)
	if err != nil {
		return err
	}
	presenceConfig, err := presence.FromValues(values, registrationConfig.InstanceID)
	if err != nil {
		return err
	}
	chatConfiguration, err := chatconfig.FromValues(values)
	if err != nil {
		return err
	}
	mysqlConfig, err := platformmysql.FromValues(values)
	if err != nil {
		return err
	}
	userRPCAddress, err := serviceruntime.RPCAddress(values, "USER")
	if err != nil {
		return err
	}
	startupContext, startupCancel := context.WithTimeout(context.Background(), runtimeConfig.StartupTimeout)
	defer startupCancel()
	redisClient, err := platformredis.Open(startupContext, redisConfig)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	mysqlClient, err := platformmysql.Open(startupContext, mysqlConfig)
	if err != nil {
		return err
	}
	defer mysqlClient.Close()
	userConnection, err := googlegrpc.NewClient(userRPCAddress, googlegrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create user rpc client: %w", err)
	}
	defer userConnection.Close()
	if _, err := healthv1.NewHealthClient(userConnection).Check(startupContext, &healthv1.HealthCheckRequest{}); err != nil {
		return fmt.Errorf("user rpc health check failed")
	}
	presenceService, err := presence.NewService(presence.NewRedisStore(redisClient.Raw()), presenceConfig, log.Default())
	if err != nil {
		return err
	}
	deliveryBus, err := delivery.New(redisClient.Raw(), chatConfiguration.DeliveryTimeout, log.Default())
	if err != nil {
		return err
	}
	messageConfig := message.DefaultConfig()
	messageConfig.MaxTextBytes = chatConfiguration.MaxTextBytes
	messageConfig.MaxPayloadBytes = chatConfiguration.MaxPayloadBytes
	messageConfig.DependencyTimeout = chatConfiguration.DependencyTimeout
	messageService, err := message.NewService(
		chatmysql.New(mysqlClient.DB()), chatuserclient.New(userv1.NewUserServiceClient(userConnection)),
		chatmysql.NewFriendshipRepository(mysqlClient.DB()), messageConfig,
	)
	if err != nil {
		return err
	}
	chatModule, err := chat.NewWithInboundFactory(values, func(hub *chat.Hub) (chat.InboundHandler, error) {
		return message.NewInbound(messageService, hub, deliveryBus, log.Default())
	}, presenceService)
	if err != nil {
		return err
	}
	subscription, err := deliveryBus.Subscribe(startupContext, message.LocalDeliveryHandler(chatModule.Hub, chatConfiguration.DeliveryTimeout, log.Default()))
	if err != nil {
		return err
	}
	defer subscription.Close()
	server, err := rest.NewServer(restConfig)
	if err != nil {
		return err
	}
	chat.RegisterRoute(server, chatModule.Config.Path, chatModule.Handler)

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
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
		defer cleanupCancel()
		if err := registration.Close(cleanupContext); err != nil {
			log.Printf("chat api registration cleanup failed: %v", err)
		}
	}()

	hubDone := make(chan struct{})
	presenceContext, presenceCancel := context.WithCancel(context.Background())
	presenceDone := make(chan struct{})
	deliveryDone := make(chan struct{})
	go func() {
		subscription.Run(lifecycleContext)
		close(deliveryDone)
	}()
	go func() {
		presenceService.Run(presenceContext)
		close(presenceDone)
	}()
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
	presenceShutdownContext, presenceShutdownCancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
	presenceErr := presenceService.Shutdown(presenceShutdownContext)
	presenceCancel()
	select {
	case <-presenceDone:
	case <-presenceShutdownContext.Done():
		if presenceErr == nil {
			presenceErr = presenceShutdownContext.Err()
		}
	}
	presenceShutdownCancel()
	select {
	case <-deliveryDone:
	case <-shutdownContext.Done():
	}
	return errors.Join(serveErr, hubErr, presenceErr)
}
