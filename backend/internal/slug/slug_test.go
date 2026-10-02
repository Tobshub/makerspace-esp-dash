package slug

import "testing"

func TestFromName(t *testing.T) {
	cases := map[string]string{
		"Smart Greenhouse":       "smart-greenhouse",
		"  Greenhouse Monitor  ": "greenhouse-monitor",
		"---":                    "item",
		"Soil moisture #1":       "soil-moisture-1",
	}
	for in, want := range cases {
		if got := FromName(in); got != want {
			t.Fatalf("FromName(%q) = %q, want %q", in, got, want)
		}
	}
}
