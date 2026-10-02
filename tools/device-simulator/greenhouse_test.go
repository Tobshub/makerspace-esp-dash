package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestGreenhouseDrift(t *testing.T) {
	started := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	bed := NewGreenhouse(started)

	first := bed.Reading(started)
	if first.Temperature != 24 || first.Humidity != 68 || first.SoilMoisture != 55 || first.PumpActive {
		t.Fatalf("start sample = %+v", first)
	}

	var quarterTurn float64 = 45 * math.Pi
	peakAt := time.Duration(quarterTurn * float64(time.Second))
	peak := bed.Reading(started.Add(peakAt))
	if peak.Temperature != 28 || peak.Humidity != 62 {
		t.Fatalf("peak sample = %+v", peak)
	}

	dry := bed.Advance(started)
	if dry.SoilMoisture != 54.6 || dry.PumpActive {
		t.Fatalf("dry soil = %+v", dry)
	}
}

func TestSetPumpAndDuplicate(t *testing.T) {
	bed := NewGreenhouse(time.Now())
	first, ok := bed.HandleCommand([]byte(`{"id":"cmd_1","command":"set_pump","payload":{"enabled":true}}`))
	if !ok || !first.Success || !first.Changed {
		t.Fatalf("first = %+v ok=%v", first, ok)
	}
	var ack ackBody
	if err := json.Unmarshal(first.Ack, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.ID != "cmd_1" || !ack.Success || ack.State == nil || !ack.State.PumpActive {
		t.Fatalf("ack = %+v", ack)
	}

	replay, ok := bed.HandleCommand([]byte(`{"id":"cmd_1","command":"set_pump","payload":{"enabled":false}}`))
	if !ok || !replay.Success || replay.Changed || string(replay.Ack) != string(first.Ack) {
		t.Fatalf("replay = %+v", replay)
	}
	if sample := bed.Reading(time.Now()); !sample.PumpActive {
		t.Fatal("duplicate command changed the pump")
	}

	wet := bed.Advance(time.Now())
	if wet.SoilMoisture != 56.2 || !wet.PumpActive {
		t.Fatalf("wet soil = %+v", wet)
	}
}

func TestCommandFailures(t *testing.T) {
	bed := NewGreenhouse(time.Now())
	cases := []struct {
		name    string
		payload string
		ack     bool
		reason  string
	}{
		{name: "junk", payload: `{`, ack: false},
		{name: "missing id", payload: `{"command":"set_pump","payload":{"enabled":true}}`, ack: false},
		{name: "missing enabled", payload: `{"id":"cmd_2","command":"set_pump","payload":{}}`, ack: true, reason: "enabled required"},
		{name: "unknown", payload: `{"id":"cmd_3","command":"set_fan","payload":{"enabled":true}}`, ack: true, reason: "unknown command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := bed.HandleCommand([]byte(tc.payload))
			if ok != tc.ack {
				t.Fatalf("ok=%v", ok)
			}
			if !ok {
				return
			}
			if result.Success || result.Changed || result.Error != tc.reason {
				t.Fatalf("result = %+v", result)
			}
		})
	}
	if sample := bed.Reading(time.Now()); sample.PumpActive {
		t.Fatal("failed commands turned the pump on")
	}
}

func TestCommandCacheEvictsOldest(t *testing.T) {
	bed := NewGreenhouse(time.Now())
	for i := 0; i < ackCacheLimit; i++ {
		payload := []byte(`{"id":"cmd_` + itoa(i) + `","command":"set_pump","payload":{"enabled":true}}`)
		if _, ok := bed.HandleCommand(payload); !ok {
			t.Fatalf("command %d ignored", i)
		}
	}
	replay, ok := bed.HandleCommand([]byte(`{"id":"cmd_0","command":"set_pump","payload":{"enabled":false}}`))
	if !ok || !replay.Success || replay.Changed {
		t.Fatalf("cached replay = %+v", replay)
	}
	extra := []byte(`{"id":"cmd_extra","command":"set_pump","payload":{"enabled":true}}`)
	if _, ok := bed.HandleCommand(extra); !ok {
		t.Fatal("extra command ignored")
	}
	applied, ok := bed.HandleCommand([]byte(`{"id":"cmd_0","command":"set_pump","payload":{"enabled":false}}`))
	if !ok || !applied.Success || !applied.Changed {
		t.Fatalf("evicted replay = %+v", applied)
	}
	if sample := bed.Reading(time.Now()); sample.PumpActive {
		t.Fatal("pump still on after evicted command")
	}
}

func TestTelemetryPayload(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()
	raw, err := telemetryPayload(now, reading{Temperature: 24, Humidity: 68, SoilMoisture: 55, PumpActive: false})
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Timestamp int64          `json:"timestamp"`
		Metrics   map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Timestamp != now.UnixMilli() || len(packet.Metrics) != 4 {
		t.Fatalf("packet = %+v", packet)
	}
	state, err := statePayload("sim-1.0.0", reading{PumpActive: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(state, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["firmwareVersion"] != "sim-1.0.0" || doc["pump_active"] != true {
		t.Fatalf("state = %+v", doc)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [8]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
