// Package protocol defines the typed WebSocket boundary shared by Chat pumps
// and message services.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const maxEnvelopeBytes = 1 << 20

var ErrInvalidEnvelope = errors.New("invalid websocket envelope")

type Envelope struct {
	Event       string          `json:"event"`
	RequestID   string          `json:"requestId,omitempty"`
	ClientMsgID string          `json:"clientMsgId,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Error       *Error          `json:"error"`
}

const (
	EventChatSend    = "chat.send"
	EventChatAck     = "chat.ack"
	EventChatMessage = "chat.message"
	EventError       = "error"
)

type Error struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}

type TextMessage struct {
	Content string `json:"content"`
}

type MessagePayload struct {
	Type    uint8        `json:"type"`
	TextMsg *TextMessage `json:"textMsg,omitempty"`
}

type ChatSendData struct {
	ReceiverID uint64         `json:"revUserID"`
	Message    MessagePayload `json:"msg"`
}

type AckData struct {
	MessageID uint64 `json:"messageId"`
	CreatedAt string `json:"createdAt"`
}

// PrivateMessage is shared by chat.message and the future history endpoint.
// clientMsgId remains a sender-side idempotency key; messageId is the server
// identity and ordering cursor.
type PrivateMessage struct {
	MessageID   uint64         `json:"messageId"`
	SenderID    uint64         `json:"sendUserID"`
	ReceiverID  uint64         `json:"revUserID"`
	ClientMsgID string         `json:"clientMsgId"`
	Message     MessagePayload `json:"msg"`
	CreatedAt   string         `json:"createdAt"`
}

func Decode(payload []byte) (Envelope, error) {
	if len(payload) == 0 || len(payload) > maxEnvelopeBytes {
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

func Encode(envelope Envelope) ([]byte, error) {
	return json.Marshal(envelope)
}

func Data(value any) (json.RawMessage, error) {
	return json.Marshal(value)
}
