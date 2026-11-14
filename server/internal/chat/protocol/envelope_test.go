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

func TestDecodeImageMessagePayload(t *testing.T) {
	data, err := DecodeData[ChatSendData](json.RawMessage(`{"revUserID":42,"msg":{"type":2,"imageMsg":{"fileID":7,"width":800,"height":600}}}`))
	if err != nil || data.ReceiverID != 42 || data.Message.Type != 2 ||
		data.Message.ImageMsg == nil || data.Message.ImageMsg.FileID != 7 ||
		data.Message.ImageMsg.Width != 800 || data.Message.ImageMsg.Height != 600 {
		t.Fatalf("DecodeData() = %#v, %v", data, err)
	}
	if _, err := DecodeData[ChatSendData](json.RawMessage(`{"revUserID":42,"msg":{"type":2,"imageMsg":{"fileID":7,"senderId":1}}}`)); err == nil {
		t.Fatal("DecodeData() accepted an unknown image field")
	}
}

func TestDecodeFileMessagePayload(t *testing.T) {
	data, err := DecodeData[ChatSendData](json.RawMessage(`{"revUserID":42,"msg":{"type":3,"fileMsg":{"fileID":7,"fileName":"report.pdf","size":4096}}}`))
	if err != nil || data.Message.Type != 3 ||
		data.Message.FileMsg == nil || data.Message.FileMsg.FileID != 7 ||
		data.Message.FileMsg.FileName != "report.pdf" || data.Message.FileMsg.Size != 4096 {
		t.Fatalf("DecodeData() = %#v, %v", data, err)
	}
	if _, err := DecodeData[ChatSendData](json.RawMessage(`{"revUserID":42,"msg":{"type":3,"fileMsg":{"fileID":7,"unknown":1}}}`)); err == nil {
		t.Fatal("DecodeData() accepted an unknown file field")
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
