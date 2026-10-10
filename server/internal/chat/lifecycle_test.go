package chat

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestStandardLifecycleObserverLogsOnlySafeConnectionMetadata(t *testing.T) {
	var output bytes.Buffer
	observer := NewStandardLifecycleObserver(log.New(&output, "", 0))
	identity := ClientIdentity{UserID: 75, ClientID: "opaque-client"}
	observer.Connected(identity)
	observer.Disconnected(identity, DisconnectTimeout)
	logged := output.String()
	for _, expected := range []string{"connected user_id=75 client_id=opaque-client", "reason=timeout"} {
		if !strings.Contains(logged, expected) {
			t.Fatalf("log missing %q: %q", expected, logged)
		}
	}
	for _, forbidden := range []string{"token=", "authorization=", "secret="} {
		if strings.Contains(strings.ToLower(logged), forbidden) {
			t.Fatalf("log contains forbidden field %q", forbidden)
		}
	}
	NewStandardLifecycleObserver(nil).Connected(identity)
}
