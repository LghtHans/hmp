package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// QueuedMessage is one pending message waiting for a peer to come online.
type QueuedMessage struct {
	TargetDeviceID string    `json:"target_device_id"`
	MessageType    uint8     `json:"message_type"` // MsgText or reused for mail
	Content        string    `json:"content"`
	QueuedAt       time.Time `json:"queued_at"`
}

// queueDir returns ~/.hmp/queue, creating it if needed.
func queueDir() (string, error) {
	dir, err := hmpDir()
	if err != nil {
		return "", err
	}
	qdir := filepath.Join(dir, "queue")
	if err := os.MkdirAll(qdir, 0700); err != nil {
		return "", err
	}
	return qdir, nil
}

// queueFilePath returns the queue file for a specific target peer.
// One file per peer, each line is one JSON-encoded QueuedMessage (JSONL).
func queueFilePath(targetDeviceID string) (string, error) {
	dir, err := queueDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, targetDeviceID+".jsonl"), nil
}

// EnqueueMessage appends a message to the target peer's queue file.
// Called when a send attempt fails because the peer is unreachable.
func EnqueueMessage(msg QueuedMessage) error {
	path, err := queueFilePath(msg.TargetDeviceID)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// LoadQueue reads all pending messages queued for a specific peer.
// Returns an empty slice (not an error) if there's nothing queued.
func LoadQueue(targetDeviceID string) ([]QueuedMessage, error) {
	path, err := queueFilePath(targetDeviceID)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []QueuedMessage{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var messages []QueuedMessage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg QueuedMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip corrupted lines rather than failing the whole load.
			continue
		}
		messages = append(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// ClearQueue deletes the queue file for a peer, called after a
// successful flush (all queued messages delivered).
func ClearQueue(targetDeviceID string) error {
	path, err := queueFilePath(targetDeviceID)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
