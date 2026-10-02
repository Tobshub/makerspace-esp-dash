package controls

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

const (
	TypeButton = "button"
	TypeToggle = "toggle"
	TypeSlider = "slider"
)

// Input is a create or update body.
type Input struct {
	Key           string
	Name          string
	ControlType   string
	Command       string
	Configuration json.RawMessage
	checkKey      bool
}

func validate(in Input) (Input, map[string]string) {
	fields := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	in.Command = strings.TrimSpace(in.Command)
	if in.checkKey && !telemetry.ValidMetricKey(in.Key) {
		fields["key"] = "Key must start with a letter and use letters, digits, or underscores"
	}
	if in.Name == "" || len(in.Name) > 80 {
		fields["name"] = "Name is required"
	}
	if !telemetry.ValidMetricKey(in.Command) {
		fields["command"] = "Command must start with a letter and use letters, digits, or underscores"
	}
	config, configFields := normalizeConfig(in.ControlType, in.Configuration)
	for key, value := range configFields {
		fields[key] = value
	}
	if len(fields) > 0 {
		return Input{}, fields
	}
	in.Configuration = config
	return in, nil
}

func normalizeConfig(controlType string, raw json.RawMessage) (json.RawMessage, map[string]string) {
	fields := map[string]string{}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	if !json.Valid(raw) || raw[0] != '{' {
		return nil, map[string]string{"configuration": "Configuration must be a JSON object"}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, map[string]string{"configuration": "Configuration must be a JSON object"}
	}
	switch controlType {
	case TypeButton:
		payload := obj["payload"]
		if len(payload) == 0 {
			payload = []byte(`{}`)
		}
		if !jsonObject(payload) {
			fields["configuration"] = "Button payload must be a JSON object"
			break
		}
		return mustObject(map[string]any{"payload": json.RawMessage(payload)}), fields
	case TypeToggle:
		on := obj["onPayload"]
		off := obj["offPayload"]
		if !jsonObject(on) || !jsonObject(off) {
			fields["configuration"] = "Toggle needs onPayload and offPayload objects"
			break
		}
		return mustObject(map[string]any{
			"onPayload":  json.RawMessage(on),
			"offPayload": json.RawMessage(off),
		}), fields
	case TypeSlider:
		min, minOK := number(obj["min"])
		max, maxOK := number(obj["max"])
		step, stepOK := number(obj["step"])
		var key string
		if err := json.Unmarshal(obj["payloadKey"], &key); err != nil || !telemetry.ValidMetricKey(key) {
			fields["configuration"] = "Slider needs a payload key"
		}
		if !minOK || !maxOK || !stepOK || min >= max || step <= 0 || step > max-min {
			fields["configuration"] = "Slider needs min, max, and a positive step inside the range"
		}
		if len(fields) > 0 {
			break
		}
		return mustObject(map[string]any{"min": min, "max": max, "step": step, "payloadKey": key}), fields
	default:
		fields["controlType"] = "Control type must be button, toggle, or slider"
	}
	return nil, fields
}

func jsonObject(raw json.RawMessage) bool {
	return json.Valid(raw) && len(raw) > 0 && raw[0] == '{'
}

func number(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func mustObject(value map[string]any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}
