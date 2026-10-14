package api

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	failures map[string]rateLimitEntry
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:    limit,
		window:   window,
		failures: make(map[string]rateLimitEntry),
	}
}

func (l *RateLimiter) TooManyFailures(key string) bool {
	if l.limit <= 0 || l.window <= 0 {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.failures[key]
	if !exists || time.Now().After(entry.resetAt) {
		return false
	}
	return entry.count >= l.limit
}

func (l *RateLimiter) RecordFailure(key string) {
	if l.limit <= 0 || l.window <= 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, exists := l.failures[key]
	if !exists || now.After(entry.resetAt) {
		entry = rateLimitEntry{resetAt: now.Add(l.window)}
	}
	entry.count++
	l.failures[key] = entry
}

func (l *RateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// Allow counts one attempt for key and reports whether it is within the limit.
// The check and the increment happen under one lock, so concurrent callers cannot
// slip past the limit. A limit or window of zero disables limiting. Expired
// entries are evicted opportunistically so the map cannot grow without bound.
func (l *RateLimiter) Allow(key string) bool {
	if l.limit <= 0 || l.window <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if len(l.failures) >= rateLimiterEvictThreshold {
		for k, e := range l.failures {
			if now.After(e.resetAt) {
				delete(l.failures, k)
			}
		}
	}
	entry, exists := l.failures[key]
	if !exists || now.After(entry.resetAt) {
		entry = rateLimitEntry{resetAt: now.Add(l.window)}
	}
	entry.count++
	l.failures[key] = entry
	return entry.count <= l.limit
}

const rateLimiterEvictThreshold = 1024
