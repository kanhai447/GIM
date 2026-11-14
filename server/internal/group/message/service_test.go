package message

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/kanhai447/GIM/server/internal/group/domain"
	"github.com/kanhai447/GIM/server/internal/group/ws/protocol"
)

type memoryGroupRepository struct {
	mu         sync.Mutex
	nextID     uint64
	rows       map[string]Message
	images     map[uint64]ImageFile
	imageErr   error
	files      map[uint64]FileInfo
	fileErr    error
	tombstoned map[uint64]bool
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

func (repository *memoryGroupRepository) GetOwnedFile(_ context.Context, _ uint64, fileID uint64) (FileInfo, error) {
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

func (repository *memoryGroupRepository) GetByIDForRecall(_ context.Context, messageID uint64) (Message, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.ID == messageID {
			return value, repository.tombstoned[messageID], nil
		}
	}
	return Message{}, false, ErrNotFound
}

func (repository *memoryGroupRepository) IsRecalled(_ context.Context, targetID uint64) (bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.Type == WithdrawMessageType && value.Payload.WithdrawMsg != nil && value.Payload.WithdrawMsg.MsgID == targetID {
			return true, nil
		}
	}
	return false, nil
}

func (repository *memoryGroupRepository) GetWithdrawBySenderAndTarget(_ context.Context, senderID, targetID uint64) (Message, bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, value := range repository.rows {
		if value.SenderID == senderID && value.Type == WithdrawMessageType && value.Payload.WithdrawMsg != nil && value.Payload.WithdrawMsg.MsgID == targetID {
			return value, true, nil
		}
	}
	return Message{}, false, nil
}

func (repository *memoryGroupRepository) RecalledTargets(_ context.Context, ids []uint64) (map[uint64]bool, error) {
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

func (repository *memoryGroupRepository) ListHistory(_ context.Context, _ uint64, groupID, beforeID uint64, limit int) ([]Message, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	items := make([]Message, 0)
	for _, value := range repository.rows {
		if value.GroupID != groupID || (beforeID != 0 && value.ID >= beforeID) {
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

func newFileReceiver(t *testing.T, files map[uint64]FileInfo, fileErr error, maxFileBytes int) (*Service, *memoryGroupRepository) {
	t.Helper()
	config := DefaultConfig()
	if maxFileBytes > 0 {
		config.MaxFileBytes = maxFileBytes
	}
	repository := &memoryGroupRepository{rows: make(map[string]Message), files: files, fileErr: fileErr}
	service, err := NewService(repository, inboundPolicy{}, config)
	if err != nil {
		t.Fatal(err)
	}
	return service, repository
}

func TestSendGroupFileMessageNormalizesServerMetadata(t *testing.T) {
	service, repository := newFileReceiver(t, map[uint64]FileInfo{
		7: {ID: 7, Name: "report.pdf", Size: 4096, MIMEType: "application/pdf"},
		8: {ID: 8, Name: "other.txt", Size: 16, MIMEType: "text/plain"},
	}, nil, 0)
	request := groupFileRequest("group-file-key", 9, 7)
	first, err := service.Send(context.Background(), 1, request)
	if err != nil || !first.CreatedNew {
		t.Fatalf("first file send = %#v, %v", first, err)
	}
	file := first.Message.Payload.FileMsg
	if first.Message.Type != FileMessageType || first.Message.Preview != "[文件]" ||
		file == nil || file.URL != "/api/file/7" || file.FileName != "report.pdf" ||
		file.Size != 4096 || file.MIMEType != "application/pdf" {
		t.Fatalf("normalized file = %#v", first.Message)
	}
	second, err := service.Send(context.Background(), 1, request)
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID || len(repository.rows) != 1 {
		t.Fatalf("file retry = %#v, %v rows=%d", second, err, len(repository.rows))
	}
	_, err = service.Send(context.Background(), 1, groupFileRequest("group-file-key", 9, 8))
	var publicError *PublicError
	if !errors.As(err, &publicError) || publicError.Code != CodeIdempotencyConflict {
		t.Fatalf("conflicting file retry error = %v", err)
	}
}

func TestSendGroupFileMessageRejectsInvalidFiles(t *testing.T) {
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
			request:  SendRequest{ClientMsgID: "group-file-key", GroupID: 9, Message: protocol.MessagePayload{Type: FileMessageType}},
			wantCode: CodeFilePayload,
		},
		{
			name: "zero file id", files: map[uint64]FileInfo{},
			request: groupFileRequest("group-file-key", 9, 0), wantCode: CodeFilePayload,
		},
		{
			name: "negative size", files: map[uint64]FileInfo{},
			request: SendRequest{
				ClientMsgID: "group-file-key", GroupID: 9,
				Message: protocol.MessagePayload{Type: FileMessageType, FileMsg: &protocol.FileMessage{FileID: 7, Size: -1}},
			},
			wantCode: CodeFilePayload,
		},
		{
			name: "file not owned", files: map[uint64]FileInfo{},
			request: groupFileRequest("group-file-key", 9, 7), wantCode: CodeFileInvalid,
		},
		{
			name: "too large", files: map[uint64]FileInfo{7: {ID: 7, Name: "big.bin", Size: 2048, MIMEType: "application/octet-stream"}},
			request: groupFileRequest("group-file-key", 9, 7), maxBytes: 1024, wantCode: CodeFileTooLarge,
		},
		{
			name: "lookup failure", files: map[uint64]FileInfo{7: {ID: 7, Name: "a.txt", Size: 10, MIMEType: "text/plain"}},
			fileErr: errors.New("database unavailable"), request: groupFileRequest("group-file-key", 9, 7), wantCode: CodeUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, repository := newFileReceiver(t, test.files, test.fileErr, test.maxBytes)
			_, err := service.Send(context.Background(), 1, test.request)
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

type recallPolicy struct{ member bool }

func (recallPolicy) GetGroupByID(context.Context, uint64) (domain.Group, error) {
	return domain.Group{ID: 9, Status: domain.GroupStatusActive}, nil
}
func (policy recallPolicy) IsMember(context.Context, uint64, uint64) (bool, error) {
	return policy.member, nil
}
func (recallPolicy) IsMuted(context.Context, uint64, uint64) (bool, error) { return false, nil }

func TestRecallGroupMessageReplacesHistoryAndIsIdempotent(t *testing.T) {
	repository := &memoryGroupRepository{rows: make(map[string]Message)}
	service, err := NewService(repository, recallPolicy{member: true}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	target, err := service.Send(context.Background(), 1, groupRequest(9, "recall-group-target", "delete me"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Send(context.Background(), 1, groupWithdrawRequest(9, "withdraw-group-1", target.Message.ID))
	if err != nil || !first.CreatedNew || first.Message.Type != WithdrawMessageType || first.Message.Preview != "[撤回]" {
		t.Fatalf("recall = %#v, %v", first, err)
	}
	second, err := service.Send(context.Background(), 1, groupWithdrawRequest(9, "withdraw-group-2", target.Message.ID))
	if err != nil || second.CreatedNew || second.Message.ID != first.Message.ID {
		t.Fatalf("recall retry = %#v, %v", second, err)
	}
	page, err := service.History(context.Background(), 1, 9, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	foundTarget, foundWithdraw := false, false
	for _, item := range page.Items {
		if item.ID == target.Message.ID {
			foundTarget = true
			if item.Type != WithdrawMessageType || item.Preview != "[撤回]" || item.Payload.TextMsg != nil {
				t.Fatalf("group history target not replaced: %#v", item)
			}
		}
		if item.ID == first.Message.ID {
			foundWithdraw = true
		}
	}
	if !foundTarget || !foundWithdraw || len(repository.rows) != 2 {
		t.Fatalf("history/rows target=%v withdraw=%v rows=%d", foundTarget, foundWithdraw, len(repository.rows))
	}
}

func TestRecallGroupMessageValidation(t *testing.T) {
	tests := []struct {
		name         string
		policy       recallPolicy
		targetSender uint64
		targetGroup  uint64
		requestGroup uint64
		wantCode     uint32
	}{
		{name: "target missing", policy: recallPolicy{member: true}, requestGroup: 9, wantCode: CodeWithdrawTarget},
		{name: "wrong group", policy: recallPolicy{member: true}, targetSender: 1, targetGroup: 9, requestGroup: 10, wantCode: CodeWithdrawTarget},
		{name: "not owner", policy: recallPolicy{member: true}, targetSender: 2, targetGroup: 9, requestGroup: 9, wantCode: CodeWithdrawNotOwner},
		{name: "not member", policy: recallPolicy{member: false}, targetSender: 1, targetGroup: 9, requestGroup: 9, wantCode: CodeNotMember},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &memoryGroupRepository{rows: make(map[string]Message), tombstoned: map[uint64]bool{}}
			service, err := NewService(repository, test.policy, DefaultConfig())
			if err != nil {
				t.Fatal(err)
			}
			targetID := uint64(999_999)
			if test.targetSender != 0 {
				target, createErr := repository.PersistWithSessions(context.Background(), Message{
					GroupID: test.targetGroup, SenderID: test.targetSender, ClientMsgID: "seed-group-target",
					Type: TextMessageType, Preview: "seed", Payload: protocol.MessagePayload{Type: TextMessageType, TextMsg: &protocol.TextMessage{Content: "seed"}},
				})
				if createErr != nil {
					t.Fatal(createErr)
				}
				targetID = target.Message.ID
			}
			_, err = service.Send(context.Background(), 1, groupWithdrawRequest(test.requestGroup, "withdraw-group-key", targetID))
			var publicError *PublicError
			if !errors.As(err, &publicError) || publicError.Code != test.wantCode {
				t.Fatalf("error = %v, want code %d", err, test.wantCode)
			}
		})
	}

	t.Run("invalid payload", func(t *testing.T) {
		repository := &memoryGroupRepository{rows: make(map[string]Message)}
		service, _ := NewService(repository, recallPolicy{member: true}, DefaultConfig())
		_, err := service.Send(context.Background(), 1, SendRequest{
			ClientMsgID: "withdraw-empty", GroupID: 9,
			Message: protocol.MessagePayload{Type: WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{}},
		})
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawPayload {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("tombstoned target", func(t *testing.T) {
		repository := &memoryGroupRepository{rows: make(map[string]Message), tombstoned: map[uint64]bool{}}
		service, _ := NewService(repository, recallPolicy{member: true}, DefaultConfig())
		target, _ := service.Send(context.Background(), 1, groupRequest(9, "tomb-group-target", "gone"))
		repository.tombstoned[target.Message.ID] = true
		_, err := service.Send(context.Background(), 1, groupWithdrawRequest(9, "withdraw-tomb", target.Message.ID))
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawRecalled {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("outside window", func(t *testing.T) {
		repository := &memoryGroupRepository{rows: make(map[string]Message)}
		service, _ := NewService(repository, recallPolicy{member: true}, DefaultConfig())
		target, _ := service.Send(context.Background(), 1, groupRequest(9, "window-group-target", "too old"))
		for key, value := range repository.rows {
			if value.ID == target.Message.ID {
				value.CreatedAt = time.Now().Add(-time.Hour)
				repository.rows[key] = value
			}
		}
		_, err := service.Send(context.Background(), 1, groupWithdrawRequest(9, "withdraw-window", target.Message.ID))
		var publicError *PublicError
		if !errors.As(err, &publicError) || publicError.Code != CodeWithdrawWindow {
			t.Fatalf("error = %v", err)
		}
	})
}

func groupRequest(groupID uint64, clientMsgID, content string) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, GroupID: groupID,
		Message: protocol.MessagePayload{Type: TextMessageType, TextMsg: &protocol.TextMessage{Content: content}},
	}
}

func groupWithdrawRequest(groupID uint64, clientMsgID string, targetID uint64) SendRequest {
	return SendRequest{
		GroupID: groupID, ClientMsgID: clientMsgID,
		Message: protocol.MessagePayload{Type: WithdrawMessageType, WithdrawMsg: &protocol.WithdrawMessage{MsgID: targetID}},
	}
}

func groupFileRequest(clientMsgID string, groupID, fileID uint64) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, GroupID: groupID,
		Message: protocol.MessagePayload{Type: FileMessageType, FileMsg: &protocol.FileMessage{FileID: fileID}},
	}
}

func groupImageRequest(clientMsgID string, groupID, fileID uint64) SendRequest {
	return SendRequest{
		ClientMsgID: clientMsgID, GroupID: groupID,
		Message: protocol.MessagePayload{Type: ImageMessageType, ImageMsg: &protocol.ImageMessage{FileID: fileID, Width: 800, Height: 600}},
	}
}
