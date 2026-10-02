package telemetry

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseScalars(t *testing.T) {
	received := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	packet, err := Parse([]byte(`{
		"metrics": {
			"temperature": 28.4,
			"humidity": 0,
			"pump_active": false,
			"mode": "automatic"
		}
	}`), DefaultLimits(), received)
	if err != nil {
		t.Fatal(err)
	}
	if !packet.RecordedAt.Equal(received) || len(packet.Points) != 4 {
		t.Fatalf("%+v", packet)
	}
	byKey := map[string]Point{}
	for _, point := range packet.Points {
		byKey[point.Key] = point
	}
	if byKey["temperature"].Numeric == nil || *byKey["temperature"].Numeric != 28.4 {
		t.Fatal("temperature")
	}
	if byKey["humidity"].Numeric == nil || *byKey["humidity"].Numeric != 0 {
		t.Fatal("humidity")
	}
	if byKey["pump_active"].Boolean == nil || *byKey["pump_active"].Boolean {
		t.Fatal("pump")
	}
	if byKey["mode"].Text == nil || *byKey["mode"].Text != "automatic" {
		t.Fatal("mode")
	}
}

func TestParseUsesDeviceTimestamp(t *testing.T) {
	received := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	packet, err := Parse([]byte(`{"timestamp":1700000000000,"metrics":{"temperature":1}}`), DefaultLimits(), received)
	if err != nil {
		t.Fatal(err)
	}
	if !packet.RecordedAt.Equal(time.UnixMilli(1700000000000).UTC()) {
		t.Fatal(packet.RecordedAt)
	}
	if packet.RecordedAt.Equal(packet.ReceivedAt) {
		t.Fatal("device time and receive time must stay distinct")
	}
}

func TestParseRejects(t *testing.T) {
	received := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	future := received.Add(25 * time.Hour).UnixMilli()
	cases := []struct {
		name    string
		payload string
		limits  Limits
		reason  string
	}{
		{name: "json", payload: `{`, reason: "invalid json"},
		{name: "trailing", payload: `{"metrics":{}} extra`, reason: "invalid json"},
		{name: "metrics", payload: `{"timestamp":1}`, reason: "metrics required"},
		{name: "nested", payload: `{"metrics":{"temperature":{"c":1}}}`, reason: "nested metric"},
		{name: "array", payload: `{"metrics":{"samples":[1,2]}}`, reason: "nested metric"},
		{name: "null", payload: `{"metrics":{"temperature":null}}`, reason: "unsupported metric"},
		{name: "inf", payload: `{"metrics":{"temperature":1e309}}`, reason: "non finite number"},
		{name: "string number", payload: `{"metrics":{"temperature":"28.4"}}`, reason: ""},
		{name: "key", payload: `{"metrics":{"soil-moisture":1}}`, reason: "metric key invalid"},
		{name: "count", payload: `{"metrics":{"a":1,"b":2}}`, limits: Limits{MaxMetrics: 1}, reason: "too many metrics"},
		{name: "long key", payload: `{"metrics":{"temperature":1}}`, limits: Limits{MaxKeyLength: 4}, reason: "metric key invalid"},
		{name: "long string", payload: `{"metrics":{"mode":"` + strings.Repeat("x", 20) + `"}}`, limits: Limits{MaxStringLength: 4}, reason: "metric value too long"},
		{name: "future", payload: `{"timestamp":` + strconv.FormatInt(future, 10) + `,"metrics":{}}`, reason: "invalid timestamp"},
		{name: "fraction", payload: `{"timestamp":1.5,"metrics":{}}`, reason: "invalid timestamp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limits := tc.limits
			if limits == (Limits{}) {
				limits = DefaultLimits()
			}
			_, err := Parse([]byte(tc.payload), limits, received)
			if tc.reason == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Error() != tc.reason {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestParseStateAndPresence(t *testing.T) {
	state, err := ParseState([]byte(`{"firmwareVersion":"1.0.3","wifiRssi":-61,"mode":"automatic","relay":true,"status":"idle"}`))
	if err != nil {
		t.Fatal(err)
	}
	if state.FirmwareVersion != "1.0.3" || state.Presence != "" {
		t.Fatalf("%+v", state)
	}
	offline, err := ParseState([]byte(`{"status":"offline","online":false}`))
	if err != nil || offline.Presence != "offline" {
		t.Fatalf("%s %+v", err, offline)
	}
	if _, err := ParseState([]byte(`{"status":"online","online":false}`)); err == nil || err.Error() != "conflicting presence" {
		t.Fatal(err)
	}
	online, err := ParsePresence([]byte(`{"online":true}`))
	if err != nil || !online {
		t.Fatal(err)
	}
	if _, err := ParsePresence([]byte(`{"status":"idle"}`)); err == nil {
		t.Fatal("expected explicit presence")
	}
	event, err := ParseDeviceEvent([]byte(`{"type":"disconnected","message":"lwt"}`))
	if err != nil || event.Type != "disconnected" {
		t.Fatal(err)
	}
	plain, err := ParseDeviceEvent([]byte(`{"message":"boot"}`))
	if err != nil || plain.Type != "event" {
		t.Fatal(err)
	}
}
