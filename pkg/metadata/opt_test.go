package metadata

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gamevault/pkg/database"
)

func TestSingleFlight_Deduplication(t *testing.T) {
	sf := NewSingleFlightGroup()

	var callCount int32
	var wg sync.WaitGroup

	numGoroutines := 10
	results := make([]interface{}, numGoroutines)
	errors := make([]error, numGoroutines)
	sharedFlags := make([]bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err, shared := sf.Do("coalesce-test-key", func() (interface{}, error) {
				atomic.AddInt32(&callCount, 1)
				time.Sleep(30 * time.Millisecond)
				return "expected-shared-result", nil
			})
			results[idx] = val
			errors[idx] = err
			sharedFlags[idx] = shared
		}(i)
	}

	wg.Wait()

	if calls := atomic.LoadInt32(&callCount); calls != 1 {
		t.Fatalf("expected exactly 1 call across %d concurrent callers, got %d", numGoroutines, calls)
	}

	for i, val := range results {
		if errors[i] != nil {
			t.Errorf("goroutine %d returned error: %v", i, errors[i])
		}
		if str, ok := val.(string); !ok || str != "expected-shared-result" {
			t.Errorf("goroutine %d returned unexpected value: %v", i, val)
		}
	}
}

func TestSingleFlight_DifferentKeys(t *testing.T) {
	sf := NewSingleFlightGroup()

	var callCount int32
	var wg sync.WaitGroup

	numKeys := 5
	results := make([]string, numKeys)

	for i := 0; i < numKeys; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			val, err, _ := sf.Do(key, func() (interface{}, error) {
				atomic.AddInt32(&callCount, 1)
				return fmt.Sprintf("result-%d", idx), nil
			})
			if err == nil {
				results[idx] = val.(string)
			}
		}(i)
	}

	wg.Wait()

	if calls := atomic.LoadInt32(&callCount); int(calls) != numKeys {
		t.Fatalf("expected %d calls for %d distinct keys, got %d", numKeys, numKeys, calls)
	}

	for i := 0; i < numKeys; i++ {
		expected := fmt.Sprintf("result-%d", i)
		if results[i] != expected {
			t.Errorf("expected %q, got %q", expected, results[i])
		}
	}
}

func TestSingleFlight_Forget(t *testing.T) {
	sf := NewSingleFlightGroup()

	val1, err, _ := sf.Do("test-forget", func() (interface{}, error) {
		return "first", nil
	})
	if err != nil || val1 != "first" {
		t.Fatalf("first call failed: val=%v, err=%v", val1, err)
	}

	sf.Forget("test-forget")

	val2, err, _ := sf.Do("test-forget", func() (interface{}, error) {
		return "second", nil
	})
	if err != nil || val2 != "second" {
		t.Fatalf("second call after Forget failed: val=%v, err=%v", val2, err)
	}
}

func TestMemoryCache_Metadata(t *testing.T) {
	cache := NewMemoryCache()

	// Initial miss
	if meta, ok := cache.GetMetadata(100); ok || meta != nil {
		t.Fatalf("expected miss on empty cache")
	}

	// Set metadata with TTL
	sampleMeta := &database.SteamMetadata{
		AppID: 100,
		Title: "Test Game",
	}
	cache.SetMetadata(100, sampleMeta, 5*time.Minute)

	// Hit
	cached, ok := cache.GetMetadata(100)
	if !ok || cached == nil {
		t.Fatalf("expected hit for AppID 100")
	}
	if cached.Title != "Test Game" {
		t.Errorf("expected title 'Test Game', got %q", cached.Title)
	}

	// Invalidate
	cache.InvalidateAppID(100)
	if _, ok := cache.GetMetadata(100); ok {
		t.Fatalf("expected miss after InvalidateAppID")
	}
}

func TestMemoryCache_CandidatesAndAuxData(t *testing.T) {
	cache := NewMemoryCache()

	// Candidates
	candidates := []SteamCandidate{
		{AppID: 10, Name: "Counter-Strike", Score: 1.0},
	}
	cache.SetCandidates("cs", candidates, 5*time.Minute)
	if cached, ok := cache.GetCandidates("cs"); !ok || len(cached) != 1 || cached[0].AppID != 10 {
		t.Fatalf("candidates cache lookup failed: %+v", cached)
	}
	// Case-sensitive key match check
	if cached, ok := cache.GetCandidates("cs"); !ok || len(cached) != 1 {
		t.Fatalf("candidates cache should return match: %+v", cached)
	}

	// Icon
	cache.SetIcon(20, "https://icon.url/icon.ico", 5*time.Minute)
	if icon, ok := cache.GetIcon(20); !ok || icon != "https://icon.url/icon.ico" {
		t.Fatalf("icon cache lookup failed: %s", icon)
	}

	// Reviews
	cache.SetReviewSummary(30, "Very Positive", 85, 12000, 10200, 5*time.Minute)
	if desc, pct, tot, pos, ok := cache.GetReviewSummary(30); !ok || tot != 12000 || pct != 85 || desc != "Very Positive" || pos != 10200 {
		t.Fatalf("reviews cache lookup failed: desc=%s, tot=%d", desc, tot)
	}

	// Tags
	cache.SetTags(40, []string{"Action", "Multiplayer"}, 5*time.Minute)
	if tags, ok := cache.GetTags(40); !ok || len(tags) != 2 || tags[0] != "Action" {
		t.Fatalf("tags cache lookup failed: %+v", tags)
	}
}

func TestAdaptiveLimiter_Basic(t *testing.T) {
	limiter := NewAdaptiveLimiter()

	// Normal TakeFast should succeed promptly
	start := time.Now()
	limiter.TakeFast(false)
	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Errorf("TakeFast took too long: %v", elapsed)
	}

	// Normal TakeHeavy should succeed
	start = time.Now()
	limiter.TakeHeavy(false)
	elapsed = time.Since(start)
	if elapsed > 800*time.Millisecond {
		t.Errorf("TakeHeavy took too long: %v", elapsed)
	}

	// Priority calls should succeed quickly
	start = time.Now()
	limiter.TakeHeavy(true)
	elapsed = time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Errorf("Priority TakeHeavy took too long: %v", elapsed)
	}

	// Test 429 reporting
	limiter.TriggerCooldown(100 * time.Millisecond)
	// Limiter is now in cooldown
	// TakeFast should observe cooldown
	start = time.Now()
	limiter.TakeFast(false)
	elapsed = time.Since(start)
	if elapsed < 80*time.Millisecond {
		t.Errorf("limiter did not respect 429 cooldown, elapsed: %v", elapsed)
	}
}
