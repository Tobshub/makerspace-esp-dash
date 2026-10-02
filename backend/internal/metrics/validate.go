package metrics

import (
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

const (
	DataNumber  = "number"
	DataBoolean = "boolean"
	DataString  = "string"

	DisplayNumber  = "number"
	DisplayLine    = "line"
	DisplayGauge   = "gauge"
	DisplayBoolean = "boolean"
	DisplayStatus  = "status"
	DisplayText    = "text"
)

func validate(in Input) map[string]string {
	fields := map[string]string{}
	if in.checkKey && !telemetry.ValidMetricKey(in.Key) {
		fields["key"] = "Key must start with a letter and use letters, digits, or underscores"
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 80 {
		fields["name"] = "Display name is required"
	}
	if len(in.Description) > 2000 {
		fields["description"] = "Description is too long"
	}
	switch in.DataType {
	case DataNumber, DataBoolean, DataString:
	default:
		fields["dataType"] = "Data type must be number, boolean, or string"
	}
	if utf8.RuneCountInString(in.Unit) > 32 {
		fields["unit"] = "Unit is too long"
	}
	if !displayAllowed(in.DataType, in.DisplayType) {
		fields["displayType"] = "Choose a display that matches the data type"
	}
	if badNumber(in.MinValue) {
		fields["minValue"] = "Min must be a finite number"
	}
	if badNumber(in.MaxValue) {
		fields["maxValue"] = "Max must be a finite number"
	}
	if in.DataType != DataNumber && (in.MinValue != nil || in.MaxValue != nil) {
		fields["minValue"] = "Min and max apply to numbers"
	}
	if in.DataType == DataNumber && in.MinValue != nil && in.MaxValue != nil && *in.MinValue >= *in.MaxValue {
		fields["maxValue"] = "Max must be greater than min"
	}
	if in.DisplayType == DisplayGauge && (in.MinValue == nil || in.MaxValue == nil) && fields["minValue"] == "" {
		fields["minValue"] = "A gauge needs a min and a max"
	}
	metadata := in.Metadata
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	if !json.Valid(metadata) || strings.TrimSpace(string(metadata))[0] != '{' {
		fields["metadata"] = "Metadata must be a JSON object"
	} else if len(metadata) > 8192 {
		fields["metadata"] = "Metadata is too large"
	}
	return fields
}

func displayAllowed(dataType, displayType string) bool {
	switch dataType {
	case DataNumber:
		return displayType == DisplayNumber || displayType == DisplayLine || displayType == DisplayGauge
	case DataBoolean:
		return displayType == DisplayBoolean || displayType == DisplayStatus
	case DataString:
		return displayType == DisplayText
	default:
		return false
	}
}

func badNumber(v *float64) bool {
	return v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0))
}

func normalize(in Input) Input {
	in.Key = strings.TrimSpace(in.Key)
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Unit = strings.TrimSpace(in.Unit)
	if len(in.Metadata) == 0 {
		in.Metadata = []byte(`{}`)
	}
	if in.DataType != DataNumber {
		in.MinValue = nil
		in.MaxValue = nil
	}
	return in
}
