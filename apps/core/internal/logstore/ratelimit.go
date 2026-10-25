package logstore

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// AppLimiter is a per-app token bucket measured in entries: rate per second, burst of twice that.
type AppLimiter struct {
	limitedAt atomic.Int64 // unix nano of the last refused request, 0 = never
	mu        sync.Mutex
	rate      float64
	buckets   map[string]*bucket
	now       func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewAppLimiter(perSecond int) *AppLimiter {
	return &AppLimiter{rate: float64(perSecond), buckets: map[string]*bucket{}, now: time.Now}
}

// Allow takes n entries from the app's bucket. When it cannot, it reports how long
// until n entries will be available, for the Retry-After header. A zero rate disables limiting.
func (l *AppLimiter) Allow(appID string, n int) (bool, time.Duration) {
	return l.AllowAt(appID, n, 0)
}

// AllowAt is Allow with a per-app rate (entries per second); 0 uses the limiter's own rate.
func (l *AppLimiter) AllowAt(appID string, n int, perSecond int) (bool, time.Duration) {
	rate := l.rate
	if perSecond > 0 {
		rate = float64(perSecond)
	}
	if rate <= 0 {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	capacity := rate * 2
	b := l.buckets[appID]
	now := l.now()
	if b == nil {
		b = &bucket{tokens: capacity, last: now}
		l.buckets[appID] = b
	}
	b.tokens = math.Min(capacity, b.tokens+now.Sub(b.last).Seconds()*rate)
	b.last = now
	need := float64(n)
	if need > capacity {
		need = capacity // a single request larger than the burst waits for a full bucket
	}
	if b.tokens >= need {
		b.tokens -= need
		return true, 0
	}
	l.limitedAt.Store(now.UnixNano())
	return false, time.Duration((need - b.tokens) / rate * float64(time.Second))
}

// RecentlyLimited reports whether any app was refused within the window.
func (l *AppLimiter) RecentlyLimited(window time.Duration) bool {
	at := l.limitedAt.Load()
	return at != 0 && l.now().Sub(time.Unix(0, at)) < window
}
