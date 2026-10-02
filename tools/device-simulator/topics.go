package main

import (
	"fmt"
	"regexp"
)

var (
	projectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	deviceKeyPattern = regexp.MustCompile(`^dev_[0-9a-fA-F]{20}$`)
)

const topicPrefix = "makerspace/v1"

// deviceTopics are the MQTT leaves for one device.
type deviceTopics struct {
	telemetry string
	state     string
	status    string
	commands  string
	ack       string
}

func topicsFor(projectID, deviceKey string) deviceTopics {
	base := fmt.Sprintf("%s/projects/%s/devices/%s/", topicPrefix, projectID, deviceKey)
	return deviceTopics{
		telemetry: base + "telemetry",
		state:     base + "state",
		status:    base + "status",
		commands:  base + "commands",
		ack:       base + "commands/ack",
	}
}
