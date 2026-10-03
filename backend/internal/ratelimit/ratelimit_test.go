package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterWindow(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	l := New(2, time.Minute)
	l.now = func() time.Time { return now }

	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("first two attempts must pass")
	}
	if l.Allow("k") {
		t.Fatal("third attempt must be limited")
	}
	if !l.Allow("other") {
		t.Fatal("keys are independent")
	}
	now = now.Add(61 * time.Second)
	if !l.Allow("k") {
		t.Fatal("window must reset")
	}
}
