package downloader

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token-bucket rate limiter for bandwidth throttling
type RateLimiter struct {
	mu           sync.Mutex
	rateBytesSec int64
	tokens       int64
	lastRefill   time.Time
}

// NewRateLimiter creates a new limiter with speed limit in KB/s. Returns nil if kbps <= 0 (unlimited).
func NewRateLimiter(kbps int) *RateLimiter {
	if kbps <= 0 {
		return nil
	}
	rate := int64(kbps) * 1024
	return &RateLimiter{
		rateBytesSec: rate,
		tokens:       rate,
		lastRefill:   time.Now(),
	}
}

// SetLimit updates the transfer rate dynamically in KB/s (0 = unlimited).
func (r *RateLimiter) SetLimit(kbps int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if kbps <= 0 {
		r.rateBytesSec = 0
		return
	}
	rate := int64(kbps) * 1024
	r.rateBytesSec = rate
	if r.tokens > rate*2 {
		r.tokens = rate * 2
	}
	r.lastRefill = time.Now()
}

// Wait blocks until the requested number of bytes can be transferred or context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context, bytes int64) error {
	if r == nil || bytes <= 0 {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.mu.Lock()
		if r.rateBytesSec <= 0 {
			r.mu.Unlock()
			return nil
		}

		now := time.Now()
		elapsed := now.Sub(r.lastRefill).Seconds()
		r.tokens += int64(elapsed * float64(r.rateBytesSec))

		// Ensure max bucket capacity is at least as large as the requested chunk
		maxBurst := r.rateBytesSec * 2
		if bytes > maxBurst {
			maxBurst = bytes
		}
		if r.tokens > maxBurst {
			r.tokens = maxBurst
		}
		r.lastRefill = now

		if r.tokens >= bytes {
			r.tokens -= bytes
			r.mu.Unlock()
			return nil
		}

		needed := bytes - r.tokens
		sleepSec := float64(needed) / float64(r.rateBytesSec)
		if sleepSec > 0.05 {
			sleepSec = 0.05
		}
		sleepDur := time.Duration(sleepSec * float64(time.Second))
		r.mu.Unlock()

		timer := time.NewTimer(sleepDur)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
