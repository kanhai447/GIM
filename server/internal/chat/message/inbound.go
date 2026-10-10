package message

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/delivery"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

type Sender interface {
	SendToClient(context.Context, uint64, string, []byte) (chat.DeliveryResult, error)
}

type Publisher interface {
	Publish(context.Context, delivery.Event) error
}

type Inbound struct {
	service   *Service
	sender    Sender
	publisher Publisher
	logger    *log.Logger
}

func NewInbound(service *Service, sender Sender, publisher Publisher, logger *log.Logger) (*Inbound, error) {
	if service == nil || sender == nil || publisher == nil {
		return nil, errors.New("invalid chat message inbound configuration")
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Inbound{service: service, sender: sender, publisher: publisher, logger: logger}, nil
}

func (inbound *Inbound) Handle(ctx context.Context, identity chat.ClientIdentity, envelope protocol.Envelope) error {
	if envelope.Event != protocol.EventChatSend {
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, CodeInvalidMessage, "不支持的 WebSocket 事件")
		return nil
	}
	data, err := protocol.DecodeData[protocol.ChatSendData](envelope.Data)
	if err != nil {
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, CodeInvalidMessage, "消息格式无效")
		return nil
	}
	result, err := inbound.service.SendPrivateMessage(ctx, identity.UserID, SendRequest{
		ClientMsgID: envelope.ClientMsgID, ReceiverID: data.ReceiverID, Message: data.Message,
	})
	if err != nil {
		code, message := safeError(err)
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, code, message)
		return nil
	}

	ackData, err := protocol.Data(protocol.AckData{
		MessageID: result.Message.ID, CreatedAt: result.Message.CreatedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		inbound.sendError(ctx, identity, envelope.RequestID, envelope.ClientMsgID, CodeMessageUnavailable, "消息服务暂不可用")
		return nil
	}
	ack, err := protocol.Encode(protocol.Envelope{
		Event: protocol.EventChatAck, RequestID: envelope.RequestID, ClientMsgID: envelope.ClientMsgID, Data: ackData,
	})
	if err != nil {
		return nil
	}
	_, ackErr := inbound.sender.SendToClient(ctx, identity.UserID, identity.ClientID, ack)

	if result.CreatedNew {
		messageData, marshalErr := protocol.Data(toProtocolMessage(result.Message))
		if marshalErr == nil {
			outbound, encodeErr := protocol.Encode(protocol.Envelope{
				Event: protocol.EventChatMessage, ClientMsgID: result.Message.ClientMsgID, Data: messageData,
			})
			if encodeErr == nil {
				if publishErr := inbound.publisher.Publish(ctx, delivery.Event{
					MessageID: result.Message.ID, ReceiverID: result.Message.ReceiverID, Payload: json.RawMessage(outbound),
				}); publishErr != nil {
					inbound.logger.Printf("chat realtime delivery degraded messageId=%d senderID=%d receiverID=%d", result.Message.ID, result.Message.SenderID, result.Message.ReceiverID)
				}
			}
		}
	}
	return ackErr
}

func (inbound *Inbound) HandleMalformed(ctx context.Context, identity chat.ClientIdentity) error {
	inbound.sendError(ctx, identity, "", "", CodeInvalidMessage, "WebSocket 消息不是有效 JSON envelope")
	return nil
}

func (inbound *Inbound) sendError(ctx context.Context, identity chat.ClientIdentity, requestID, clientMsgID string, code uint32, message string) {
	payload, err := protocol.Encode(protocol.Envelope{
		Event: protocol.EventError, RequestID: requestID, ClientMsgID: clientMsgID,
		Error: &protocol.Error{Code: code, Message: message},
	})
	if err != nil {
		return
	}
	_, _ = inbound.sender.SendToClient(ctx, identity.UserID, identity.ClientID, payload)
}

func safeError(err error) (uint32, string) {
	var publicError *PublicError
	if errors.As(err, &publicError) {
		return publicError.Code, publicError.Message
	}
	return CodeMessageUnavailable, "消息服务暂不可用"
}

func toProtocolMessage(value Message) protocol.PrivateMessage {
	return protocol.PrivateMessage{
		MessageID: value.ID, SenderID: value.SenderID, ReceiverID: value.ReceiverID,
		ClientMsgID: value.ClientMsgID, Message: value.Payload, CreatedAt: value.CreatedAt.Format(time.RFC3339Nano),
	}
}

func LocalDeliveryHandler(hub *chat.Hub, timeout time.Duration, logger *log.Logger) delivery.Handler {
	if logger == nil {
		logger = log.Default()
	}
	return func(ctx context.Context, event delivery.Event) {
		deliveryContext, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if _, err := hub.SendToUser(deliveryContext, event.ReceiverID, event.Payload); err != nil && !errors.Is(err, chat.ErrHubClosed) && !errors.Is(err, context.Canceled) {
			logger.Printf("chat local realtime delivery failed messageId=%d receiverID=%d", event.MessageID, event.ReceiverID)
		}
	}
}
