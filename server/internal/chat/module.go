// Package chat assembles the Chat WebSocket connection runtime.
package chat

import (
	"errors"
	"log"

	chatconfig "github.com/kanhai447/GIM/server/internal/chat/config"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type Module struct {
	Config  chatconfig.Config
	Hub     *Hub
	Handler *Handler
}

type InboundFactory func(*Hub) (InboundHandler, error)

func New(values platformconfig.Values, transitions ...LocalConnectionTransitionObserver) (*Module, error) {
	return NewWithInboundFactory(values, func(*Hub) (InboundHandler, error) {
		return UnavailableInboundHandler{}, nil
	}, transitions...)
}

func NewWithInboundFactory(values platformconfig.Values, factory InboundFactory, transitions ...LocalConnectionTransitionObserver) (*Module, error) {
	if factory == nil {
		return nil, errors.New("chat inbound factory is required")
	}
	configuration, err := chatconfig.FromValues(values)
	if err != nil {
		return nil, err
	}
	origins, err := NewOriginPolicy(configuration.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	hub := NewHub(transitions...)
	inbound, err := factory(hub)
	if err != nil {
		return nil, err
	}
	handler, err := NewHandler(
		hub,
		inbound,
		configuration.SendBuffer,
		origins,
		HeartbeatConfig{
			ReadLimit: configuration.ReadLimit, PongWait: configuration.PongWait,
			PingPeriod: configuration.PingPeriod, WriteWait: configuration.WriteWait,
		},
		NewStandardLifecycleObserver(log.Default()),
	)
	if err != nil {
		return nil, err
	}
	return &Module{Config: configuration, Hub: hub, Handler: handler}, nil
}
