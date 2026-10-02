package controls

import "testing"

func TestValidateControls(t *testing.T) {
	button, fields := validate(Input{
		Key: "open_gate", Name: "Open gate", ControlType: TypeButton, Command: "open_gate", checkKey: true,
		Configuration: []byte(`{}`),
	})
	if len(fields) > 0 || string(button.Configuration) == "" {
		t.Fatalf("button %#v fields %#v", button, fields)
	}
	_, fields = validate(Input{
		Key: "fan", Name: "Fan", ControlType: TypeToggle, Command: "set_fan", checkKey: true,
		Configuration: []byte(`{"onPayload":{"enabled":true},"offPayload":{"enabled":false}}`),
	})
	if len(fields) > 0 {
		t.Fatalf("toggle %#v", fields)
	}
	_, fields = validate(Input{
		Key: "speed", Name: "Speed", ControlType: TypeSlider, Command: "set_speed", checkKey: true,
		Configuration: []byte(`{"min":0,"max":100,"step":5,"payloadKey":"speed"}`),
	})
	if len(fields) > 0 {
		t.Fatalf("slider %#v", fields)
	}
	if _, fields = validate(Input{
		Key: "speed", Name: "Speed", ControlType: TypeSlider, Command: "set_speed", checkKey: true,
		Configuration: []byte(`{"min":10,"max":0,"step":1,"payloadKey":"speed"}`),
	}); fields["configuration"] == "" {
		t.Fatal("expected slider range error")
	}
}
