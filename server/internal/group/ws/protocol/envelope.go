// Package protocol defines the Group WebSocket transport boundary.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const MaxEnvelopeBytes = 1 << 20

var ErrInvalidEnvelope = errors.New("invalid group websocket envelope")

const (
	EventGroupSend    = "group.send"
	EventGroupAck     = "group.ack"
	EventGroupMessage = "group.message"
	EventError        = "error"

	ErrorMalformed   uint32 = 2401
	ErrorUnsupported uint32 = 2402
)

type Envelope struct {
	Event       string          `json:"event"`
	RequestID   string          `json:"requestId,omitempty"`
	ClientMsgID string          `json:"clientMsgId,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Error       *Error          `json:"error"`
}

type Error struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}

func Decode(payload []byte) (Envelope, error) {
	if len(payload) == 0 || len(payload) > MaxEnvelopeBytes {
		return Envelope{}, ErrInvalidEnvelope
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil || strings.TrimSpace(envelope.Event) == "" {
		return Envelope{}, ErrInvalidEnvelope
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Envelope{}, ErrInvalidEnvelope
	}
	return envelope, nil
}

func EncodeError(requestID, clientMsgID string, code uint32, message string) ([]byte, error) {
	return json.Marshal(Envelope{
		Event: EventError, RequestID: requestID, ClientMsgID: clientMsgID,
		Error: &Error{Code: code, Message: message},
	})
}

type TextMessage struct {
	Content string `json:"content"`
}

// ImageMessage references an uploaded user_files row. URL, MIMEType and Size are
// filled by the server from file_objects; Width/Height are optional display hints.
type ImageMessage struct {
	FileID   uint64 `json:"fileID"`
	URL      string `json:"url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
}

type MessagePayload struct {
	Type     uint8         `json:"type"`
	TextMsg  *TextMessage  `json:"textMsg,omitempty"`
	ImageMsg *ImageMessage `json:"imageMsg,omitempty"`
}

type GroupSendData struct {
	GroupID uint64         `json:"groupID"`
	Message MessagePayload `json:"msg"`
}

type AckData struct {
	MessageID uint64 `json:"messageId"`
	GroupID   uint64 `json:"groupID"`
	CreatedAt string `json:"createdAt"`
}

type GroupMessage struct {
	MessageID   uint64         `json:"messageId"`
	GroupID     uint64         `json:"groupID"`
	SenderID    uint64         `json:"sendUserID"`
	ClientMsgID string         `json:"clientMsgId"`
	Message     MessagePayload `json:"msg"`
	CreatedAt   string         `json:"createdAt"`
}

func DecodeData[T any](raw json.RawMessage) (T, error) {
	var value T
	if len(raw) == 0 {
		return value, ErrInvalidEnvelope
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, ErrInvalidEnvelope
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return value, ErrInvalidEnvelope
	}
	return value, nil
}

func Encode(envelope Envelope) ([]byte, error) { return json.Marshal(envelope) }
func Data(value any) (json.RawMessage, error)  { return json.Marshal(value) }
