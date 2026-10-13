// Package domain defines persisted safe audit records and query filters.
package domain

import (
	"encoding/json"
	"time"
)

type Log struct {
	ID            uint64          `json:"id"`
	EventID       string          `json:"eventId"`
	SchemaVersion int             `json:"schemaVersion"`
	EventType     string          `json:"eventType"`
	Service       string          `json:"service"`
	Action        string          `json:"action"`
	UserID        *uint64         `json:"userId,omitempty"`
	Role          *int32          `json:"role,omitempty"`
	ResourceType  string          `json:"resourceType,omitempty"`
	ResourceID    string          `json:"resourceId,omitempty"`
	RequestID     string          `json:"requestId,omitempty"`
	TraceID       string          `json:"traceId,omitempty"`
	Status        string          `json:"status"`
	ErrorCode     uint32          `json:"errorCode,omitempty"`
	IP            string          `json:"ip,omitempty"`
	Method        string          `json:"method,omitempty"`
	Path          string          `json:"path,omitempty"`
	OccurredAt    time.Time       `json:"occurredAt"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type Query struct {
	BeforeID  uint64
	Limit     int
	From      *time.Time
	To        *time.Time
	Service   string
	EventType string
	UserID    *uint64
	Status    string
	RequestID string
}

type Page struct {
	Items        []Log  `json:"list"`
	NextBeforeID uint64 `json:"nextBeforeId"`
	HasMore      bool   `json:"hasMore"`
}
