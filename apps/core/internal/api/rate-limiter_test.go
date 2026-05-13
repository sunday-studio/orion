package api

import (
	"testing"
	"time"
)

func TestRateLimiterTracksFailuresAndReset(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	key := "127.0.0.1"

	if limiter.TooManyFailures(key) {
		t.Fatal("TooManyFailures() = true before failures")
	}
	limiter.RecordFailure(key)
	limiter.RecordFailure(key)
	if !limiter.TooManyFailures(key) {
		t.Fatal("TooManyFailures() = false after limit reached")
	}
	limiter.Reset(key)
	if limiter.TooManyFailures(key) {
		t.Fatal("TooManyFailures() = true after reset")
	}
}
