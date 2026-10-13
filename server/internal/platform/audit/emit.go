package audit

import (
	"context"
	"log"
)

// EmitBestEffort is the explicit non-transactional boundary: an audit failure
// is observable through the ordinary process logger but never returned to the
// already successful business operation.
func EmitBestEffort(ctx context.Context, publisher Publisher, event Event, logger *log.Logger) {
	if publisher == nil {
		return
	}
	if err := publisher.Publish(ctx, event); err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("audit enqueue degraded event_id=%s event_type=%s", event.EventID, event.EventType)
	}
}
