// Package protocol defines the typed WebSocket boundary shared by Chat pumps
// and future message services. This checkpoint does not implement events.
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
