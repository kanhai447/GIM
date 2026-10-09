// Package chat assembles the Chat WebSocket foundation.
package chat

import (
	chatconfig "github.com/kanhai447/GIM/server/internal/chat/config"
	platformconfig "github.com/kanhai447/GIM/server/internal/platform/config"
)

type Module struct {
	Config  chatconfig.Config
	Hub     *Hub
	Handler *Handler
}

func New(values platformconfig.Values) (*Module, error) {
	configuration, err := chatconfig.FromValues(values)
	if err != nil {
		return nil, err
	}
	origins, err := NewOriginPolicy(configuration.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	hub := NewHub()
	handler := NewHandler(hub, UnavailableInboundHandler{}, configuration.SendBuffer, origins)
	return &Module{Config: configuration, Hub: hub, Handler: handler}, nil
}
