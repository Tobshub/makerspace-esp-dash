package realtime

import (
	"testing"
	"time"
)

func TestHubDeliversAndDropsSlowSubscribers(t *testing.T) {
	hub := New()
	events, cancel := hub.Subscribe("project-a")
	defer cancel()
	_, otherCancel := hub.Subscribe("project-b")
	defer otherCancel()

	want := Event{Type: TelemetryReceived, DeviceID: "device-1", Timestamp: time.Unix(10, 0).UTC(), Data: map[string]any{"temperature": 28.4}}
	hub.Publish("project-a", want)
	hub.Publish("project-b", Event{Type: DeviceOffline, DeviceID: "other"})

	select {
	case got := <-events:
		if got.Type != want.Type || got.DeviceID != want.DeviceID {
			t.Fatalf("%#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	for i := 0; i < 64; i++ {
		hub.Publish("project-a", Event{Type: TelemetryReceived, DeviceID: "device-1"})
	}
}

func TestHubUnsubscribe(t *testing.T) {
	hub := New()
	events, cancel := hub.Subscribe("project-a")
	cancel()
	select {
	case _, open := <-events:
		if open {
			t.Fatal("subscription still open")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
	hub.Publish("project-a", Event{Type: DeviceOnline, DeviceID: "device-1"})
}
