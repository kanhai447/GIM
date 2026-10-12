// Package ws implements the Group WebSocket connection runtime. It has no
// Presence dependency and does not bind a connection to a single group.
package ws

import (
	"log"

	groupconfig "github.com/kanhai447/GIM/server/internal/group/config"
)

type Module struct {
	Hub     *Hub
	Handler *Handler
}

func New(configuration groupconfig.WebSocketConfig) (*Module, error) {
	origins, err := NewOriginPolicy(configuration.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	hub := NewHub()
	handler, err := NewHandler(
		hub,
		NewFoundationInbound(hub),
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
