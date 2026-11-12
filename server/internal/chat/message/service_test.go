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
	mu       sync.Mutex
	nextID   uint64
	rows     map[string]Message
	images   map[uint64]ImageFile
	imageErr error
	err      error
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

func (repository *memoryRepository) PersistWithSessions(ctx context.Context, value Message) (PersistResult, error) {
	created, err := repository.Create(ctx, value)
	if errors.Is(err, ErrDuplicate) {
		existing, queryErr := repository.GetBySenderAndClientMsgID(ctx, value.SenderID, value.ClientMsgID)
		return PersistResult{Message: existing}, queryErr
	}
	if err != nil {
		return PersistResult{}, err
	}
	return PersistResult{Message: created, CreatedNew: true}, nil
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

func (repository *memoryRepository) GetOwnedImage(_ context.Context, _ uint64, fileID uint64) (ImageFile, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.imageErr != nil {
		return ImageFile{}, repository.imageErr
	}
	file, exists := repository.images[fileID]
	if !exists {
		return ImageFile{}, ErrImageFileNotFound
	}
	return file, nil
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

func TestSendPrivateImageMessageNormalizesServerMetadata(t *testing.T) {
	repository := &memoryRepository{
		rows:   make(map[string]Message),
		images: map[uint64]ImageFile{7: {ID: 7, Size: 512, MIMEType: "image/png"}},
	}
	service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	request := imageRequest("image-key", 2, 7, 800, 600)
	first, err := service.SendPrivateMessage(context.Background(), 1, request)
	if err != nil || !first.CreatedNew {
		t.Fatalf("first image send = %#v, %v", first, err)
	}
	image := first.Message.Payload.ImageMsg
	if first.Message.Type != ImageMessageType || first.Message.Preview != "[图片]" ||
		image == nil || image.URL != "/api/file/7" || image.MIMEType != "image/png" ||
		image.Size != 512 || image.Width != 800 || image.Height != 600 {
		t.Fatalf("normalized image = %#v", first.Message)
	}
	second, err := service.SendPrivateMessage(context.Background(), 1, request)
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID {
		t.Fatalf("image retry = %#v, %v", second, err)
	}
	repository.images[8] = ImageFile{ID: 8, Size: 64, MIMEType: "image/jpeg"}
	_, err = service.SendPrivateMessage(context.Background(), 1, imageRequest("image-key", 2, 8, 1, 1))
	var publicError *PublicError
	if !errors.As(err, &publicError) || publicError.Code != CodeIdempotencyConflict {
		t.Fatalf("conflicting image retry error = %v", err)
	}
}

func TestSendPrivateImageMessageRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name     string
		images   map[uint64]ImageFile
		imageErr error
		request  SendRequest
		maxBytes int
		wantCode uint32
	}{
		{
			name: "missing image payload", images: map[uint64]ImageFile{},
			request:  SendRequest{ClientMsgID: "image-key", ReceiverID: 2, Message: protocol.MessagePayload{Type: ImageMessageType}},
			wantCode: CodeImagePayload,
		},
		{
			name: "zero file id", images: map[uint64]ImageFile{},
			request: imageRequest("image-key", 2, 0, 1, 1), wantCode: CodeImagePayload,
		},
		{
			name: "invalid dimensions", images: map[uint64]ImageFile{},
			request: imageRequest("image-key", 2, 7, 30000, 1), wantCode: CodeImagePayload,
		},
		{
			name: "file not owned", images: map[uint64]ImageFile{},
			request: imageRequest("image-key", 2, 7, 1, 1), wantCode: CodeImageFileInvalid,
		},
		{
			name: "not an image", images: map[uint64]ImageFile{7: {ID: 7, Size: 10, MIMEType: "text/plain"}},
			request: imageRequest("image-key", 2, 7, 1, 1), wantCode: CodeImageMIMEType,
		},
		{
			name: "too large", images: map[uint64]ImageFile{7: {ID: 7, Size: 2048, MIMEType: "image/png"}},
			request: imageRequest("image-key", 2, 7, 1, 1), maxBytes: 1024, wantCode: CodeImageTooLarge,
		},
		{
			name: "lookup failure", images: map[uint64]ImageFile{7: {ID: 7, Size: 10, MIMEType: "image/png"}},
			imageErr: errors.New("database unavailable"), request: imageRequest("image-key", 2, 7, 1, 1), wantCode: CodeMessageUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			if test.maxBytes > 0 {
				config.MaxImageBytes = test.maxBytes
			}
			repository := &memoryRepository{rows: make(map[string]Message), images: test.images, imageErr: test.imageErr}
			service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, config)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.SendPrivateMessage(context.Background(), 1, test.request)
			var publicError *PublicError
			if !errors.As(err, &publicError) || publicError.Code != test.wantCode {
				t.Fatalf("error = %v, want code %d", err, test.wantCode)
			}
			if len(repository.rows) != 0 {
				t.Fatalf("invalid image message was persisted: %#v", repository.rows)
			}
		})
	}
}

func imageRequest(clientMsgID string, receiverID, fileID uint64, width, height int) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: ImageMessageType, ImageMsg: &protocol.ImageMessage{FileID: fileID, Width: width, Height: height}},
	}
}

func valueKey(senderID uint64, clientMsgID string) string {
	return clientMsgID + string(rune(senderID))
}
