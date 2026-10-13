package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const maxMetadataBytes = 8 << 10

var allowedMetadataKeys = map[string]struct{}{
	"failure_category": {}, "message_id": {}, "sender_id": {}, "receiver_id": {}, "group_id": {},
	"message_type": {}, "size_bytes": {}, "user_file_id": {}, "file_object_uid": {}, "mime_type": {},
	"member_id": {}, "target_user_id": {}, "client_msg_id": {}, "reference_created": {}, "object_created": {},
}

var forbiddenKeyFragments = []string{
	"password", "passwd", "pwd", "token", "authorization", "cookie", "secret", "credential", "private_key", "dsn", "storage_path", "content", "body",
}

// Metadata builds a JSON object only from the explicit V1 allowlist. Values
// must be JSON scalars; request bodies and arbitrary objects are rejected.
func Metadata(values map[string]any) (json.RawMessage, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for key, value := range values {
		if !safeMetadataKey(key) {
			return nil, fmt.Errorf("metadata key %q is not allowed", key)
		}
		switch value.(type) {
		case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, nil:
		default:
			return nil, fmt.Errorf("metadata value for %q must be scalar", key)
		}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return nil, errors.New("encode audit metadata")
	}
	if len(payload) > maxMetadataBytes {
		return nil, errors.New("audit metadata exceeds maximum size")
	}
	return payload, nil
}

func ValidateMetadata(payload json.RawMessage) error {
	if len(payload) == 0 {
		return nil
	}
	if len(payload) > maxMetadataBytes {
		return errors.New("exceeds maximum size")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil {
		return errors.New("must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("must contain one JSON object")
	}
	for key, value := range values {
		if !safeMetadataKey(key) {
			return fmt.Errorf("key %q is not allowed", key)
		}
		switch value.(type) {
		case string, bool, json.Number, nil:
		default:
			return fmt.Errorf("value for %q must be scalar", key)
		}
	}
	return nil
}

func safeMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, fragment := range forbiddenKeyFragments {
		if strings.Contains(normalized, fragment) {
			return false
		}
	}
	_, allowed := allowedMetadataKeys[normalized]
	return allowed
}
