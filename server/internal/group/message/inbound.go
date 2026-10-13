package message

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/delivery"
	"github.com/kanhai447/GIM/server/internal/group/ws"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
)

type Sender interface {
	SendToClient(context.Context, ws.ClientIdentity, []byte) (ws.DeliveryResult, error)
}
type Publisher interface {
	Publish(context.Context, delivery.Event) error
}
type Inbound struct {
	service   *Service
	sender    Sender
	publisher Publisher
	logger    *log.Logger
	auditor   audit.Publisher
}

func NewInbound(service *Service, sender Sender, publisher Publisher, logger *log.Logger, auditors ...audit.Publisher) (*Inbound, error) {
	if service == nil || sender == nil || publisher == nil {
		return nil, errors.New("invalid group message inbound configuration")
	}
	if logger == nil {
		logger = log.Default()
	}
	var auditor audit.Publisher
	if len(auditors) > 0 {
		auditor = auditors[0]
	}
	return &Inbound{service: service, sender: sender, publisher: publisher, logger: logger, auditor: auditor}, nil
}
func (inbound *Inbound) Handle(ctx context.Context, identity ws.ClientIdentity, envelope protocol.Envelope) error {
	if envelope.Event != protocol.EventGroupSend {
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, protocol.ErrorUnsupported, "不支持的群 WebSocket 事件")
		return nil
	}
	data, err := protocol.DecodeData[protocol.GroupSendData](envelope.Data)
	if err != nil {
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, CodeInvalidMessage, "群消息格式无效")
		return nil
	}
	result, err := inbound.service.Send(ctx, identity.UserID, SendRequest{ClientMsgID: envelope.ClientMsgID, GroupID: data.GroupID, Message: data.Message})
	if err != nil {
		code, text := safeError(err)
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, code, text)
		return nil
	}
	ackData, _ := protocol.Data(protocol.AckData{MessageID: result.Message.ID, GroupID: result.Message.GroupID, CreatedAt: result.Message.CreatedAt.Format(time.RFC3339Nano)})
	ack, err := protocol.Encode(protocol.Envelope{Event: protocol.EventGroupAck, RequestID: envelope.RequestID, ClientMsgID: envelope.ClientMsgID, Data: ackData})
	if err != nil {
		return nil
	}
	_, ackErr := inbound.sender.SendToClient(ctx, identity, ack)
	if result.CreatedNew {
		messageData, marshalErr := protocol.Data(toProtocolMessage(result.Message))
		if marshalErr == nil {
			outbound, encodeErr := protocol.Encode(protocol.Envelope{Event: protocol.EventGroupMessage, ClientMsgID: result.Message.ClientMsgID, Data: messageData})
			if encodeErr == nil {
				if publishErr := inbound.publisher.Publish(ctx, delivery.Event{MessageID: result.Message.ID, TargetUserIDs: result.TargetUserIDs, Payload: json.RawMessage(outbound)}); publishErr != nil {
					inbound.logger.Printf("group realtime delivery degraded messageId=%d groupID=%d senderID=%d", result.Message.ID, result.Message.GroupID, result.Message.SenderID)
				}
			}
		}
		metadata, metadataErr := audit.Metadata(map[string]any{
			"message_id": result.Message.ID, "sender_id": result.Message.SenderID, "group_id": result.Message.GroupID,
			"message_type": result.Message.Type, "client_msg_id": result.Message.ClientMsgID,
		})
		if metadataErr == nil {
			event := audit.NewEvent(audit.EventGroupMessageAccepted, "group", "message.accept", audit.StatusSuccess)
			event.UserID, event.ResourceType, event.ResourceID, event.RequestID, event.Metadata = audit.Uint64(result.Message.SenderID), "group_message", strconv.FormatUint(result.Message.ID, 10), audit.NewRequestID(), metadata
			audit.EmitBestEffort(ctx, inbound.auditor, event, inbound.logger)
		}
	}
	return ackErr
}
func (inbound *Inbound) HandleMalformed(ctx context.Context, identity ws.ClientIdentity) error {
	inbound.sendError(ctx, identity, "", "", protocol.ErrorMalformed, "group websocket payload is invalid")
	return nil
}
func (inbound *Inbound) sendError(ctx context.Context, identity ws.ClientIdentity, requestID, clientMsgID string, code uint32, text string) {
	payload, err := protocol.EncodeError(requestID, clientMsgID, code, text)
	if err == nil {
		_, _ = inbound.sender.SendToClient(ctx, identity, payload)
	}
}
func safeError(err error) (uint32, string) {
	var publicError *PublicError
	if errors.As(err, &publicError) {
		return publicError.Code, publicError.Message
	}
	return CodeUnavailable, "群消息服务暂不可用"
}
func toProtocolMessage(value Message) protocol.GroupMessage {
	return protocol.GroupMessage{MessageID: value.ID, GroupID: value.GroupID, SenderID: value.SenderID, ClientMsgID: value.ClientMsgID, Message: value.Payload, CreatedAt: value.CreatedAt.Format(time.RFC3339Nano)}
}
func LocalDeliveryHandler(hub *ws.Hub, timeout time.Duration, logger *log.Logger) delivery.Handler {
	if logger == nil {
		logger = log.Default()
	}
	return func(ctx context.Context, event delivery.Event) {
		for _, userID := range event.TargetUserIDs {
			deliveryCtx, cancel := context.WithTimeout(ctx, timeout)
			_, err := hub.SendToUser(deliveryCtx, userID, event.Payload)
			cancel()
			if err != nil && !errors.Is(err, ws.ErrHubClosed) && !errors.Is(err, context.Canceled) {
				logger.Printf("group local realtime delivery failed messageId=%d userID=%d", event.MessageID, userID)
			}
		}
	}
}
