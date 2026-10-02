// Package ratelimit is a small in-memory fixed-window limiter. The API runs as a single
// instance on the free tier, so process memory is enough; swap for Postgres if it scales out.
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	count   int
	resetAt time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, buckets: map[string]*bucket{}, now: time.Now}
}

// Allow records one attempt for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.buckets) > 10_000 {
		l.sweep(now)
	}
	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		l.buckets[key] = &bucket{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	b.count++
	return b.count <= l.limit
}

func (l *Limiter) sweep(now time.Time) {
	for k, b := range l.buckets {
		if now.After(b.resetAt) {
			delete(l.buckets, k)
		}
	}
}
