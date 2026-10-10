package protocol

import (
	"encoding/json"
	"testing"
)

func TestDecodeTypedEnvelope(t *testing.T) {
	envelope, err := Decode([]byte(`{"event":"foundation.test","requestId":"request-1","data":{"value":1}}`))
	if err != nil || envelope.Event != "foundation.test" || envelope.RequestID != "request-1" || string(envelope.Data) != `{"value":1}` {
		t.Fatalf("Decode() = %#v, %v", envelope, err)
	}
}

func TestDecodeDataUsesTypedStrictBoundary(t *testing.T) {
	data, err := DecodeData[ChatSendData](json.RawMessage(`{"revUserID":42,"msg":{"type":1,"textMsg":{"content":"hello"}}}`))
	if err != nil || data.ReceiverID != 42 || data.Message.TextMsg == nil || data.Message.TextMsg.Content != "hello" {
		t.Fatalf("DecodeData() = %#v, %v", data, err)
	}
	if _, err := DecodeData[ChatSendData](json.RawMessage(`{"revUserID":42,"senderId":1,"msg":{"type":1}}`)); err == nil {
		t.Fatal("DecodeData() accepted a client supplied senderId")
	}
}

func TestDecodeRejectsInvalidEnvelope(t *testing.T) {
	for _, payload := range [][]byte{
		nil,
		[]byte(`{}`),
		[]byte(`{"event":""}`),
		[]byte(`{"event":"test","unknown":true}`),
		[]byte(`{"event":"test"}{"event":"second"}`),
	} {
		if _, err := Decode(payload); err == nil {
			t.Fatalf("Decode() accepted %q", payload)
		}
	}
}
