package commands

import "testing"

func TestParseAck(t *testing.T) {
	ack, err := ParseAck([]byte(`{"id":"cmd_123","success":true,"state":{"fan":true}}`))
	if err != nil || !ack.Success || ack.ID != "cmd_123" {
		t.Fatalf("ack %#v err %v", ack, err)
	}
	failed, err := ParseAck([]byte(`{"id":"cmd_123","success":false,"error":"invalid fan mode"}`))
	if err != nil || failed.Success || failed.Error != "invalid fan mode" {
		t.Fatalf("failed %#v err %v", failed, err)
	}
	if _, err := ParseAck([]byte(`{"id":"cmd_123"}`)); err == nil {
		t.Fatal("missing success")
	}
	if _, err := ParseAck([]byte(`{"success":true}`)); err == nil {
		t.Fatal("missing id")
	}
}

func TestValidateCommand(t *testing.T) {
	in, fields := validateInput("set_pump", []byte(`{"enabled":true}`))
	if len(fields) > 0 || in.Command != "set_pump" {
		t.Fatalf("in %#v fields %#v", in, fields)
	}
	if _, fields := validateInput("", []byte(`[]`)); fields["command"] == "" || fields["payload"] == "" {
		t.Fatalf("fields %#v", fields)
	}
}
