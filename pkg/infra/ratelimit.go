package infra

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiter *rate.Limiter
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

func (r *RateLimiter) Allow() bool {
	return r.limiter.Allow()
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.limiter.Wait(ctx)
}

func (r *RateLimiter) Reserve() *rate.Reservation {
	return r.limiter.Reserve()
}

type RateLimiterFactory struct {
	mu       sync.Mutex
	limiters map[string]*RateLimiter
	rps      float64
	burst    int
}

func NewRateLimiterFactory(rps float64, burst int) *RateLimiterFactory {
	return &RateLimiterFactory{
		limiters: make(map[string]*RateLimiter),
		rps:      rps,
		burst:    burst,
	}
}

func (f *RateLimiterFactory) GetOrCreate(key string) *RateLimiter {
	f.mu.Lock()
	defer f.mu.Unlock()

	if limiter, ok := f.limiters[key]; ok {
		return limiter
	}

	limiter := NewRateLimiter(f.rps, f.burst)
	f.limiters[key] = limiter
	return limiter
}

func (f *RateLimiterFactory) Remove(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.limiters, key)
}
