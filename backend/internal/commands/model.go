package commands

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	StatusPending      = "pending"
	StatusPublished    = "published"
	StatusAcknowledged = "acknowledged"
	StatusFailed       = "failed"
	StatusTimedOut     = "timed_out"
)

// Command is one request sent to a device.
type Command struct {
	ID             string          `json:"id"`
	ProjectID      string          `json:"projectId"`
	DeviceID       string          `json:"deviceId"`
	Command        string          `json:"command"`
	Payload        json.RawMessage `json:"payload"`
	Status         string          `json:"status"`
	RequestedBy    string          `json:"requestedBy,omitempty"`
	RequestedAt    time.Time       `json:"requestedAt"`
	PublishedAt    *time.Time      `json:"publishedAt"`
	AcknowledgedAt *time.Time      `json:"acknowledgedAt"`
	FailedAt       *time.Time      `json:"failedAt"`
	CorrelationID  string          `json:"correlationId"`
	ErrorMessage   string          `json:"errorMessage"`
}

// Ack is a device acknowledgement. ID is the correlation id.
type Ack struct {
	ID      string
	Success bool
	Error   string
}

// Publisher sends one MQTT message. Implementations wait for the broker handshake, not the device.
type Publisher interface {
	Publish(topic string, qos byte, retained bool, payload []byte) error
}

type wire struct {
	ID        string          `json:"id"`
	Command   string          `json:"command"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

func newCorrelationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cmd_" + hex.EncodeToString(buf), nil
}

// ParseAck reads the acknowledgement contract. success is required.
func ParseAck(payload []byte) (Ack, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Ack{}, errors.New("acknowledgement must be a JSON object")
	}
	var ack Ack
	if err := json.Unmarshal(raw["id"], &ack.ID); err != nil || strings.TrimSpace(ack.ID) == "" {
		return Ack{}, errors.New("acknowledgement id is required")
	}
	ack.ID = strings.TrimSpace(ack.ID)
	success, ok := raw["success"]
	if !ok {
		return Ack{}, errors.New("acknowledgement success is required")
	}
	if err := json.Unmarshal(success, &ack.Success); err != nil {
		return Ack{}, errors.New("acknowledgement success must be a boolean")
	}
	if rawError, exists := raw["error"]; exists && string(rawError) != "null" {
		if err := json.Unmarshal(rawError, &ack.Error); err != nil {
			return Ack{}, errors.New("acknowledgement error must be a string")
		}
	}
	return ack, nil
}

func marshalWire(cmd Command, at time.Time) ([]byte, error) {
	payload := cmd.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	return json.Marshal(wire{
		ID:        cmd.CorrelationID,
		Command:   cmd.Command,
		Payload:   payload,
		Timestamp: at.UTC().UnixMilli(),
	})
}
