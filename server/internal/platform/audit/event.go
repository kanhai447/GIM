// Package audit defines the versioned, privacy-safe event contract used by GIM.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const SchemaVersion = 1

const (
	EventUserRegistered       = "user.registered"
	EventAuthLoginSuccess     = "auth.login.success"
	EventAuthLoginFailed      = "auth.login.failed"
	EventAuthLogout           = "auth.logout"
	EventChatMessageAccepted  = "chat.message.accepted"
	EventGroupCreated         = "group.created"
	EventGroupMemberJoined    = "group.member.joined"
	EventGroupMemberKicked    = "group.member.kicked"
	EventGroupDissolved       = "group.dissolved"
	EventGroupMessageAccepted = "group.message.accepted"
	EventFileUploaded         = "file.uploaded"
	EventFileDeleted          = "user_file.deleted"
)

const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusDenied  = "denied"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,127}$`)

var knownEventTypes = map[string]struct{}{
	EventUserRegistered: {}, EventAuthLoginSuccess: {}, EventAuthLoginFailed: {}, EventAuthLogout: {},
	EventChatMessageAccepted: {}, EventGroupCreated: {}, EventGroupMemberJoined: {}, EventGroupMemberKicked: {},
	EventGroupDissolved: {}, EventGroupMessageAccepted: {}, EventFileUploaded: {}, EventFileDeleted: {},
}

// Event is the only payload written to the GIM audit topic.
type Event struct {
	SchemaVersion int             `json:"schema_version"`
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	Service       string          `json:"service"`
	Action        string          `json:"action"`
	UserID        *uint64         `json:"user_id,omitempty"`
	Role          *int32          `json:"role,omitempty"`
	ResourceType  string          `json:"resource_type,omitempty"`
	ResourceID    string          `json:"resource_id,omitempty"`
	RequestID     string          `json:"request_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
	Status        string          `json:"status"`
	ErrorCode     uint32          `json:"error_code,omitempty"`
	IP            string          `json:"ip,omitempty"`
	Method        string          `json:"method,omitempty"`
	Path          string          `json:"path,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

func NewEvent(eventType, service, action, status string) Event {
	return Event{
		SchemaVersion: SchemaVersion,
		EventID:       uuid.NewString(),
		EventType:     eventType,
		Service:       service,
		Action:        action,
		Status:        status,
		OccurredAt:    time.Now().UTC(),
	}
}

func (event Event) Validate() error {
	if event.SchemaVersion != SchemaVersion {
		return errors.New("unsupported audit schema version")
	}
	if _, err := uuid.Parse(event.EventID); err != nil {
		return errors.New("invalid audit event id")
	}
	if _, known := knownEventTypes[event.EventType]; !known {
		return errors.New("unknown audit event type")
	}
	if !namePattern.MatchString(event.Service) || !namePattern.MatchString(event.Action) {
		return errors.New("invalid audit service or action")
	}
	if event.Status != StatusSuccess && event.Status != StatusFailed && event.Status != StatusDenied {
		return errors.New("invalid audit status")
	}
	if event.OccurredAt.IsZero() {
		return errors.New("audit occurred_at is required")
	}
	if event.UserID != nil && *event.UserID == 0 {
		return errors.New("invalid audit user id")
	}
	if event.Role != nil && *event.Role != 1 && *event.Role != 2 {
		return errors.New("invalid audit role")
	}
	if len(event.ResourceType) > 64 || len(event.ResourceID) > 128 || len(event.RequestID) > 128 || len(event.TraceID) > 128 || len(event.IP) > 64 || len(event.Method) > 16 || len(event.Path) > 512 {
		return errors.New("audit field exceeds maximum length")
	}
	if err := ValidateMetadata(event.Metadata); err != nil {
		return fmt.Errorf("audit metadata: %w", err)
	}
	return nil
}

func Uint64(value uint64) *uint64 { return &value }
func Int32(value int32) *int32    { return &value }
func NewRequestID() string        { return uuid.NewString() }

// Publisher is the business-facing abstraction. Implementations must never
// retain or mutate the supplied Event.
type Publisher interface {
	Publish(context.Context, Event) error
}

type NopPublisher struct{}

func (NopPublisher) Publish(context.Context, Event) error { return nil }

func PartitionKey(event Event) string {
	if event.UserID != nil {
		return fmt.Sprintf("user:%d", *event.UserID)
	}
	if strings.TrimSpace(event.ResourceID) != "" {
		return event.ResourceType + ":" + event.ResourceID
	}
	return event.EventType
}
