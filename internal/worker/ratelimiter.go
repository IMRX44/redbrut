package worker

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPLimiter manages per-IP rate limiting and lockout tracking.
type IPLimiter struct {
	mu      sync.Mutex
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
	l.mu.Lock()
	defer l.mu.Unlock()
	if lim, ok := l.buckets[ip]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(l.rps), int(l.rps)+1)
	l.buckets[ip] = lim
	return lim
}

// IsPaused returns true if the IP has an active lockout pause.
func (l *IPLimiter) IsPaused(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t, ok := l.paused[ip]; ok {
		if time.Now().Before(t) {
			return true
		}
		delete(l.paused, ip)
	}
	return false
}

// PauseIP marks an IP as locked out for the given duration.
func (l *IPLimiter) PauseIP(ip string, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused[ip] = time.Now().Add(d)
}

// Wait blocks until the rate limiter allows a request for the given IP.
func (l *IPLimiter) Wait(ctx context.Context, ip string) error {
	return l.getLimiter(ip).Wait(ctx)
}
