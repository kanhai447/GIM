package ws

import "log"

type DisconnectReason string

const (
	DisconnectNormal   DisconnectReason = "normal"
	DisconnectTimeout  DisconnectReason = "timeout"
	DisconnectAbnormal DisconnectReason = "abnormal"
	DisconnectSlow     DisconnectReason = "slow_client"
	DisconnectShutdown DisconnectReason = "shutdown"
)

type ClientIdentity struct {
	UserID   uint64
	ClientID string
}

type LifecycleObserver interface {
	Connected(ClientIdentity)
	Disconnected(ClientIdentity, DisconnectReason)
}

type standardLifecycleObserver struct{ logger *log.Logger }

func NewStandardLifecycleObserver(logger *log.Logger) LifecycleObserver {
	if logger == nil {
		return noopLifecycleObserver{}
	}
	return standardLifecycleObserver{logger: logger}
}

func (observer standardLifecycleObserver) Connected(identity ClientIdentity) {
	observer.logger.Printf("group websocket connected user_id=%d client_id=%s", identity.UserID, identity.ClientID)
}

func (observer standardLifecycleObserver) Disconnected(identity ClientIdentity, reason DisconnectReason) {
	observer.logger.Printf("group websocket disconnected user_id=%d client_id=%s reason=%s", identity.UserID, identity.ClientID, reason)
}

type noopLifecycleObserver struct{}

func (noopLifecycleObserver) Connected(ClientIdentity)                      {}
func (noopLifecycleObserver) Disconnected(ClientIdentity, DisconnectReason) {}
