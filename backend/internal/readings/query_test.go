package readings

import (
	"net/url"
	"testing"
	"time"
)

func TestParseHistoryDefaults(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	query, fields := ParseHistory(now, url.Values{"metric": {"temperature"}})
	if len(fields) > 0 {
		t.Fatal(fields)
	}
	if query.Resolution != "raw" || query.Limit != defaultLimit {
		t.Fatalf("query = %+v", query)
	}
	if !query.From.Equal(now.Add(-defaultWindow)) || !query.To.Equal(now) {
		t.Fatalf("window %s .. %s", query.From, query.To)
	}
}

func TestParseHistoryRejectsBadInput(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		query url.Values
		field string
	}{
		{name: "missing metric", query: url.Values{}, field: "metric"},
		{name: "bad metric", query: url.Values{"metric": {"1temp"}}, field: "metric"},
		{name: "bad from", query: url.Values{"metric": {"temperature"}, "from": {"yesterday"}}, field: "from"},
		{name: "from after to", query: url.Values{"metric": {"temperature"}, "from": {"2026-10-03T00:00:00Z"}, "to": {"2026-10-02T00:00:00Z"}}, field: "from"},
		{name: "limit", query: url.Values{"metric": {"temperature"}, "limit": {"0"}}, field: "limit"},
		{name: "future resolution", query: url.Values{"metric": {"temperature"}, "resolution": {"1h"}}, field: "resolution"},
		{name: "unknown resolution", query: url.Values{"metric": {"temperature"}, "resolution": {"2m"}}, field: "resolution"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, fields := ParseHistory(now, tc.query)
			if fields[tc.field] == "" {
				t.Fatalf("fields = %#v", fields)
			}
		})
	}
}

func TestParseHistoryAcceptsRawRange(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	query, fields := ParseHistory(now, url.Values{
		"metric":     {"soil_moisture"},
		"from":       {"2026-10-01T00:00:00Z"},
		"to":         {"2026-10-02T00:00:00.5Z"},
		"limit":      {"10"},
		"resolution": {"RAW"},
	})
	if len(fields) > 0 {
		t.Fatal(fields)
	}
	if query.Metric != "soil_moisture" || query.Limit != 10 || query.Resolution != "raw" {
		t.Fatalf("query = %+v", query)
	}
	if query.From.Format(time.RFC3339) != "2026-10-01T00:00:00Z" {
		t.Fatal(query.From)
	}
}
