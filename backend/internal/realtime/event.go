package realtime

import "time"

const (
	DeviceOnline      = "device.online"
	DeviceOffline     = "device.offline"
	TelemetryReceived = "telemetry.received"
	StateUpdated      = "state.updated"
	CommandUpdated    = "command.updated"
)

// Event is one server-to-browser message. The stream is already project-scoped.
type Event struct {
	Type      string    `json:"type"`
	DeviceID  string    `json:"deviceId"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// Publisher receives events after they are stored.
type Publisher interface {
	Publish(projectID string, event Event)
}
