package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEventValidationAndSerialization(t *testing.T) {
	metadata, err := Metadata(map[string]any{"message_id": uint64(9), "message_type": 1})
	if err != nil {
		t.Fatal(err)
	}
	event := NewEvent(EventChatMessageAccepted, "chat", "message.accept", StatusSuccess)
	event.UserID = Uint64(7)
	event.ResourceType, event.ResourceID, event.Metadata = "chat_message", "9", metadata
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "content") || !strings.Contains(string(payload), `"schema_version":1`) {
		t.Fatalf("unsafe or incomplete payload: %s", payload)
	}
	second := NewEvent(EventChatMessageAccepted, "chat", "message.accept", StatusSuccess)
	if event.EventID == second.EventID {
		t.Fatal("event IDs are not unique")
	}
}

func TestMetadataRejectsSensitiveAndStructuredValues(t *testing.T) {
	for _, key := range []string{"password", "jwt_token", "Authorization", "message_content", "storage_path", "dsn"} {
		if _, err := Metadata(map[string]any{key: "sensitive"}); err == nil {
			t.Fatalf("sensitive key %q accepted", key)
		}
	}
	if _, err := Metadata(map[string]any{"message_id": map[string]string{"raw": "value"}}); err == nil {
		t.Fatal("structured metadata value accepted")
	}
	if err := ValidateMetadata(json.RawMessage(`{"password":"secret"}`)); err == nil {
		t.Fatal("persisted sensitive metadata accepted")
	}
}
