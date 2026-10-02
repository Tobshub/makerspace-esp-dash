package main

import (
	"encoding/json"
	"math"
	"sync"
	"time"
)

const ackCacheLimit = 64

// reading is one greenhouse sample. Temperature and humidity follow the clock.
// Soil moisture changes only when the simulator advances one interval.
type reading struct {
	Temperature  float64
	Humidity     float64
	SoilMoisture float64
	PumpActive   bool
}

// commandResult is the acknowledgement for one command id.
// Changed is true when set_pump altered the pump. A duplicate id is not applied again.
type commandResult struct {
	Ack     []byte
	ID      string
	Command string
	Success bool
	Changed bool
	Error   string
}

// Greenhouse is the in-memory plant bed the simulator publishes.
type Greenhouse struct {
	mu      sync.Mutex
	started time.Time
	soil    float64
	pump    bool
	acks    map[string]cachedAck
	order   []string
}

type cachedAck struct {
	raw     []byte
	command string
	success bool
	reason  string
}

func NewGreenhouse(started time.Time) *Greenhouse {
	return &Greenhouse{
		started: started,
		soil:    55,
		acks:    map[string]cachedAck{},
	}
}

// Reading is the current sample without moving soil moisture.
func (g *Greenhouse) Reading(now time.Time) reading {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.readingLocked(now)
}

// Advance moves soil moisture by one interval, then returns the sample.
// The pump raises moisture. While it is off, moisture falls.
func (g *Greenhouse) Advance(now time.Time) reading {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pump {
		g.soil += 1.2
	} else {
		g.soil -= 0.4
	}
	g.soil = clamp(g.soil, 8, 95)
	return g.readingLocked(now)
}

func (g *Greenhouse) readingLocked(now time.Time) reading {
	elapsed := now.Sub(g.started).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	temp := round1(24 + 4*math.Sin(elapsed/90))
	humidity := round1(clamp(68-(temp-24)*1.5, 35, 95))
	return reading{
		Temperature:  temp,
		Humidity:     humidity,
		SoilMoisture: round1(g.soil),
		PumpActive:   g.pump,
	}
}

// HandleCommand applies a command payload and returns the acknowledgement bytes.
// The second return is false when the payload has no command id to acknowledge.
// A repeated id returns the original acknowledgement and does not change the pump.
func (g *Greenhouse) HandleCommand(payload []byte) (commandResult, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	var cmd struct {
		ID      string          `json:"id"`
		Command string          `json:"command"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(payload, &cmd); err != nil || cmd.ID == "" || len(cmd.ID) > 128 {
		return commandResult{}, false
	}
	if prev, ok := g.acks[cmd.ID]; ok {
		return commandResult{
			Ack:     append([]byte(nil), prev.raw...),
			ID:      cmd.ID,
			Command: prev.command,
			Success: prev.success,
			Error:   prev.reason,
		}, true
	}

	result := commandResult{ID: cmd.ID, Command: cmd.Command}
	ack := ackBody{ID: cmd.ID}
	switch cmd.Command {
	case "set_pump":
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if len(cmd.Payload) == 0 || json.Unmarshal(cmd.Payload, &body) != nil || body.Enabled == nil {
			ack.Error = "enabled required"
			result.Error = ack.Error
		} else {
			ack.Success = true
			ack.State = &pumpState{PumpActive: *body.Enabled}
			result.Success = true
			result.Changed = g.pump != *body.Enabled
			g.pump = *body.Enabled
		}
	default:
		ack.Error = "unknown command"
		result.Error = ack.Error
	}
	raw, err := json.Marshal(ack)
	if err != nil {
		return commandResult{}, false
	}
	g.remember(cmd.ID, cachedAck{
		raw:     raw,
		command: cmd.Command,
		success: result.Success,
		reason:  result.Error,
	})
	result.Ack = raw
	return result, true
}

func (g *Greenhouse) remember(id string, ack cachedAck) {
	ack.raw = append([]byte(nil), ack.raw...)
	g.acks[id] = ack
	g.order = append(g.order, id)
	if len(g.order) <= ackCacheLimit {
		return
	}
	drop := g.order[0]
	g.order = g.order[1:]
	delete(g.acks, drop)
}

type ackBody struct {
	ID      string     `json:"id"`
	Success bool       `json:"success"`
	State   *pumpState `json:"state,omitempty"`
	Error   string     `json:"error,omitempty"`
}

type pumpState struct {
	PumpActive bool `json:"pump_active"`
}

func telemetryPayload(now time.Time, sample reading) ([]byte, error) {
	return json.Marshal(struct {
		Timestamp int64          `json:"timestamp"`
		Metrics   map[string]any `json:"metrics"`
	}{
		Timestamp: now.UnixMilli(),
		Metrics: map[string]any{
			"temperature":   sample.Temperature,
			"humidity":      sample.Humidity,
			"soil_moisture": sample.SoilMoisture,
			"pump_active":   sample.PumpActive,
		},
	})
}

func statePayload(firmware string, sample reading) ([]byte, error) {
	return json.Marshal(struct {
		FirmwareVersion string `json:"firmwareVersion"`
		Mode            string `json:"mode"`
		PumpActive      bool   `json:"pump_active"`
	}{
		FirmwareVersion: firmware,
		Mode:            "automatic",
		PumpActive:      sample.PumpActive,
	})
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
