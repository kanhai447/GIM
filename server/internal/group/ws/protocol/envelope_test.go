package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDecodeAndErrorEnvelope(t *testing.T) {
	envelope, err := Decode([]byte(`{"event":"group.send","requestId":"r1","clientMsgId":"c1","data":{"groupID":7}}`))
	if err != nil || envelope.Event != EventGroupSend || envelope.RequestID != "r1" || envelope.ClientMsgID != "c1" {
		t.Fatalf("Decode() = %#v, %v", envelope, err)
	}
	payload, err := EncodeError("r1", "c1", ErrorUnsupported, "not implemented")
	if err != nil {
		t.Fatal(err)
	}
	var response Envelope
	if err := json.Unmarshal(payload, &response); err != nil || response.Event != EventError || response.Error == nil || response.Error.Code != ErrorUnsupported {
		t.Fatalf("error envelope = %#v, %v", response, err)
	}
}

func TestDecodeRejectsMalformedEnvelope(t *testing.T) {
	for _, payload := range [][]byte{nil, []byte(`{}`), []byte(`{"event":"group.send","extra":true}`), []byte(`{"event":"group.send"}{}`)} {
		if _, err := Decode(payload); !errors.Is(err, ErrInvalidEnvelope) {
			t.Fatalf("Decode(%q) error = %v", payload, err)
		}
	}
}
