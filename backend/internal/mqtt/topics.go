package mqtt

import (
	"fmt"
	"regexp"
	"strings"
)

const prefix = "makerspace/v1"

// IngestFilter is the broker subscription for every device leaf.
const IngestFilter = prefix + "/projects/+/devices/+/#"

var (
	deviceTopicPattern = regexp.MustCompile(`^` + prefix + `/projects/([^/]+)/devices/([^/]+)/(.+)$`)
	projectIDPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	deviceKeyPattern   = regexp.MustCompile(`^dev_[0-9a-fA-F]{20}$`)
)

// Topic is one device publish the ingestion pipeline accepts.
type Topic struct {
	ProjectID string
	DeviceKey string
	Leaf      string
}

// ParseDeviceTopic returns the device identity for telemetry, state, status,
// events, and command acknowledgements. Other leaves, including commands the
// API itself publishes, are ignored.
func ParseDeviceTopic(topic string) (Topic, bool) {
	match := deviceTopicPattern.FindStringSubmatch(topic)
	if match == nil {
		return Topic{}, false
	}
	leaf := match[3]
	switch leaf {
	case "telemetry", "state", "status", "events", "commands/ack":
	default:
		return Topic{}, false
	}
	if !projectIDPattern.MatchString(match[1]) || !deviceKeyPattern.MatchString(match[2]) {
		return Topic{}, false
	}
	return Topic{ProjectID: strings.ToLower(match[1]), DeviceKey: strings.ToLower(match[2]), Leaf: leaf}, true
}

// DeviceTopic is the canonical topic for one device leaf, such as "telemetry"
// or "commands/ack".
func DeviceTopic(projectID, deviceKey, leaf string) string {
	return fmt.Sprintf("%s/projects/%s/devices/%s/%s", prefix, projectID, deviceKey, leaf)
}

// TopicTemplates are the patterns shown before a device key exists.
func TopicTemplates() map[string]string {
	base := prefix + "/projects/{projectId}/devices/{deviceKey}/"
	return map[string]string{
		"telemetry":   base + "telemetry",
		"state":       base + "state",
		"status":      base + "status",
		"commands":    base + "commands",
		"commandsAck": base + "commands/ack",
		"events":      base + "events",
	}
}
