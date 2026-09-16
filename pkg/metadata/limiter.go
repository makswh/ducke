package metadata

import (
	"math"
	"sync"
	"time"
)

// AdaptiveLimiter manages multi-tiered rate limiting with priority channels and global cooldown.
type AdaptiveLimiter struct {
	mu            sync.Mutex
	cooldownUntil time.Time

	// Heavy endpoint bucket (store.steampowered.com/api/appdetails)
	heavyTokens     float64
	heavyCapacity   float64
	heavyRefillRate float64
	heavyLastRefill time.Time

	// Fast endpoint bucket (search/suggest, community, appreviews, tags)
	fastTokens     float64
	fastCapacity   float64
	fastRefillRate float64
	fastLastRefill time.Time
}

// NewAdaptiveLimiter creates rate limiters tuned for Steam API tiers.
func NewAdaptiveLimiter() *AdaptiveLimiter {
	now := time.Now()
	return &AdaptiveLimiter{
		// Heavy bucket: burst 4, 2.0 tokens/sec
		heavyTokens:     4.0,
		heavyCapacity:   4.0,
		heavyRefillRate: 2.0,
		heavyLastRefill: now,

		// Fast bucket: burst 12, 8.0 tokens/sec
		fastTokens:     12.0,
		fastCapacity:   12.0,
		fastRefillRate: 8.0,
		fastLastRefill: now,
	}
}

// TriggerCooldown pauses all traffic for duration d when Steam responds with HTTP 429.
func (l *AdaptiveLimiter) TriggerCooldown(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	until := time.Now().Add(d)
	if until.After(l.cooldownUntil) {
		l.cooldownUntil = until
		l.heavyTokens = 0
		l.fastTokens = 0
	}
}

// InCooldown returns true if currently within a cooldown window.
func (l *AdaptiveLimiter) InCooldown() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Now().Before(l.cooldownUntil)
}

// TakeHeavy consumes 1 token for heavy store API requests (appdetails).
// If priority is true, it only waits out cooldown and bypasses background rate delay.
func (l *AdaptiveLimiter) TakeHeavy(priority bool) {
	for {
		l.mu.Lock()
		now := time.Now()

		if now.Before(l.cooldownUntil) {
			wait := l.cooldownUntil.Sub(now)
			l.mu.Unlock()
			time.Sleep(wait)
			continue
		}

		if priority {
			// Priority caller gets immediate pass outside cooldown
			l.heavyTokens = math.Max(0, l.heavyTokens-1.0)
			l.mu.Unlock()
			return
		}

		elapsed := now.Sub(l.heavyLastRefill).Seconds()
		l.heavyTokens = math.Min(l.heavyCapacity, l.heavyTokens+elapsed*l.heavyRefillRate)
		l.heavyLastRefill = now

		if l.heavyTokens >= 1.0 {
			l.heavyTokens -= 1.0
			l.mu.Unlock()
			return
		}

		needed := 1.0 - l.heavyTokens
		sleepDur := time.Duration(needed/l.heavyRefillRate*float64(time.Second)) + time.Millisecond
		l.mu.Unlock()

		if sleepDur > 0 {
			time.Sleep(sleepDur)
		}
	}
}

// TakeFast consumes 1 token for lightweight suggest/community requests.
// If priority is true, it only waits out cooldown.
func (l *AdaptiveLimiter) TakeFast(priority bool) {
	for {
		l.mu.Lock()
		now := time.Now()

		if now.Before(l.cooldownUntil) {
			wait := l.cooldownUntil.Sub(now)
			l.mu.Unlock()
			time.Sleep(wait)
			continue
		}

		if priority {
			l.fastTokens = math.Max(0, l.fastTokens-1.0)
			l.mu.Unlock()
			return
		}

		elapsed := now.Sub(l.fastLastRefill).Seconds()
		l.fastTokens = math.Min(l.fastCapacity, l.fastTokens+elapsed*l.fastRefillRate)
		l.fastLastRefill = now

		if l.fastTokens >= 1.0 {
			l.fastTokens -= 1.0
			l.mu.Unlock()
			return
		}

		needed := 1.0 - l.fastTokens
		sleepDur := time.Duration(needed/l.fastRefillRate*float64(time.Second)) + time.Millisecond
		l.mu.Unlock()

		if sleepDur > 0 {
			time.Sleep(sleepDur)
		}
	}
}
