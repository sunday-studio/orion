package logstore

import (
	"math"
	"sync"
	"time"
)

// AppLimiter is a per-app token bucket measured in entries: rate per second, burst of twice that.
type AppLimiter struct {
	mu      sync.Mutex
	rate    float64
	buckets map[string]*bucket
	now     func() time.Time
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
	if l.rate <= 0 {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	capacity := l.rate * 2
	b := l.buckets[appID]
	now := l.now()
	if b == nil {
		b = &bucket{tokens: capacity, last: now}
		l.buckets[appID] = b
	}
	b.tokens = math.Min(capacity, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	need := float64(n)
	if need > capacity {
		need = capacity // a single request larger than the burst waits for a full bucket
	}
	if b.tokens >= need {
		b.tokens -= need
		return true, 0
	}
	return false, time.Duration((need - b.tokens) / l.rate * float64(time.Second))
}
