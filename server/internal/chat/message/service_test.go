package message

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

type memoryRepository struct {
	mu     sync.Mutex
	nextID uint64
	rows   map[string]Message
	err    error
}

func (repository *memoryRepository) Create(_ context.Context, value Message) (Message, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.err != nil {
		return Message{}, repository.err
	}
	key := valueKey(value.SenderID, value.ClientMsgID)
	if _, exists := repository.rows[key]; exists {
		return Message{}, ErrDuplicate
	}
	repository.nextID++
	value.ID = repository.nextID
	value.CreatedAt = time.Now()
	repository.rows[key] = value
	return value, nil
}

func (repository *memoryRepository) GetBySenderAndClientMsgID(_ context.Context, senderID uint64, clientMsgID string) (Message, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.err != nil {
		return Message{}, repository.err
	}
	value, exists := repository.rows[valueKey(senderID, clientMsgID)]
	if !exists {
		return Message{}, ErrNotFound
	}
	return value, nil
}

func (repository *memoryRepository) GetByID(_ context.Context, messageID uint64) (Message, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.ID == messageID {
			return value, nil
		}
	}
	return Message{}, ErrNotFound
}

type recipientStub struct{ err error }

func (validator recipientStub) GetByID(_ context.Context, id uint64) (Recipient, error) {
	return Recipient{ID: id, Active: true}, validator.err
}

type friendshipStub struct {
	ok         bool
	err        error
	restricted bool
}

func (repository friendshipStub) AreFriends(context.Context, uint64, uint64) (bool, error) {
	return repository.ok, repository.err
}

func (repository friendshipStub) CanChat(context.Context, uint64) (bool, error) {
	return !repository.restricted, repository.err
}

func TestSendPrivateMessageIsIdempotentAndDetectsConflict(t *testing.T) {
	repository := &memoryRepository{rows: make(map[string]Message)}
	service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	request := textRequest("retry-key", 2, "hello")
	first, err := service.SendPrivateMessage(context.Background(), 1, request)
	if err != nil || !first.CreatedNew || first.Message.ID == 0 {
		t.Fatalf("first send = %#v, %v", first, err)
	}
	second, err := service.SendPrivateMessage(context.Background(), 1, request)
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID {
		t.Fatalf("retry send = %#v, %v", second, err)
	}
	_, err = service.SendPrivateMessage(context.Background(), 1, textRequest("retry-key", 2, "changed"))
	var publicError *PublicError
	if !errors.As(err, &publicError) || publicError.Code != CodeIdempotencyConflict {
		t.Fatalf("conflicting retry error = %v", err)
	}
}

func TestSendPrivateMessageValidationAndSafeErrors(t *testing.T) {
	tests := []struct {
		name       string
		senderID   uint64
		request    SendRequest
		recipients RecipientValidator
		friends    FriendshipRepository
		wantCode   uint32
	}{
		{name: "self", senderID: 1, request: textRequest("valid-key", 1, "hello"), recipients: recipientStub{}, friends: friendshipStub{ok: true}, wantCode: CodeInvalidMessage},
		{name: "client id", senderID: 1, request: textRequest("bad key", 2, "hello"), recipients: recipientStub{}, friends: friendshipStub{ok: true}, wantCode: CodeInvalidMessage},
		{name: "blank", senderID: 1, request: textRequest("valid-key", 2, "  "), recipients: recipientStub{}, friends: friendshipStub{ok: true}, wantCode: CodeInvalidMessage},
		{name: "missing recipient", senderID: 1, request: textRequest("valid-key", 2, "hello"), recipients: recipientStub{err: ErrRecipientNotFound}, friends: friendshipStub{ok: true}, wantCode: CodeRecipientMissing},
		{name: "not friends", senderID: 1, request: textRequest("valid-key", 2, "hello"), recipients: recipientStub{}, friends: friendshipStub{}, wantCode: CodeFriendshipRequired},
		{name: "restricted", senderID: 1, request: textRequest("valid-key", 2, "hello"), recipients: recipientStub{}, friends: friendshipStub{ok: true, restricted: true}, wantCode: CodeChatRestricted},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(&memoryRepository{rows: make(map[string]Message)}, test.recipients, test.friends, DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.SendPrivateMessage(context.Background(), test.senderID, test.request)
			var publicError *PublicError
			if !errors.As(err, &publicError) || publicError.Code != test.wantCode {
				t.Fatalf("error = %v, want code %d", err, test.wantCode)
			}
		})
	}
}

func textRequest(clientMsgID string, receiverID uint64, content string) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: TextMessageType, TextMsg: &protocol.TextMessage{Content: content}},
	}
}

func valueKey(senderID uint64, clientMsgID string) string {
	return clientMsgID + string(rune(senderID))
}
