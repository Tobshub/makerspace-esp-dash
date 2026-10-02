package mqtt

import (
	"strings"
	"testing"
)

func TestParsePublic(t *testing.T) {
	host, port, err := ParsePublic("tcp://localhost:1883")
	if err != nil {
		t.Fatal(err)
	}
	if host != "localhost" || port != 1883 {
		t.Fatalf("got %s:%d", host, port)
	}
}

func TestTopicShape(t *testing.T) {
	got := DeviceTopic("pid", "dev_abc", "telemetry")
	want := "makerspace/v1/projects/pid/devices/dev_abc/telemetry"
	if got != want {
		t.Fatalf("got %s", got)
	}
	if DeviceTopic("pid", "dev_abc", "commands/ack") != "makerspace/v1/projects/pid/devices/dev_abc/commands/ack" {
		t.Fatal("ack topic")
	}
}

func TestParseDeviceTopic(t *testing.T) {
	projectID := "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	deviceKey := "dev_" + strings.Repeat("ab", 10)
	topic := DeviceTopic(projectID, deviceKey, "commands/ack")
	got, ok := ParseDeviceTopic(topic)
	if !ok {
		t.Fatal("expected a device topic")
	}
	if got.ProjectID != strings.ToLower(projectID) || got.DeviceKey != deviceKey || got.Leaf != "commands/ack" {
		t.Fatalf("%+v", got)
	}
	if _, ok := ParseDeviceTopic(DeviceTopic(projectID, deviceKey, "commands")); ok {
		t.Fatal("command publishes are not ingested")
	}
	if _, ok := ParseDeviceTopic(DeviceTopic("not-a-uuid", deviceKey, "telemetry")); ok {
		t.Fatal("project id")
	}
	if IngestFilter != "makerspace/v1/projects/+/devices/+/#" {
		t.Fatal(IngestFilter)
	}
}
