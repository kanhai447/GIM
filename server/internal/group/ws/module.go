// Package ws implements the Group WebSocket connection runtime. It has no
// Presence dependency and does not bind a connection to a single group.
package ws

import (
	"errors"
	"log"

	groupconfig "github.com/kanhai447/GIM/server/internal/group/config"
)

type InboundFactory func(*Hub) (InboundHandler, error)

type Module struct {
	Hub     *Hub
	Handler *Handler
}

func New(configuration groupconfig.WebSocketConfig) (*Module, error) {
	return NewWithInboundFactory(configuration, func(hub *Hub) (InboundHandler, error) { return NewFoundationInbound(hub), nil })
}

func NewWithInboundFactory(configuration groupconfig.WebSocketConfig, factory InboundFactory) (*Module, error) {
	if factory == nil {
		return nil, errors.New("group websocket inbound factory is required")
	}
	origins, err := NewOriginPolicy(configuration.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	hub := NewHub()
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
	return &Module{Hub: hub, Handler: handler}, nil
}
