// Package chat assembles the Chat WebSocket connection runtime.
package chat

import (
	"log"

	chatconfig "github.com/kanhai447/GIM/server/internal/chat/config"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type Module struct {
	Config  chatconfig.Config
	Hub     *Hub
	Handler *Handler
}

func New(values platformconfig.Values, transitions ...LocalConnectionTransitionObserver) (*Module, error) {
	configuration, err := chatconfig.FromValues(values)
	if err != nil {
		return nil, err
	}
	origins, err := NewOriginPolicy(configuration.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	hub := NewHub(transitions...)
	handler, err := NewHandler(
		hub,
		UnavailableInboundHandler{},
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
