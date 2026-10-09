package protocol

import "testing"

func TestDecodeTypedEnvelope(t *testing.T) {
	envelope, err := Decode([]byte(`{"event":"foundation.test","requestId":"request-1","data":{"value":1}}`))
	if err != nil || envelope.Event != "foundation.test" || envelope.RequestID != "request-1" || string(envelope.Data) != `{"value":1}` {
		t.Fatalf("Decode() = %#v, %v", envelope, err)
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
