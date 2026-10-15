package main

import (
	"encoding/json"
	"sync"
	"time"
)

type pendingMessage struct {
	started   time.Time
	messageID uint64
	acks      int
}

type wsTracker struct {
	mu sync.Mutex

	connect recorder
	ack     recorder

	pending          map[string]*pendingMessage
	uniqueAcked      map[string]uint64
	deliveryByID     map[uint64]uint64
	duplicateRetries uint64
	duplicateAcks    uint64
	ackConflicts     uint64
	errors           uint64
	drops            uint64
	reconnects       uint64
	errorCodes       map[uint32]uint64
}

func newWSTracker() *wsTracker {
	return &wsTracker{pending: make(map[string]*pendingMessage), uniqueAcked: make(map[string]uint64), deliveryByID: make(map[uint64]uint64), errorCodes: make(map[uint32]uint64)}
}

func (tracker *wsTracker) sent(clientMsgID string, duplicate bool) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if _, exists := tracker.pending[clientMsgID]; !exists {
		tracker.pending[clientMsgID] = &pendingMessage{started: time.Now()}
	}
	if duplicate {
		tracker.duplicateRetries++
	}
}

func (tracker *wsTracker) connected(latency time.Duration, reconnect bool) {
	tracker.connect.success(latency)
	if reconnect {
		tracker.mu.Lock()
		tracker.reconnects++
		tracker.mu.Unlock()
	}
}

func (tracker *wsTracker) dropped() {
	tracker.mu.Lock()
	tracker.drops++
	tracker.mu.Unlock()
}

func (tracker *wsTracker) handle(payload []byte, ackEvent, messageEvent string) {
	var envelope struct {
		Event       string          `json:"event"`
		ClientMsgID string          `json:"clientMsgId"`
		Data        json.RawMessage `json:"data"`
		Error       *struct {
			Code uint32 `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		tracker.ack.failure(false, true, false)
		return
	}
	if envelope.Event == "error" || envelope.Error != nil {
		tracker.mu.Lock()
		tracker.errors++
		if envelope.Error != nil {
			tracker.errorCodes[envelope.Error.Code]++
		}
		tracker.mu.Unlock()
		tracker.ack.failure(false, false, true)
		return
	}
	if envelope.Event == ackEvent {
		var data struct {
			MessageID uint64 `json:"messageId"`
		}
		if err := json.Unmarshal(envelope.Data, &data); err != nil || data.MessageID == 0 {
			tracker.ack.failure(false, true, false)
			return
		}
		tracker.mu.Lock()
		pending := tracker.pending[envelope.ClientMsgID]
		if pending == nil {
			tracker.mu.Unlock()
			return
		}
		pending.acks++
		latency := time.Since(pending.started)
		if pending.messageID == 0 {
			pending.messageID = data.MessageID
			tracker.uniqueAcked[envelope.ClientMsgID] = data.MessageID
			tracker.mu.Unlock()
			tracker.ack.success(latency)
			return
		}
		if pending.messageID != data.MessageID {
			tracker.ackConflicts++
		} else {
			tracker.duplicateAcks++
		}
		tracker.mu.Unlock()
		return
	}
	if envelope.Event == messageEvent {
		var data struct {
			MessageID uint64 `json:"messageId"`
		}
		if err := json.Unmarshal(envelope.Data, &data); err != nil || data.MessageID == 0 {
			tracker.mu.Lock()
			tracker.errors++
			tracker.mu.Unlock()
			return
		}
		tracker.mu.Lock()
		tracker.deliveryByID[data.MessageID]++
		tracker.mu.Unlock()
	}
}

type trackerSnapshot struct {
	UniqueAcked      int
	Deliveries       uint64
	DuplicateRetries uint64
	DuplicateAcks    uint64
	AckConflicts     uint64
	Errors           uint64
	Drops            uint64
	Reconnects       uint64
	ErrorCodes       map[uint32]uint64
	AckedIDs         map[string]uint64
	DeliveryByID     map[uint64]uint64
}

func (tracker *wsTracker) snapshot() trackerSnapshot {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	result := trackerSnapshot{
		UniqueAcked: len(tracker.uniqueAcked), DuplicateRetries: tracker.duplicateRetries,
		DuplicateAcks: tracker.duplicateAcks, AckConflicts: tracker.ackConflicts, Errors: tracker.errors,
		Drops: tracker.drops, Reconnects: tracker.reconnects, AckedIDs: make(map[string]uint64, len(tracker.uniqueAcked)),
		DeliveryByID: make(map[uint64]uint64, len(tracker.deliveryByID)), ErrorCodes: make(map[uint32]uint64, len(tracker.errorCodes)),
	}
	for key, value := range tracker.uniqueAcked {
		result.AckedIDs[key] = value
	}
	for key, value := range tracker.deliveryByID {
		result.DeliveryByID[key] = value
		result.Deliveries += value
	}
	for key, value := range tracker.errorCodes {
		result.ErrorCodes[key] = value
	}
	return result
}
