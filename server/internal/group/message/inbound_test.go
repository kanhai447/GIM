package message

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/delivery"
	"github.com/kanhai447/GIM/server/internal/group/domain"
	"github.com/kanhai447/GIM/server/internal/group/ws"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
	"github.com/kanhai447/GIM/server/internal/platform/audit"
)

type inboundRepository struct{}

func (inboundRepository) PersistWithSessions(_ context.Context, value Message) (PersistResult, error) {
	value.ID, value.GroupMemberID, value.CreatedAt = 31, 12, time.Now().UTC()
	return PersistResult{Message: value, TargetUserIDs: []uint64{7, 8}, CreatedNew: true}, nil
}
func (inboundRepository) GetBySenderAndClientMsgID(context.Context, uint64, string) (Message, error) {
	return Message{}, ErrNotFound
}
func (inboundRepository) GetByID(context.Context, uint64) (Message, error) {
	return Message{}, ErrNotFound
}
func (inboundRepository) ListSessions(context.Context, uint64, int) ([]Session, int64, error) {
	return nil, 0, nil
}
func (inboundRepository) ListHistory(context.Context, uint64, uint64, uint64, int) ([]Message, error) {
	return nil, nil
}
func (inboundRepository) MarkRead(context.Context, uint64, uint64, uint64) (Session, error) {
	return Session{}, nil
}

type inboundPolicy struct{}

func (inboundPolicy) GetGroupByID(context.Context, uint64) (domain.Group, error) {
	return domain.Group{ID: 9, Status: domain.GroupStatusActive}, nil
}
func (inboundPolicy) IsMember(context.Context, uint64, uint64) (bool, error) { return true, nil }
func (inboundPolicy) IsMuted(context.Context, uint64, uint64) (bool, error)  { return false, nil }

type inboundSender struct{ payloads [][]byte }

func (sender *inboundSender) SendToClient(_ context.Context, _ ws.ClientIdentity, payload []byte) (ws.DeliveryResult, error) {
	sender.payloads = append(sender.payloads, payload)
	return ws.DeliveryResult{Delivered: 1}, nil
}

type inboundDeliveryPublisher struct{ events []delivery.Event }

func (publisher *inboundDeliveryPublisher) Publish(_ context.Context, event delivery.Event) error {
	publisher.events = append(publisher.events, event)
	return nil
}

type inboundAuditPublisher struct {
	events []audit.Event
	err    error
}

func (publisher *inboundAuditPublisher) Publish(_ context.Context, event audit.Event) error {
	publisher.events = append(publisher.events, event)
	return publisher.err
}

func TestGroupInboundAuditExcludesMessageContent(t *testing.T) {
	service, err := NewService(inboundRepository{}, inboundPolicy{}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	sender, deliveryPublisher, auditor := &inboundSender{}, &inboundDeliveryPublisher{}, &inboundAuditPublisher{err: errors.New("Kafka unavailable")}
	inbound, err := NewInbound(service, sender, deliveryPublisher, nil, auditor)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := protocol.Data(protocol.GroupSendData{GroupID: 9, Message: protocol.MessagePayload{Type: TextMessageType, TextMsg: &protocol.TextMessage{Content: "do-not-log-group-content"}}})
	err = inbound.Handle(context.Background(), ws.ClientIdentity{UserID: 7, ClientID: "client"}, protocol.Envelope{Event: protocol.EventGroupSend, ClientMsgID: "group-client-1", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.payloads) != 1 || len(deliveryPublisher.events) != 1 || len(auditor.events) != 1 {
		t.Fatalf("ack/delivery/audit = %d/%d/%d", len(sender.payloads), len(deliveryPublisher.events), len(auditor.events))
	}
	payload, _ := json.Marshal(auditor.events[0])
	if auditor.events[0].EventType != audit.EventGroupMessageAccepted || strings.Contains(string(payload), "do-not-log-group-content") {
		t.Fatalf("unsafe audit = %s", payload)
	}
}
