package readings

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

const (
	defaultLimit  = 500
	maxLimit      = 5000
	defaultWindow = 24 * time.Hour
)

// HistoryQuery is a validated device history request.
// From and To are inclusive. Resolution is raw until aggregate buckets exist.
type HistoryQuery struct {
	Metric     string
	From       time.Time
	To         time.Time
	Limit      int
	Resolution string
}

// ParseHistory reads metric, from, to, limit, and resolution.
// Omitted from/to select the last 24 hours ending at now.
// Omitted limit is 500. 1m, 5m, 1h, and 1d are recognized and rejected until aggregation exists.
func ParseHistory(now time.Time, values url.Values) (HistoryQuery, map[string]string) {
	now = now.UTC()
	fields := map[string]string{}
	metric := strings.TrimSpace(values.Get("metric"))
	if metric == "" {
		fields["metric"] = "Metric is required"
	} else if !telemetry.ValidMetricKey(metric) {
		fields["metric"] = "Metric key is invalid"
	}

	from, fromOK := parseTime(values.Get("from"), now.Add(-defaultWindow))
	if !fromOK {
		fields["from"] = "From must be an RFC3339 timestamp"
	}
	to, toOK := parseTime(values.Get("to"), now)
	if !toOK {
		fields["to"] = "To must be an RFC3339 timestamp"
	}
	if fromOK && toOK && from.After(to) {
		fields["from"] = "From must be before to"
	}

	limit := defaultLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxLimit {
			fields["limit"] = "Limit must be from 1 to 5000"
		} else {
			limit = n
		}
	}

	resolution := strings.ToLower(strings.TrimSpace(values.Get("resolution")))
	if resolution == "" {
		resolution = "raw"
	}
	switch resolution {
	case "raw":
	case "1m", "5m", "1h", "1d":
		fields["resolution"] = "Only raw is supported"
	default:
		fields["resolution"] = "Resolution must be raw, 1m, 5m, 1h, or 1d"
	}

	if len(fields) > 0 {
		return HistoryQuery{}, fields
	}
	return HistoryQuery{
		Metric:     metric,
		From:       from,
		To:         to,
		Limit:      limit,
		Resolution: resolution,
	}, nil
}

func parseTime(raw string, fallback time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback.UTC(), true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, raw)
	}
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
