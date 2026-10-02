package alerts

import "testing"

func TestMatches(t *testing.T) {
	if !Matches(">", 41, 40) || Matches(">", 40, 40) {
		t.Fatal("greater than")
	}
	if !Matches(">=", 40, 40) || !Matches("<", 1, 2) || !Matches("<=", 2, 2) {
		t.Fatal("inclusive")
	}
	if !Matches("==", 3, 3) || !Matches("!=", 3, 4) || Matches("!=", 3, 3) {
		t.Fatal("equality")
	}
	if Matches("nope", 1, 0) {
		t.Fatal("unknown operator")
	}
}

func TestValidateRule(t *testing.T) {
	in, fields := validate(Input{
		Name: "Hot", RuleType: RuleThreshold, MetricKey: "temperature", Operator: ">",
		ThresholdValue: floatPtr(40), DurationSeconds: 30, Enabled: true,
	})
	if len(fields) > 0 || in.Name != "Hot" {
		t.Fatalf("in %#v fields %#v", in, fields)
	}
	if _, fields = validate(Input{Name: "Down", RuleType: RuleOffline, DurationSeconds: 300, Enabled: true}); len(fields) > 0 {
		t.Fatalf("offline %#v", fields)
	}
	if _, fields = validate(Input{Name: "Hot", RuleType: RuleThreshold, MetricKey: "temperature", Operator: "eq"}); fields["operator"] == "" || fields["thresholdValue"] == "" {
		t.Fatalf("fields %#v", fields)
	}
}

func floatPtr(v float64) *float64 { return &v }
