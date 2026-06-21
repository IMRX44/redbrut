package worker

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPLimiter manages per-IP rate limiting and lockout tracking.
type IPLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*rate.Limiter
	paused  map[string]time.Time // IP → resume time
	rps     float64
}

func NewIPLimiter(rps float64) *IPLimiter {
	return &IPLimiter{
		buckets: make(map[string]*rate.Limiter),
		paused:  make(map[string]time.Time),
		rps:     rps,
	}
}

func (l *IPLimiter) getLimiter(ip string) *rate.Limiter {
	// Fast path: read lock (no allocation, common case).
	l.mu.RLock()
	lim, ok := l.buckets[ip]
	l.mu.RUnlock()
	if ok {
		return lim
	}
	// Slow path: create new limiter.
	l.mu.Lock()
	defer l.mu.Unlock()
	// Re-check after acquiring write lock.
	if lim, ok = l.buckets[ip]; ok {
		return lim
	}
	burst := int(l.rps) + 1
	if burst < 1 {
		burst = 1
	}
	lim = rate.NewLimiter(rate.Limit(l.rps), burst)
	l.buckets[ip] = lim
	return lim
}

// IsPaused returns true if the IP has an active lockout pause.
func (l *IPLimiter) IsPaused(ip string) bool {
	l.mu.RLock()
	t, ok := l.paused[ip]
	l.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().Before(t) {
		return true
	}
	// Pause expired — clean up.
	l.mu.Lock()
	delete(l.paused, ip)
	l.mu.Unlock()
	return false
}

// PauseIP marks an IP as locked out for the given duration.
func (l *IPLimiter) PauseIP(ip string, d time.Duration) {
	l.mu.Lock()
	l.paused[ip] = time.Now().Add(d)
	l.mu.Unlock()
}

// Wait blocks until the rate limiter allows a request for the given IP.
func (l *IPLimiter) Wait(ctx context.Context, ip string) error {
	return l.getLimiter(ip).Wait(ctx)
}
