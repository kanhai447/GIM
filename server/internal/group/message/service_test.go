package message

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
)

type memoryGroupRepository struct {
	mu       sync.Mutex
	nextID   uint64
	rows     map[string]Message
	images   map[uint64]ImageFile
	imageErr error
}

func (repository *memoryGroupRepository) PersistWithSessions(_ context.Context, value Message) (PersistResult, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	key := value.ClientMsgID + string(rune(value.SenderID))
	if existing, exists := repository.rows[key]; exists {
		return PersistResult{Message: existing, TargetUserIDs: []uint64{value.SenderID}, CreatedNew: false}, nil
	}
	repository.nextID++
	value.ID = repository.nextID
	value.CreatedAt = time.Now().UTC()
	repository.rows[key] = value
	return PersistResult{Message: value, TargetUserIDs: []uint64{value.SenderID}, CreatedNew: true}, nil
}

func (repository *memoryGroupRepository) GetBySenderAndClientMsgID(_ context.Context, senderID uint64, clientMsgID string) (Message, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	value, exists := repository.rows[clientMsgID+string(rune(senderID))]
	if !exists {
		return Message{}, ErrNotFound
	}
	return value, nil
}

func (repository *memoryGroupRepository) GetByID(context.Context, uint64) (Message, error) {
	return Message{}, ErrNotFound
}

func (repository *memoryGroupRepository) ListSessions(context.Context, uint64, int) ([]Session, int64, error) {
	return nil, 0, nil
}

func (repository *memoryGroupRepository) ListHistory(context.Context, uint64, uint64, uint64, int) ([]Message, error) {
	return nil, nil
}

func (repository *memoryGroupRepository) MarkRead(context.Context, uint64, uint64, uint64) (Session, error) {
	return Session{}, nil
}

func (repository *memoryGroupRepository) GetOwnedImage(_ context.Context, _ uint64, fileID uint64) (ImageFile, error) {
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

func newImageReceiver(t *testing.T, images map[uint64]ImageFile, imageErr error, maxImageBytes int) (*Service, *memoryGroupRepository) {
	t.Helper()
	config := DefaultConfig()
	if maxImageBytes > 0 {
		config.MaxImageBytes = maxImageBytes
	}
	repository := &memoryGroupRepository{rows: make(map[string]Message), images: images, imageErr: imageErr}
	service, err := NewService(repository, inboundPolicy{}, config)
	if err != nil {
		t.Fatal(err)
	}
	return service, repository
}

func TestSendGroupImageMessageNormalizesServerMetadata(t *testing.T) {
	service, repository := newImageReceiver(t, map[uint64]ImageFile{
		7: {ID: 7, Size: 512, MIMEType: "image/png"},
		8: {ID: 8, Size: 64, MIMEType: "image/jpeg"},
	}, nil, 0)
	request := groupImageRequest("group-image-key", 9, 7)
	first, err := service.Send(context.Background(), 1, request)
	if err != nil || !first.CreatedNew {
		t.Fatalf("first image send = %#v, %v", first, err)
	}
	image := first.Message.Payload.ImageMsg
	if first.Message.Type != ImageMessageType || first.Message.Preview != "[图片]" ||
		image == nil || image.URL != "/api/file/7" || image.MIMEType != "image/png" ||
		image.Size != 512 || image.Width != 800 || image.Height != 600 {
		t.Fatalf("normalized image = %#v", first.Message)
	}
	second, err := service.Send(context.Background(), 1, request)
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID || len(repository.rows) != 1 {
		t.Fatalf("image retry = %#v, %v rows=%d", second, err, len(repository.rows))
	}
	_, err = service.Send(context.Background(), 1, groupImageRequest("group-image-key", 9, 8))
	var publicError *PublicError
	if !errors.As(err, &publicError) || publicError.Code != CodeIdempotencyConflict {
		t.Fatalf("conflicting image retry error = %v", err)
	}
}

func TestSendGroupImageMessageRejectsInvalidFiles(t *testing.T) {
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
			request:  SendRequest{ClientMsgID: "group-image-key", GroupID: 9, Message: protocol.MessagePayload{Type: ImageMessageType}},
			wantCode: CodeImagePayload,
		},
		{
			name: "zero file id", images: map[uint64]ImageFile{},
			request: groupImageRequest("group-image-key", 9, 0), wantCode: CodeImagePayload,
		},
		{
			name: "invalid dimensions", images: map[uint64]ImageFile{},
			request: SendRequest{
				ClientMsgID: "group-image-key", GroupID: 9,
				Message: protocol.MessagePayload{Type: ImageMessageType, ImageMsg: &protocol.ImageMessage{FileID: 7, Width: 30000, Height: 1}},
			},
			wantCode: CodeImagePayload,
		},
		{
			name: "file not owned", images: map[uint64]ImageFile{},
			request: groupImageRequest("group-image-key", 9, 7), wantCode: CodeImageFileInvalid,
		},
		{
			name: "not an image", images: map[uint64]ImageFile{7: {ID: 7, Size: 10, MIMEType: "application/pdf"}},
			request: groupImageRequest("group-image-key", 9, 7), wantCode: CodeImageMIMEType,
		},
		{
			name: "too large", images: map[uint64]ImageFile{7: {ID: 7, Size: 2048, MIMEType: "image/png"}},
			request: groupImageRequest("group-image-key", 9, 7), maxBytes: 1024, wantCode: CodeImageTooLarge,
		},
		{
			name: "lookup failure", images: map[uint64]ImageFile{7: {ID: 7, Size: 10, MIMEType: "image/png"}},
			imageErr: errors.New("database unavailable"), request: groupImageRequest("group-image-key", 9, 7), wantCode: CodeUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, repository := newImageReceiver(t, test.images, test.imageErr, test.maxBytes)
			_, err := service.Send(context.Background(), 1, test.request)
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

func groupImageRequest(clientMsgID string, groupID, fileID uint64) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, GroupID: groupID,
		Message: protocol.MessagePayload{Type: ImageMessageType, ImageMsg: &protocol.ImageMessage{FileID: fileID, Width: 800, Height: 600}},
	}
}
