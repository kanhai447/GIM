// Package adminapi contains transport helpers shared by target-service Admin APIs.
package adminapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxJSONBody = 64 * 1024

// Identity accepts only the identity headers injected by Gateway/Auth.
// Target services still perform this check because frontend route guards are
// never an authorization boundary.
func Identity(request *http.Request) (uint64, bool) {
	if request == nil || strings.TrimSpace(request.Header.Get("Role")) != "1" {
		return 0, false
	}
	id, err := strconv.ParseUint(strings.TrimSpace(request.Header.Get("User-ID")), 10, 64)
	return id, err == nil && id > 0
}

func CursorLimit(values url.Values, fallback, maximum int) (uint64, int, bool) {
	before := uint64(0)
	if text := strings.TrimSpace(values.Get("beforeId")); text != "" {
		parsed, err := strconv.ParseUint(text, 10, 64)
		if err != nil || parsed == 0 {
			return 0, 0, false
		}
		before = parsed
	}
	limit := fallback
	if text := strings.TrimSpace(values.Get("limit")); text != "" {
		parsed, err := strconv.Atoi(text)
		if err != nil || parsed < 1 || parsed > maximum {
			return 0, 0, false
		}
		limit = parsed
	}
	return before, limit, true
}

func DecodeJSON(request *http.Request, target any) bool {
	if request == nil || request.Body == nil {
		return false
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, maxJSONBody+1))
	if err != nil || len(payload) == 0 || len(payload) > maxJSONBody {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}
