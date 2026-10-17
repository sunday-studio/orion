package api

import (
	"fmt"
	"sync"
	"sync/atomic"
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

func TestRateLimiterAllowCountsEveryAttemptWithinWindow(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if !limiter.Allow("203.0.113.9") {
			t.Fatalf("attempt %d rejected, want allowed", i)
		}
	}
	if limiter.Allow("203.0.113.9") {
		t.Fatal("4th attempt allowed, want rejected")
	}
	if !limiter.Allow("198.51.100.1") {
		t.Fatal("a different key must have its own budget")
	}
}

func TestRateLimiterAllowIsAtomicUnderConcurrency(t *testing.T) {
	limiter := NewRateLimiter(10, time.Minute)
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("same-ip") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("allowed = %d, want exactly the limit of 10", allowed.Load())
	}
}

func TestRateLimiterAllowDisabledAndEvictsExpired(t *testing.T) {
	if !NewRateLimiter(0, time.Minute).Allow("x") {
		t.Fatal("zero limit must disable limiting")
	}
	limiter := NewRateLimiter(1, time.Millisecond)
	for i := range rateLimiterEvictThreshold {
		limiter.Allow(fmt.Sprintf("ip-%d", i))
	}
	time.Sleep(5 * time.Millisecond)
	limiter.Allow("fresh")
	limiter.mu.Lock()
	size := len(limiter.failures)
	limiter.mu.Unlock()
	if size > 2 {
		t.Fatalf("limiter holds %d entries after expiry, want expired ones evicted", size)
	}
}
