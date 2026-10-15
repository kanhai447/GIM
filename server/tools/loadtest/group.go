package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"sync/atomic"
	"time"
)

type groupRow struct {
	ID       uint64
	SenderID uint64
	ClientID string
}

func runGroup(arguments []string) error {
	flags := flag.NewFlagSet("group", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "../.tmp/perf/manifest.json", "manifest path")
	groupSize := flags.Int("group-size", 5, "prepared group size")
	devicesPerMember := flags.Int("devices-per-member", 1, "active websocket clients per member")
	duration := flags.Duration("duration", 30*time.Second, "hold/send duration")
	messageRate := flags.Int("message-rate", 10, "unique messages per second; zero is idle")
	duplicateEvery := flags.Int("duplicate-every", 20, "retry every Nth clientMsgId; zero disables")
	retryUnacked := flags.Bool("retry-unacked", true, "retry unacknowledged clientMsgIds once after the send window")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *groupSize < 2 || *devicesPerMember < 1 || *devicesPerMember > 5 || *duration <= 0 || *messageRate < 0 || *messageRate > 2000 || *duplicateEvery < 0 {
		return fmt.Errorf("invalid group websocket parameters")
	}
	value, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	var selected perfGroup
	for _, group := range value.Groups {
		if group.Size == *groupSize {
			selected = group
			break
		}
	}
	if selected.ID == 0 {
		return fmt.Errorf("manifest has no prepared group of size %d", *groupSize)
	}
	password, err := requiredSecret(passwordEnv)
	if err != nil {
		return err
	}
	api, err := newAPIClient(value.BaseURL, 10*time.Second)
	if err != nil {
		return err
	}
	usersByID := make(map[uint64]perfUser, len(value.Users))
	for _, user := range value.Users {
		usersByID[user.ID] = user
	}
	tokens := make(map[uint64]string, len(selected.MemberIDs))
	for _, userID := range selected.MemberIDs {
		user := usersByID[userID]
		loggedIn, loginErr := api.login(context.Background(), user.Account, password)
		if loginErr != nil {
			return loginErr
		}
		tokens[userID] = loggedIn.Token
	}
	db, err := openPerfDB()
	if err != nil {
		return err
	}
	defer db.Close()
	beforeUnread, err := readGroupUnread(db, selected)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	tracker := newWSTracker()
	clientCount := len(selected.MemberIDs) * *devicesPerMember
	clients := make([]*wsClient, 0, clientCount)
	for _, memberID := range selected.MemberIDs {
		for device := 0; device < *devicesPerMember; device++ {
			client, dialErr := newWSClient(ctx, value.BaseURL, "/api/group/ws/chat", tokens[memberID], value.Origin,
				func(payload []byte) { tracker.handle(payload, "group.ack", "group.message") }, tracker.connected, tracker.dropped)
			if dialErr != nil {
				cancel()
				for _, active := range clients {
					active.close()
				}
				return fmt.Errorf("group websocket connect failed")
			}
			clients = append(clients, client)
		}
	}
	started := time.Now()
	runPrefix := fmt.Sprintf("gc-%x-", started.UnixNano())
	var sent atomic.Uint64
	attempts := make(map[string]map[string]any)
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
				clientMsgID := fmt.Sprintf("%s%d", runPrefix, sequence)
				frame := map[string]any{"event": "group.send", "clientMsgId": clientMsgID, "data": map[string]any{"groupID": selected.ID, "msg": map[string]any{"type": 1, "textMsg": map[string]string{"content": "performance-message"}}}}
				attempts[clientMsgID] = frame
				tracker.sent(clientMsgID, false)
				if writeErr := clients[0].writeJSON(frame); writeErr != nil {
					tracker.ack.failure(false, true, false)
					continue
				}
				if *duplicateEvery > 0 && sequence%uint64(*duplicateEvery) == 0 {
					tracker.sent(clientMsgID, true)
					if writeErr := clients[0].writeJSON(frame); writeErr != nil {
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
			for clientMsgID, frame := range attempts {
				if _, exists := acked[clientMsgID]; exists {
					continue
				}
				tracker.sent(clientMsgID, true)
				finalUnackedRetries++
				if writeErr := clients[0].writeJSON(frame); writeErr != nil {
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
	rows, err := readGroupRows(db, selected.ID, runPrefix)
	if err != nil {
		return err
	}
	afterUnread, err := readGroupUnread(db, selected)
	if err != nil {
		return err
	}
	snapshot := tracker.snapshot()
	deliveryExpected := uint64(len(rows) * clientCount)
	deliveryCorrect := true
	for _, row := range rows {
		if snapshot.DeliveryByID[row.ID] != uint64(clientCount) {
			deliveryCorrect = false
		}
	}
	unreadCorrect := true
	for _, memberID := range selected.MemberIDs {
		expected := uint64(len(rows))
		if memberID == selected.MemberIDs[0] {
			expected = 0
		}
		if afterUnread[memberID] != beforeUnread[memberID]+expected {
			unreadCorrect = false
		}
	}
	correct := len(rows) == snapshot.UniqueAcked && snapshot.AckConflicts == 0 && unreadCorrect
	if *messageRate > 0 {
		correct = correct && deliveryCorrect && snapshot.Deliveries == deliveryExpected
	}
	return printJSON(map[string]any{
		"workload": "group.websocket", "gatewayPath": true, "groupSize": selected.Size,
		"devicesPerMember": *devicesPerMember, "connections": clientCount, "messageRateTarget": *messageRate,
		"sentUnique": sent.Load(), "persistedUnique": len(rows), "connect": tracker.connect.summarize(elapsed),
		"ack": tracker.ack.summarize(elapsed), "duplicateRetries": snapshot.DuplicateRetries,
		"duplicateAcks": snapshot.DuplicateAcks, "ackMessageIdConflicts": snapshot.AckConflicts,
		"deliveryExpected": deliveryExpected, "deliveryObserved": snapshot.Deliveries,
		"deliveryCorrect": deliveryCorrect, "unreadDeltaCorrect": unreadCorrect,
		"reconnects": snapshot.Reconnects, "unexpectedDrops": snapshot.Drops,
		"errorCodes": snapshot.ErrorCodes, "finalUnackedRetries": finalUnackedRetries,
		"correctnessPass": correct, "durationMs": elapsed.Milliseconds(), "maximumTestedNotCapacity": true,
	})
}

func readGroupUnread(db *sql.DB, group perfGroup) (map[uint64]uint64, error) {
	result := make(map[uint64]uint64, len(group.MemberIDs))
	for _, userID := range group.MemberIDs {
		var count uint64
		err := db.QueryRow(`SELECT unread_count FROM group_sessions WHERE user_id=? AND group_id=?`, userID, group.ID).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("read group unread failed")
		}
		result[userID] = count
	}
	return result, nil
}

func readGroupRows(db *sql.DB, groupID uint64, prefix string) ([]groupRow, error) {
	rows, err := db.Query(`SELECT id,send_user_id,client_msg_id FROM group_messages WHERE group_id=? AND client_msg_id LIKE ? ORDER BY id`, groupID, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("read group verification rows failed")
	}
	defer rows.Close()
	var result []groupRow
	for rows.Next() {
		var row groupRow
		if err := rows.Scan(&row.ID, &row.SenderID, &row.ClientID); err != nil {
			return nil, fmt.Errorf("scan group verification row failed")
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
