package ingest

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
)

type capture struct {
	mu     sync.Mutex
	events []realtime.Event
}

func (c *capture) Publish(_ string, event realtime.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *capture) types() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.events))
	for i, event := range c.events {
		out[i] = event.Type
	}
	return out
}

func TestIngestPublishesRealtime(t *testing.T) {
	pool := openPool(t)
	defer pool.Close()
	if err := database.Migrate(poolURL(t)); err != nil {
		t.Fatal(err)
	}
	projectID, cleanup := seedProject(t, pool)
	t.Cleanup(cleanup)
	hash, err := bcrypt.GenerateFromPassword([]byte("test-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
	fixed := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	svc := New(pool, Options{Now: func() time.Time { return fixed }, TelemetryPerSecond: 10})
	got := &capture{}
	svc.PublishTo(got)

	ctx := context.Background()
	body := []byte(`{"metrics":{"temperature":28.4,"pump_active":true}}`)
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "telemetry"), body); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "telemetry"), body); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "state"), []byte(`{"firmwareVersion":"sim-1.0.0","pump":false}`)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "commands/ack"), []byte(`{"id":"cmd_1","success":true}`)); err != nil {
		t.Fatal(err)
	}

	types := got.types()
	want := []string{
		realtime.DeviceOnline,
		realtime.TelemetryReceived,
		realtime.TelemetryReceived,
		realtime.StateUpdated,
		realtime.CommandUpdated,
	}
	if len(types) != len(want) {
		t.Fatalf("events %v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events %v", types)
		}
	}
	got.mu.Lock()
	data, ok := got.events[1].Data.(map[string]any)
	got.mu.Unlock()
	if !ok || data["temperature"] != 28.4 || got.events[1].DeviceID != deviceID {
		t.Fatalf("telemetry %#v", got.events[1])
	}

	seen := fixed.Add(-2 * time.Minute)
	staleID, _ := insertDevice(t, pool, projectID, string(hash), "online", &seen)
	if _, err := svc.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	foundOffline := false
	got.mu.Lock()
	for _, event := range got.events {
		if event.Type == realtime.DeviceOffline && event.DeviceID == staleID {
			foundOffline = true
		}
	}
	got.mu.Unlock()
	if !foundOffline {
		t.Fatalf("missing offline for %s in %v", staleID, got.types())
	}
}
