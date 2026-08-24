package inbox

import (
	"sync"
	"time"
)

// RateLimiter is a per-IP token bucket ahead of signature verification.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string][]time.Time
}

// NewRateLimiter creates a limiter allowing limit events per window per key.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, buckets: make(map[string][]time.Time)}
}

// Allow reports whether the key may proceed.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-r.window)
	var kept []time.Time
	for _, t := range r.buckets[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.limit {
		r.buckets[key] = kept
		return false
	}
	r.buckets[key] = append(kept, now)
	return true
}
