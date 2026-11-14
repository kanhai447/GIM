package message

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/chat/protocol"
)

type memoryRepository struct {
	mu         sync.Mutex
	nextID     uint64
	rows       map[string]Message
	images     map[uint64]ImageFile
	imageErr   error
	files      map[uint64]FileInfo
	fileErr    error
	tombstoned map[uint64]bool
	err        error
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

func (repository *memoryRepository) GetOwnedFile(_ context.Context, _ uint64, fileID uint64) (FileInfo, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.fileErr != nil {
		return FileInfo{}, repository.fileErr
	}
	file, exists := repository.files[fileID]
	if !exists {
		return FileInfo{}, ErrFileNotFound
	}
	return file, nil
}

func (repository *memoryRepository) GetByIDForRecall(_ context.Context, messageID uint64) (Message, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.ID == messageID {
			return value, repository.tombstoned[messageID], nil
		}
	}
	return Message{}, false, ErrNotFound
}

func (repository *memoryRepository) IsRecalled(_ context.Context, targetID uint64) (bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.Type == WithdrawMessageType && value.Payload.WithdrawMsg != nil && value.Payload.WithdrawMsg.MsgID == targetID {
			return true, nil
		}
	}
	return false, nil
}

func (repository *memoryRepository) GetWithdrawBySenderAndTarget(_ context.Context, senderID, targetID uint64) (Message, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.SenderID == senderID && value.Type == WithdrawMessageType && value.Payload.WithdrawMsg != nil && value.Payload.WithdrawMsg.MsgID == targetID {
			return value, true, nil
		}
	}
	return Message{}, false, nil
}

func (repository *memoryRepository) RecalledTargets(_ context.Context, ids []uint64) (map[uint64]bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	targets := make(map[uint64]bool, len(ids))
	wanted := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	for _, value := range repository.rows {
		if value.Type == WithdrawMessageType && value.Payload.WithdrawMsg != nil && wanted[value.Payload.WithdrawMsg.MsgID] {
			targets[value.Payload.WithdrawMsg.MsgID] = true
		}
	}
	return targets, nil
}

func (repository *memoryRepository) ListHistory(_ context.Context, userID, peerID, beforeID uint64, limit int) ([]Message, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	items := make([]Message, 0)
	for _, value := range repository.rows {
		if value.SenderID != userID && value.ReceiverID != userID {
			continue
		}
		if value.SenderID != peerID && value.ReceiverID != peerID {
			continue
		}
		if beforeID != 0 && value.ID >= beforeID {
			continue
		}
		items = append(items, value)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
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

func TestSendPrivateFileMessageNormalizesServerMetadata(t *testing.T) {
	repository := &memoryRepository{
		rows:  make(map[string]Message),
		files: map[uint64]FileInfo{7: {ID: 7, Name: "report.pdf", Size: 4096, MIMEType: "application/pdf"}},
	}
	service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	request := fileRequest("file-key", 2, 7)
	first, err := service.SendPrivateMessage(context.Background(), 1, request)
	if err != nil || !first.CreatedNew {
		t.Fatalf("first file send = %#v, %v", first, err)
	}
	file := first.Message.Payload.FileMsg
	if first.Message.Type != FileMessageType || first.Message.Preview != "[文件]" ||
		file == nil || file.URL != "/api/file/7" || file.FileName != "report.pdf" ||
		file.Size != 4096 || file.MIMEType != "application/pdf" {
		t.Fatalf("normalized file = %#v", first.Message)
	}
	second, err := service.SendPrivateMessage(context.Background(), 1, request)
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID {
		t.Fatalf("file retry = %#v, %v", second, err)
	}
	repository.files[8] = FileInfo{ID: 8, Name: "other.txt", Size: 16, MIMEType: "text/plain"}
	_, err = service.SendPrivateMessage(context.Background(), 1, fileRequest("file-key", 2, 8))
	var publicError *PublicError
	if !errors.As(err, &publicError) || publicError.Code != CodeIdempotencyConflict {
		t.Fatalf("conflicting file retry error = %v", err)
	}
}

func TestSendPrivateFileMessageRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    map[uint64]FileInfo
		fileErr  error
		request  SendRequest
		maxBytes int
		wantCode uint32
	}{
		{
			name: "missing file payload", files: map[uint64]FileInfo{},
			request:  SendRequest{ClientMsgID: "file-key", ReceiverID: 2, Message: protocol.MessagePayload{Type: FileMessageType}},
			wantCode: CodeFilePayload,
		},
		{
			name: "zero file id", files: map[uint64]FileInfo{},
			request: fileRequest("file-key", 2, 0), wantCode: CodeFilePayload,
		},
		{
			name: "negative size", files: map[uint64]FileInfo{},
			request: SendRequest{
				ClientMsgID: "file-key", ReceiverID: 2,
				Message: protocol.MessagePayload{Type: FileMessageType, FileMsg: &protocol.FileMessage{FileID: 7, Size: -1}},
			},
			wantCode: CodeFilePayload,
		},
		{
			name: "file not owned", files: map[uint64]FileInfo{},
			request: fileRequest("file-key", 2, 7), wantCode: CodeFileInvalid,
		},
		{
			name: "too large", files: map[uint64]FileInfo{7: {ID: 7, Name: "big.bin", Size: 2048, MIMEType: "application/octet-stream"}},
			request: fileRequest("file-key", 2, 7), maxBytes: 1024, wantCode: CodeFileTooLarge,
		},
		{
			name: "lookup failure", files: map[uint64]FileInfo{7: {ID: 7, Name: "a.txt", Size: 10, MIMEType: "text/plain"}},
			fileErr: errors.New("database unavailable"), request: fileRequest("file-key", 2, 7), wantCode: CodeMessageUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			if test.maxBytes > 0 {
				config.MaxFileBytes = test.maxBytes
			}
			repository := &memoryRepository{rows: make(map[string]Message), files: test.files, fileErr: test.fileErr}
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
				t.Fatalf("invalid file message was persisted: %#v", repository.rows)
			}
		})
	}
}

func TestRecallMessageReplacesHistoryAndIsIdempotent(t *testing.T) {
	repository := &memoryRepository{rows: make(map[string]Message)}
	service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	target, err := service.SendPrivateMessage(context.Background(), 1, textRequest("recall-target", 2, "delete me"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-1", 2, target.Message.ID))
	if err != nil || !first.CreatedNew {
		t.Fatalf("recall = %#v, %v", first, err)
	}
	if first.Message.Type != WithdrawMessageType || first.Message.Preview != "[撤回]" ||
		first.Message.Payload.WithdrawMsg == nil || first.Message.Payload.WithdrawMsg.MsgID != target.Message.ID {
		t.Fatalf("recall payload = %#v", first.Message)
	}
	second, err := service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-1", 2, target.Message.ID))
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID {
		t.Fatalf("recall retry = %#v, %v", second, err)
	}
	third, err := service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-2", 2, target.Message.ID))
	if err != nil || third.CreatedNew || third.Message.ID != first.Message.ID {
		t.Fatalf("recall with another clientMsgId = %#v, %v", third, err)
	}
	page, err := service.History(context.Background(), 2, 1, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	foundTarget, foundWithdraw := false, false
	for _, item := range page.Items {
		if item.ID == target.Message.ID {
			foundTarget = true
			if item.Type != WithdrawMessageType || item.Preview != "[撤回]" || item.Payload.TextMsg != nil ||
				item.Payload.WithdrawMsg == nil || item.Payload.WithdrawMsg.MsgID != target.Message.ID {
				t.Fatalf("history target not replaced: %#v", item)
			}
		}
		if item.ID == first.Message.ID {
			foundWithdraw = true
			if item.Type != WithdrawMessageType {
				t.Fatalf("history withdraw = %#v", item)
			}
		}
	}
	if !foundTarget || !foundWithdraw {
		t.Fatalf("history missing target/withdraw: target=%v withdraw=%v", foundTarget, foundWithdraw)
	}
	if len(repository.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(repository.rows))
	}
}

func TestRecallMessageValidation(t *testing.T) {
	tests := []struct {
		name         string
		targetSender uint64
		targetPeer   uint64
		wantCode     uint32
	}{
		{name: "target missing", wantCode: CodeWithdrawTarget},
		{name: "not owner", targetSender: 2, targetPeer: 1, wantCode: CodeWithdrawNotOwner},
		{name: "not participant", targetSender: 2, targetPeer: 3, wantCode: CodeWithdrawTarget},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &memoryRepository{rows: make(map[string]Message), tombstoned: map[uint64]bool{}}
			service, err := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			targetID := uint64(999_999)
			if test.targetSender != 0 {
				target, createErr := repository.Create(context.Background(), Message{
					SenderID: test.targetSender, ReceiverID: test.targetPeer, ClientMsgID: "seed-target", Type: TextMessageType,
					Preview: "seed", Payload: protocol.MessagePayload{Type: TextMessageType, TextMsg: &protocol.TextMessage{Content: "seed"}},
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				targetID = target.ID
			}
			_, err = service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-key", 2, targetID))
			var publicError *PublicError
			if !errors.As(err, &publicError) || publicError.Code != test.wantCode {
				t.Fatalf("error = %v, want code %d", err, test.wantCode)
			}
			if len(repository.rows) != 1 && test.targetSender != 0 {
				t.Fatalf("unexpected rows: %d", len(repository.rows))
			}
		})
	}

	t.Run("invalid payload", func(t *testing.T) {
		repository := &memoryRepository{rows: make(map[string]Message)}
		service, _ := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
		_, err := service.SendPrivateMessage(context.Background(), 1, SendRequest{
			ClientMsgID: "withdraw-empty", ReceiverID: 2,
			Message: protocol.MessagePayload{Type: WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{}},
		})
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawPayload {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("tombstoned target", func(t *testing.T) {
		repository := &memoryRepository{rows: make(map[string]Message), tombstoned: map[uint64]bool{}}
		service, _ := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
		target, _ := service.SendPrivateMessage(context.Background(), 1, textRequest("tomb-target", 2, "gone"))
		repository.tombstoned[target.Message.ID] = true
		_, err := service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-tomb", 2, target.Message.ID))
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawRecalled {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("already recalled by another sender", func(t *testing.T) {
		repository := &memoryRepository{rows: make(map[string]Message), tombstoned: map[uint64]bool{}}
		service, _ := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
		target, _ := service.SendPrivateMessage(context.Background(), 1, textRequest("other-target", 2, "mine"))
		if _, err := repository.Create(context.Background(), Message{
			SenderID: 2, ReceiverID: 1, ClientMsgID: "other-withdraw", Type: WithdrawMessageType,
			Preview: "[撤回]", Payload: protocol.MessagePayload{Type: WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: target.Message.ID}},
		}); err != nil {
			t.Fatal(err)
		}
		_, err := service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-late", 2, target.Message.ID))
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawRecalled {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("outside window", func(t *testing.T) {
		repository := &memoryRepository{rows: make(map[string]Message)}
		service, _ := NewService(repository, recipientStub{}, friendshipStub{ok: true}, DefaultConfig())
		target, _ := service.SendPrivateMessage(context.Background(), 1, textRequest("window-target", 2, "too old"))
		for key, value := range repository.rows {
			if value.ID == target.Message.ID {
				value.CreatedAt = time.Now().Add(-time.Hour)
				repository.rows[key] = value
			}
		}
		_, err := service.SendPrivateMessage(context.Background(), 1, withdrawRequest("withdraw-window", 2, target.Message.ID))
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawWindow {
			t.Fatalf("error = %v", err)
		}
	})
}

func withdrawRequest(clientMsgID string, receiverID, targetID uint64) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: targetID}},
	}
}

func fileRequest(clientMsgID string, receiverID, fileID uint64) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, ReceiverID: receiverID,
		Message: protocol.MessagePayload{Type: FileMessageType, FileMsg: &protocol.FileMessage{FileID: fileID}},
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
