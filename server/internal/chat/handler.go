package chat

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/rest"
)

type Handler struct {
	hub        *Hub
	inbound    InboundHandler
	sendBuffer int
	heartbeat  HeartbeatConfig
	observer   LifecycleObserver
	upgrader   websocket.Upgrader
}

func NewHandler(
	hub *Hub,
	inbound InboundHandler,
	sendBuffer int,
	origins *OriginPolicy,
	heartbeat HeartbeatConfig,
	observer LifecycleObserver,
) (*Handler, error) {
	if hub == nil || inbound == nil || sendBuffer < 1 || origins == nil || !heartbeat.valid() {
		return nil, errors.New("invalid chat websocket handler configuration")
	}
	if observer == nil {
		observer = noopLifecycleObserver{}
	}
	return &Handler{
		hub: hub, inbound: inbound, sendBuffer: sendBuffer, heartbeat: heartbeat, observer: observer,
		upgrader: websocket.Upgrader{CheckOrigin: origins.Allows},
	}, nil
}

func RegisterRoute(server *rest.Server, path string, handler *Handler) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: path, Handler: handler.ServeHTTP})
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID, _, ok := trustedIdentity(request)
	if !ok || handler == nil || handler.hub == nil || handler.inbound == nil || handler.sendBuffer < 1 {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	connection, err := handler.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	clientID, err := newClientID()
	if err != nil {
		_ = connection.Close()
		return
	}
	client := newClient(userID, clientID, connection, handler.hub, handler.inbound, handler.sendBuffer, handler.heartbeat, handler.observer)
	if err := handler.hub.Register(request.Context(), client); err != nil {
		_ = connection.Close()
		return
	}
	go client.writePump()
	client.readPump(request.Context())
}

func trustedIdentity(request *http.Request) (uint64, int64, bool) {
	if request == nil {
		return 0, 0, false
	}
	userID, err := strconv.ParseUint(request.Header.Get("User-ID"), 10, 64)
	if err != nil || userID == 0 {
		return 0, 0, false
	}
	role, err := strconv.ParseInt(request.Header.Get("Role"), 10, 32)
	if err != nil || role != 1 && role != 2 {
		return 0, 0, false
	}
	return userID, role, true
}
