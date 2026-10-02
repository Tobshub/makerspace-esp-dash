package telemetry

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"time"
)

// Limits bound a telemetry packet before it is stored.
type Limits struct {
	MaxMetrics      int
	MaxKeyLength    int
	MaxStringLength int
}

// DefaultLimits are the MVP caps: 50 scalar metrics, 64-character keys, 1024-character strings.
func DefaultLimits() Limits {
	return Limits{MaxMetrics: 50, MaxKeyLength: 64, MaxStringLength: 1024}
}

func (l Limits) normalized() Limits {
	d := DefaultLimits()
	if l.MaxMetrics <= 0 {
		l.MaxMetrics = d.MaxMetrics
	}
	if l.MaxKeyLength <= 0 {
		l.MaxKeyLength = d.MaxKeyLength
	}
	if l.MaxStringLength <= 0 {
		l.MaxStringLength = d.MaxStringLength
	}
	return l
}

// Error is a rejected payload. Reason is safe to store and log.
type Error struct {
	Reason string
}

func (e *Error) Error() string { return e.Reason }

// Point is one scalar metric. Exactly one value pointer is set.
type Point struct {
	Key     string
	Numeric *float64
	Boolean *bool
	Text    *string
}

// Packet is a validated telemetry publish.
type Packet struct {
	RecordedAt time.Time
	ReceivedAt time.Time
	Points     []Point
}

// Parse reads a telemetry packet. Timestamp is Unix milliseconds and optional.
// Omitted timestamps use received. Nested objects and arrays are rejected.
// Metric keys do not need a metric definition.
func Parse(payload []byte, limits Limits, received time.Time) (Packet, error) {
	limits = limits.normalized()
	received = received.UTC()
	obj, err := decodeObject(payload)
	if err != nil {
		return Packet{}, err
	}
	rawMetrics, ok := obj["metrics"]
	if !ok || len(bytes.TrimSpace(rawMetrics)) == 0 || bytes.TrimSpace(rawMetrics)[0] != '{' {
		return Packet{}, &Error{Reason: "metrics required"}
	}
	metrics, err := decodeObject(rawMetrics)
	if err != nil {
		return Packet{}, &Error{Reason: "metrics required"}
	}
	if len(metrics) > limits.MaxMetrics {
		return Packet{}, &Error{Reason: "too many metrics"}
	}
	recorded := received
	if raw, ok := obj["timestamp"]; ok {
		recorded, err = parseTimestamp(raw, received)
		if err != nil {
			return Packet{}, err
		}
	}
	points := make([]Point, 0, len(metrics))
	for key, raw := range metrics {
		if !validKey(key, limits.MaxKeyLength) {
			return Packet{}, &Error{Reason: "metric key invalid"}
		}
		point, err := classify(raw, limits.MaxStringLength)
		if err != nil {
			return Packet{}, err
		}
		point.Key = key
		points = append(points, point)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Key < points[j].Key })
	return Packet{RecordedAt: recorded, ReceivedAt: received, Points: points}, nil
}

func decodeObject(payload []byte) (map[string]json.RawMessage, error) {
	trim := bytes.TrimSpace(payload)
	if len(trim) == 0 || trim[0] != '{' {
		return nil, &Error{Reason: "invalid json"}
	}
	dec := json.NewDecoder(bytes.NewReader(trim))
	dec.UseNumber()
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return nil, &Error{Reason: "invalid json"}
	}
	if dec.More() {
		return nil, &Error{Reason: "invalid json"}
	}
	return obj, nil
}

func parseTimestamp(raw json.RawMessage, received time.Time) (time.Time, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	dec.UseNumber()
	var num json.Number
	if err := dec.Decode(&num); err != nil || dec.More() {
		return time.Time{}, &Error{Reason: "invalid timestamp"}
	}
	f, err := num.Float64()
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f != math.Trunc(f) || f > float64(math.MaxInt64) {
		return time.Time{}, &Error{Reason: "invalid timestamp"}
	}
	recorded := time.UnixMilli(int64(f)).UTC()
	if recorded.After(received.Add(24 * time.Hour)) {
		return time.Time{}, &Error{Reason: "invalid timestamp"}
	}
	return recorded, nil
}

// ValidMetricKey reports whether key can be stored and queried.
// Keys start with a letter and then use letters, digits, or underscores, up to 64 characters.
func ValidMetricKey(key string) bool {
	return validKey(key, DefaultLimits().MaxKeyLength)
}

func validKey(key string, max int) bool {
	if key == "" || len(key) > max {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if i == 0 {
			if !isLetter(c) {
				return false
			}
			continue
		}
		if !isLetter(c) && !isDigit(c) && c != '_' {
			return false
		}
	}
	return true
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

func classify(raw json.RawMessage, maxString int) (Point, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		return Point{}, &Error{Reason: "unsupported metric"}
	}
	switch trim[0] {
	case '{', '[':
		return Point{}, &Error{Reason: "nested metric"}
	case '"':
		var text string
		if err := json.Unmarshal(trim, &text); err != nil {
			return Point{}, &Error{Reason: "unsupported metric"}
		}
		if len(text) > maxString {
			return Point{}, &Error{Reason: "metric value too long"}
		}
		return Point{Text: &text}, nil
	case 't', 'f':
		var value bool
		if err := json.Unmarshal(trim, &value); err != nil {
			return Point{}, &Error{Reason: "unsupported metric"}
		}
		return Point{Boolean: &value}, nil
	default:
		dec := json.NewDecoder(bytes.NewReader(trim))
		dec.UseNumber()
		var num json.Number
		if err := dec.Decode(&num); err != nil || dec.More() {
			return Point{}, &Error{Reason: "unsupported metric"}
		}
		value, err := num.Float64()
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return Point{}, &Error{Reason: "non finite number"}
		}
		return Point{Numeric: &value}, nil
	}
}
