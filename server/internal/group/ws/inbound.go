package ws

import (
	"context"

	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
)

type FoundationInbound struct{ hub *Hub }

func NewFoundationInbound(hub *Hub) *FoundationInbound {
	return &FoundationInbound{hub: hub}
}

func (handler *FoundationInbound) Handle(ctx context.Context, identity ClientIdentity, envelope protocol.Envelope) error {
	return handler.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, protocol.ErrorUnsupported, "group websocket event is not implemented")
}

func (handler *FoundationInbound) HandleMalformed(ctx context.Context, identity ClientIdentity) error {
	return handler.sendError(ctx, identity, "", "", protocol.ErrorMalformed, "group websocket payload is invalid")
}

func (handler *FoundationInbound) sendError(ctx context.Context, identity ClientIdentity, requestID, clientMsgID string, code uint32, message string) error {
	payload, err := protocol.EncodeError(requestID, clientMsgID, code, message)
	if err != nil {
		return err
	}
	_, err = handler.hub.SendToClient(ctx, identity, payload)
	return err
}
