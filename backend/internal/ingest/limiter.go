package ingest

import (
	"sync"
	"time"
)

// limiter is an in-process token bucket. One API process enforces the cap.
// A second replica would allow the same rate again.
type limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	now     func() time.Time
	buckets map[string]*bucket
}

type bucket struct {
	tokens  float64
	updated time.Time
}

func newLimiter(perSecond int, now func() time.Time) *limiter {
	if perSecond <= 0 {
		perSecond = 10
	}
	if now == nil {
		now = time.Now
	}
	return &limiter{
		rate:    float64(perSecond),
		burst:   float64(perSecond),
		now:     now,
		buckets: map[string]*bucket{},
	}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.buckets) > 4096 {
		cutoff := now.Add(-2 * time.Minute)
		for id, item := range l.buckets {
			if item.updated.Before(cutoff) {
				delete(l.buckets, id)
			}
		}
	}
	item := l.buckets[key]
	if item == nil {
		item = &bucket{tokens: l.burst, updated: now}
		l.buckets[key] = item
	} else if elapsed := now.Sub(item.updated).Seconds(); elapsed > 0 {
		item.tokens += elapsed * l.rate
		if item.tokens > l.burst {
			item.tokens = l.burst
		}
		item.updated = now
	}
	if item.tokens < 1 {
		return false
	}
	item.tokens--
	return true
}
