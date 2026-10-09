package chat

import (
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/rest"
)

const maxInboundFrameBytes = 1 << 20

type Handler struct {
	hub        *Hub
	inbound    InboundHandler
	sendBuffer int
	upgrader   websocket.Upgrader
}

func NewHandler(hub *Hub, inbound InboundHandler, sendBuffer int, origins *OriginPolicy) *Handler {
	return &Handler{
		hub: hub, inbound: inbound, sendBuffer: sendBuffer,
		upgrader: websocket.Upgrader{CheckOrigin: origins.Allows},
	}
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
	connection.SetReadLimit(maxInboundFrameBytes)
	clientID, err := newClientID()
	if err != nil {
		_ = connection.Close()
		return
	}
	client := newClient(userID, clientID, connection, handler.hub, handler.inbound, handler.sendBuffer)
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
