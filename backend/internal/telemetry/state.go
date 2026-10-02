package telemetry

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// State is a validated device state document.
// Presence is "online", "offline", or empty when the document does not say.
type State struct {
	Body            json.RawMessage
	FirmwareVersion string
	Presence        string
}

// ParseState accepts a JSON object. Nested fields stay inside the stored document.
// firmwareVersion is kept when it is a short string. status "online" or "offline"
// and boolean "online" set Presence. Any other status value is product data.
func ParseState(payload []byte) (State, error) {
	obj, err := decodeObject(payload)
	if err != nil {
		return State{}, err
	}
	body := append([]byte(nil), bytes.TrimSpace(payload)...)
	state := State{Body: body}
	if raw, ok := obj["firmwareVersion"]; ok {
		state.FirmwareVersion = firmwareVersion(raw)
	}
	presence, err := presenceFrom(obj["status"], obj["online"])
	if err != nil {
		return State{}, err
	}
	state.Presence = presence
	return state, nil
}

// ParsePresence reads a status-topic document. Presence must be explicit.
func ParsePresence(payload []byte) (bool, error) {
	obj, err := decodeObject(payload)
	if err != nil {
		return false, err
	}
	if _, ok := obj["status"]; !ok {
		if _, ok := obj["online"]; !ok {
			return false, &Error{Reason: "status required"}
		}
	}
	if raw, ok := obj["status"]; ok && !explicitPresence(raw) && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if _, isString := jsonString(raw); isString {
			return false, &Error{Reason: "status required"}
		}
	}
	presence, err := presenceFrom(obj["status"], obj["online"])
	if err != nil {
		return false, err
	}
	switch presence {
	case "online":
		return true, nil
	case "offline":
		return false, nil
	default:
		return false, &Error{Reason: "status required"}
	}
}

// DeviceEvent is a debug publish on the events topic.
type DeviceEvent struct {
	Type string
	Body json.RawMessage
}

var eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// ParseDeviceEvent requires a JSON object. A short slug in "type" is kept.
// Anything else is stored as event type "event" and the original body is unchanged.
func ParseDeviceEvent(payload []byte) (DeviceEvent, error) {
	obj, err := decodeObject(payload)
	if err != nil {
		return DeviceEvent{}, err
	}
	eventType := "event"
	if raw, ok := obj["type"]; ok {
		if text, ok := jsonString(raw); ok && eventTypePattern.MatchString(text) {
			eventType = text
		}
	}
	return DeviceEvent{Type: eventType, Body: append([]byte(nil), bytes.TrimSpace(payload)...)}, nil
}

func presenceFrom(statusRaw, onlineRaw json.RawMessage) (string, error) {
	var fromStatus string
	if explicitPresence(statusRaw) {
		text, _ := jsonString(statusRaw)
		fromStatus = text
	}
	var fromOnline string
	if len(bytes.TrimSpace(onlineRaw)) > 0 {
		trim := bytes.TrimSpace(onlineRaw)
		if trim[0] == 't' || trim[0] == 'f' {
			var online bool
			if err := json.Unmarshal(trim, &online); err != nil {
				return "", &Error{Reason: "invalid json"}
			}
			fromOnline = "offline"
			if online {
				fromOnline = "online"
			}
		}
	}
	if fromStatus != "" && fromOnline != "" && fromStatus != fromOnline {
		return "", &Error{Reason: "conflicting presence"}
	}
	if fromStatus != "" {
		return fromStatus, nil
	}
	return fromOnline, nil
}

func explicitPresence(raw json.RawMessage) bool {
	text, ok := jsonString(raw)
	return ok && (text == "online" || text == "offline")
}

func jsonString(raw json.RawMessage) (string, bool) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || trim[0] != '"' {
		return "", false
	}
	var text string
	if err := json.Unmarshal(trim, &text); err != nil {
		return "", false
	}
	return text, true
}

func firmwareVersion(raw json.RawMessage) string {
	text, ok := jsonString(raw)
	if !ok {
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 64 || strings.ContainsAny(text, "\r\n") {
		return ""
	}
	return text
}
