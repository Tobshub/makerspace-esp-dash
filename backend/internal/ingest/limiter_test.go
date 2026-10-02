package ingest

import (
	"testing"
	"time"
)

func TestLimiterBurst(t *testing.T) {
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	limit := newLimiter(2, func() time.Time { return now })
	if !limit.allow("dev") || !limit.allow("dev") {
		t.Fatal("burst")
	}
	if limit.allow("dev") {
		t.Fatal("third message should wait")
	}
	if !limit.allow("other") {
		t.Fatal("devices have separate buckets")
	}
	now = now.Add(500 * time.Millisecond)
	if !limit.allow("dev") {
		t.Fatal("expected a refill")
	}
}
