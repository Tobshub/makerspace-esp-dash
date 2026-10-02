package alerts

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

const (
	RuleThreshold = "metric_threshold"
	RuleOffline   = "device_offline"

	maxDurationSeconds = 7 * 24 * 60 * 60
)

// Input is a create or update body.
type Input struct {
	Name            string
	RuleType        string
	DeviceID        string
	MetricKey       string
	Operator        string
	ThresholdValue  *float64
	DurationSeconds int
	Enabled         bool
	Configuration   json.RawMessage
}

func validate(in Input) (Input, map[string]string) {
	fields := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	in.MetricKey = strings.TrimSpace(in.MetricKey)
	in.Operator = strings.TrimSpace(in.Operator)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	if in.Name == "" || len(in.Name) > 80 {
		fields["name"] = "Name is required"
	}
	if in.DurationSeconds < 0 || in.DurationSeconds > maxDurationSeconds {
		fields["durationSeconds"] = "Duration must be from 0 to 604800 seconds"
	}
	if in.DeviceID != "" && !httpapi.ValidID(in.DeviceID) {
		fields["deviceId"] = "Choose a device in this project"
	}
	if len(in.Configuration) == 0 {
		in.Configuration = []byte(`{}`)
	}
	if !json.Valid(in.Configuration) || in.Configuration[0] != '{' {
		fields["configuration"] = "Configuration must be a JSON object"
	}
	switch in.RuleType {
	case RuleThreshold:
		if !telemetry.ValidMetricKey(in.MetricKey) {
			fields["metricKey"] = "Metric key must start with a letter and use letters, digits, or underscores"
		}
		if !validOperator(in.Operator) {
			fields["operator"] = "Operator must be >, >=, <, <=, ==, or !="
		}
		if in.ThresholdValue == nil || math.IsNaN(*in.ThresholdValue) || math.IsInf(*in.ThresholdValue, 0) {
			fields["thresholdValue"] = "Threshold must be a finite number"
		}
	case RuleOffline:
		in.MetricKey = ""
		in.Operator = ""
		in.ThresholdValue = nil
	default:
		fields["ruleType"] = "Rule type must be metric_threshold or device_offline"
	}
	if len(fields) > 0 {
		return Input{}, fields
	}
	return in, nil
}

func validOperator(op string) bool {
	switch op {
	case ">", ">=", "<", "<=", "==", "!=":
		return true
	default:
		return false
	}
}

// Matches reports whether value meets the threshold.
func Matches(operator string, value, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}
