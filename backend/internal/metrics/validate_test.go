package metrics

import "testing"

func TestValidateDisplayTypes(t *testing.T) {
	min := 0.0
	max := 100.0
	cases := []struct {
		name string
		in   Input
		bad  string
	}{
		{name: "line", in: Input{checkKey: true, Key: "temperature", Name: "Temperature", DataType: DataNumber, DisplayType: DisplayLine}},
		{name: "gauge", in: Input{checkKey: true, Key: "soil_moisture", Name: "Soil", DataType: DataNumber, DisplayType: DisplayGauge, MinValue: &min, MaxValue: &max}},
		{name: "boolean", in: Input{checkKey: true, Key: "pump_active", Name: "Pump", DataType: DataBoolean, DisplayType: DisplayBoolean}},
		{name: "text", in: Input{checkKey: true, Key: "mode", Name: "Mode", DataType: DataString, DisplayType: DisplayText}},
		{name: "gauge missing range", in: Input{checkKey: true, Key: "soil_moisture", Name: "Soil", DataType: DataNumber, DisplayType: DisplayGauge}, bad: "minValue"},
		{name: "line on boolean", in: Input{checkKey: true, Key: "pump_active", Name: "Pump", DataType: DataBoolean, DisplayType: DisplayLine}, bad: "displayType"},
		{name: "bad key", in: Input{checkKey: true, Key: "1temp", Name: "Temperature", DataType: DataNumber, DisplayType: DisplayNumber}, bad: "key"},
		{name: "key skipped on update", in: Input{Key: "1temp", Name: "Temperature", DataType: DataNumber, DisplayType: DisplayNumber}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := validate(tc.in)
			if tc.bad == "" {
				if len(fields) != 0 {
					t.Fatalf("fields %#v", fields)
				}
				return
			}
			if fields[tc.bad] == "" {
				t.Fatalf("missing %s in %#v", tc.bad, fields)
			}
		})
	}
}
