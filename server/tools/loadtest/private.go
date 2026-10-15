package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"sync/atomic"
	"time"
)

type privateRow struct {
	ID         uint64
	SenderID   uint64
	ReceiverID uint64
	ClientID   string
}

type userPair struct{ Sender, Receiver uint64 }

type privateAttempt struct {
	clientIndex int
	frame       map[string]any
}

func runPrivate(arguments []string) error {
	flags := flag.NewFlagSet("private", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "../.tmp/perf/manifest.json", "manifest path")
	connections := flags.Int("connections", 25, "authenticated websocket connections")
	duration := flags.Duration("duration", 30*time.Second, "hold/send duration")
	messageRate := flags.Int("message-rate", 0, "unique messages per second; zero is idle")
	duplicateEvery := flags.Int("duplicate-every", 20, "retry every Nth clientMsgId; zero disables")
	retryUnacked := flags.Bool("retry-unacked", true, "retry unacknowledged clientMsgIds once after the send window")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *connections < 2 || *connections > 2000 || *duration <= 0 || *messageRate < 0 || *messageRate > 5000 || *duplicateEvery < 0 {
		return fmt.Errorf("invalid private websocket parameters")
	}
	value, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	password, err := requiredSecret(passwordEnv)
	if err != nil {
		return err
	}
	api, err := newAPIClient(value.BaseURL, 10*time.Second)
	if err != nil {
		return err
	}
	tokens := make(map[uint64]string, len(value.Users))
	for _, user := range value.Users {
		loggedIn, loginErr := api.login(context.Background(), user.Account, password)
		if loginErr != nil {
			return loginErr
		}
		tokens[user.ID] = loggedIn.Token
	}
	db, err := openPerfDB()
	if err != nil {
		return err
	}
	defer db.Close()
	clientUsers := make([]perfUser, *connections)
	connectionsByUser := make(map[uint64]uint64)
	for index := range clientUsers {
		clientUsers[index] = value.Users[index%len(value.Users)]
		connectionsByUser[clientUsers[index].ID]++
	}
	pairs := make(map[userPair]struct{})
	for index := 0; index+1 < len(clientUsers); index += 2 {
		pairs[userPair{Sender: clientUsers[index].ID, Receiver: clientUsers[index+1].ID}] = struct{}{}
	}
	beforeUnread, err := readPrivateUnread(db, pairs)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	tracker := newWSTracker()
	clients := make([]*wsClient, 0, *connections)
	for index := range clientUsers {
		client, dialErr := newWSClient(ctx, value.BaseURL, "/api/chat/ws/chat", tokens[clientUsers[index].ID], value.Origin,
			func(payload []byte) { tracker.handle(payload, "chat.ack", "chat.message") }, tracker.connected, tracker.dropped)
		if dialErr != nil {
			cancel()
			for _, active := range clients {
				active.close()
			}
			return fmt.Errorf("private websocket connect %d failed", index+1)
		}
		clients = append(clients, client)
	}
	started := time.Now()
	runPrefix := fmt.Sprintf("pc-%x-", started.UnixNano())
	var sent atomic.Uint64
	attempts := make(map[string]privateAttempt)
	finalUnackedRetries := 0
	if *messageRate > 0 {
		interval := time.Second / time.Duration(*messageRate)
		if interval < time.Millisecond {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		deadline := time.NewTimer(*duration)
	loop:
		for {
			select {
			case <-deadline.C:
				break loop
			case <-ticker.C:
				sequence := sent.Add(1)
				senderIndex := int((sequence-1)%uint64(len(clientUsers)/2)) * 2
				receiver := clientUsers[senderIndex+1]
				clientMsgID := fmt.Sprintf("%s%d", runPrefix, sequence)
				frame := map[string]any{"event": "chat.send", "clientMsgId": clientMsgID, "data": map[string]any{"revUserID": receiver.ID, "msg": map[string]any{"type": 1, "textMsg": map[string]string{"content": "performance-message"}}}}
				attempts[clientMsgID] = privateAttempt{clientIndex: senderIndex, frame: frame}
				tracker.sent(clientMsgID, false)
				if writeErr := clients[senderIndex].writeJSON(frame); writeErr != nil {
					tracker.ack.failure(false, true, false)
					continue
				}
				if *duplicateEvery > 0 && sequence%uint64(*duplicateEvery) == 0 {
					tracker.sent(clientMsgID, true)
					if writeErr := clients[senderIndex].writeJSON(frame); writeErr != nil {
						tracker.ack.failure(false, true, false)
					}
				}
			}
		}
		ticker.Stop()
		deadline.Stop()
		time.Sleep(time.Second)
		if *retryUnacked {
			acked := tracker.snapshot().AckedIDs
			for clientMsgID, attempt := range attempts {
				if _, exists := acked[clientMsgID]; exists {
					continue
				}
				tracker.sent(clientMsgID, true)
				finalUnackedRetries++
				if writeErr := clients[attempt.clientIndex].writeJSON(attempt.frame); writeErr != nil {
					tracker.ack.failure(false, true, false)
				}
			}
		}
		time.Sleep(2 * time.Second)
	} else {
		time.Sleep(*duration)
	}
	cancel()
	for _, client := range clients {
		client.close()
	}
	elapsed := time.Since(started)
	rows, err := readPrivateRows(db, runPrefix)
	if err != nil {
		return err
	}
	afterUnread, err := readPrivateUnread(db, pairs)
	if err != nil {
		return err
	}
	snapshot := tracker.snapshot()
	deliveryExpected := uint64(0)
	deliveryCorrect := true
	persistedByPair := make(map[userPair]uint64)
	for _, row := range rows {
		expected := connectionsByUser[row.ReceiverID]
		deliveryExpected += expected
		if snapshot.DeliveryByID[row.ID] != expected {
			deliveryCorrect = false
		}
		persistedByPair[userPair{Sender: row.SenderID, Receiver: row.ReceiverID}]++
	}
	unreadCorrect := true
	for pair, count := range persistedByPair {
		if afterUnread[pair] != beforeUnread[pair]+count {
			unreadCorrect = false
		}
	}
	correct := len(rows) == snapshot.UniqueAcked && snapshot.AckConflicts == 0 && unreadCorrect
	if *messageRate > 0 {
		correct = correct && deliveryCorrect && snapshot.Deliveries == deliveryExpected
	}
	return printJSON(map[string]any{
		"workload": "private.websocket", "gatewayPath": true, "connections": *connections,
		"messageRateTarget": *messageRate, "sentUnique": sent.Load(), "persistedUnique": len(rows),
		"connect": tracker.connect.summarize(elapsed), "ack": tracker.ack.summarize(elapsed),
		"duplicateRetries": snapshot.DuplicateRetries, "duplicateAcks": snapshot.DuplicateAcks,
		"ackMessageIdConflicts": snapshot.AckConflicts, "deliveryExpected": deliveryExpected,
		"deliveryObserved": snapshot.Deliveries, "deliveryCorrect": deliveryCorrect,
		"unreadDeltaCorrect": unreadCorrect, "reconnects": snapshot.Reconnects, "unexpectedDrops": snapshot.Drops,
		"errorCodes": snapshot.ErrorCodes, "finalUnackedRetries": finalUnackedRetries,
		"correctnessPass": correct, "durationMs": elapsed.Milliseconds(), "maximumTestedNotCapacity": true,
	})
}

func readPrivateUnread(db *sql.DB, pairs map[userPair]struct{}) (map[userPair]uint64, error) {
	result := make(map[userPair]uint64, len(pairs))
	for pair := range pairs {
		var count uint64
		err := db.QueryRow(`SELECT unread_count FROM chat_sessions WHERE user_id=? AND peer_user_id=?`, pair.Receiver, pair.Sender).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("read private unread failed")
		}
		result[pair] = count
	}
	return result, nil
}

func readPrivateRows(db *sql.DB, prefix string) ([]privateRow, error) {
	rows, err := db.Query(`SELECT id,send_user_id,rev_user_id,client_msg_id FROM chat_messages WHERE client_msg_id LIKE ? ORDER BY id`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("read private verification rows failed")
	}
	defer rows.Close()
	var result []privateRow
	for rows.Next() {
		var row privateRow
		if err := rows.Scan(&row.ID, &row.SenderID, &row.ReceiverID, &row.ClientID); err != nil {
			return nil, fmt.Errorf("scan private verification row failed")
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
