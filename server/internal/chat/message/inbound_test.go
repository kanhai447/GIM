package message

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kanhai447/GIM/server/internal/chat"
	"github.com/kanhai447/GIM/server/internal/chat/delivery"
	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

type captureSender struct{ payloads [][]byte }

func (sender *captureSender) SendToClient(_ context.Context, _ uint64, _ string, payload []byte) (chat.DeliveryResult, error) {
	sender.payloads = append(sender.payloads, append([]byte(nil), payload...))
	return chat.DeliveryResult{Delivered: 1}, nil
}

type capturePublisher struct {
	events []delivery.Event
	err    error
}

func (publisher *capturePublisher) Publish(_ context.Context, event delivery.Event) error {
	publisher.events = append(publisher.events, event)
	return publisher.err
}

func TestInboundAckAfterPersistenceAndDuplicateDoesNotRepublish(t *testing.T) {
	repository := &memoryRepository{rows: make(map[string]Message)}
	service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	sender := &captureSender{}
	publisher := &capturePublisher{err: errors.New("redis unavailable")}
	inbound, err := NewInbound(service, sender, publisher, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := protocol.Data(protocol.ChatSendData{
		ReceiverID: 2,
		Message:    protocol.MessagePayload{Type: TextMessageType, TextMsg: &protocol.TextMessage{Content: "hello"}},
	})
	envelope := protocol.Envelope{Event: protocol.EventChatSend, RequestID: "request-1", ClientMsgID: "client-1", Data: data}
	identity := chat.ClientIdentity{UserID: 1, ClientID: "originating-client"}
	if err := inbound.Handle(context.Background(), identity, envelope); err != nil {
		t.Fatal(err)
	}
	if err := inbound.Handle(context.Background(), identity, envelope); err != nil {
		t.Fatal(err)
	}
	if len(sender.payloads) != 2 || len(publisher.events) != 1 {
		t.Fatalf("acks=%d delivery publishes=%d", len(sender.payloads), len(publisher.events))
	}
	first := decodeCapturedEnvelope(t, sender.payloads[0])
	second := decodeCapturedEnvelope(t, sender.payloads[1])
	firstAck, _ := protocol.DecodeData[protocol.AckData](first.Data)
	secondAck, _ := protocol.DecodeData[protocol.AckData](second.Data)
	if first.Event != protocol.EventChatAck || second.Event != protocol.EventChatAck || firstAck.MessageID == 0 || secondAck.MessageID != firstAck.MessageID {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}

func TestInboundMalformedAndBusinessErrorsAreTyped(t *testing.T) {
	repository := &memoryRepository{rows: make(map[string]Message)}
	service, _ := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
	sender := &captureSender{}
	inbound, _ := NewInbound(service, sender, &capturePublisher{}, nil)
	identity := chat.ClientIdentity{UserID: 1, ClientID: "client"}
	if err := inbound.HandleMalformed(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	if err := inbound.Handle(context.Background(), identity, protocol.Envelope{
		Event: protocol.EventChatSend, ClientMsgID: "bad key", Data: json.RawMessage(`{"revUserID":2,"msg":{"type":1,"textMsg":{"content":"hello"}}}`),
	}); err != nil {
		t.Fatal(err)
	}
	if len(sender.payloads) != 2 {
		t.Fatalf("typed errors = %d", len(sender.payloads))
	}
	for _, payload := range sender.payloads {
		envelope := decodeCapturedEnvelope(t, payload)
		if envelope.Event != protocol.EventError || envelope.Error == nil || envelope.Error.Code != CodeInvalidMessage {
			t.Fatalf("error envelope = %#v", envelope)
		}
	}
}

func decodeCapturedEnvelope(t *testing.T, payload []byte) protocol.Envelope {
	t.Helper()
	var envelope protocol.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}
