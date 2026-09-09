package downloader

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Unlimited(t *testing.T) {
	limiter := NewRateLimiter(0)
	if limiter != nil {
		t.Fatalf("expected nil limiter for 0 kbps, got %v", limiter)
	}

	// Wait with nil limiter should return immediately
	ctx := context.Background()
	if err := limiter.Wait(ctx, 1024*1024); err != nil {
		t.Fatalf("expected nil error for nil limiter, got %v", err)
	}
}

func TestRateLimiter_Throttling(t *testing.T) {
	// Limit to 100 KB/s
	limiter := NewRateLimiter(100)
	if limiter == nil {
		t.Fatal("expected non-nil limiter")
	}

	ctx := context.Background()

	// Drain initial tokens
	_ = limiter.Wait(ctx, 100*1024)

	start := time.Now()
	// Next 30 KB should take around ~200-400ms at 100 KB/s
	err := limiter.Wait(ctx, 30*1024)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed < 150*time.Millisecond {
		t.Fatalf("expected rate limiting delay, got %v", elapsed)
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 KB/s
	ctx, cancel := context.WithCancel(context.Background())

	// Drain initial tokens
	_ = limiter.Wait(ctx, 10*1024)

	// Cancel context quickly while Wait should be sleeping
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := limiter.Wait(ctx, 30*1024)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestRateLimiter_DynamicLimit(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 KB/s
	if limiter == nil {
		t.Fatal("expected non-nil limiter")
	}

	// Change to unlimited
	limiter.SetLimit(0)
	ctx := context.Background()
	start := time.Now()
	err := limiter.Wait(ctx, 1024*1024)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected immediate completion when unlimited, took %v", elapsed)
	}
}
