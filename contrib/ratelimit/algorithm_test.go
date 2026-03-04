package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucket_RefillOverTime(t *testing.T) {
	e := &entry{}
	limit := 10
	window := time.Minute

	// Exhaust all tokens
	for i := 0; i < limit; i++ {
		r := tokenBucketAllow(e, limit, window)
		if !r.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Should be rejected now
	r := tokenBucketAllow(e, limit, window)
	if r.Allowed {
		t.Fatal("should be rejected after exhausting tokens")
	}

	// Simulate time passing: set last to past so refill happens
	// At rate = 10/60s = 1 token every 6s, set 12s to refill ~2 tokens
	e.mu.Lock()
	e.last = time.Now().Add(-12 * time.Second)
	e.mu.Unlock()

	r = tokenBucketAllow(e, limit, window)
	if !r.Allowed {
		t.Fatal("should be allowed after token refill")
	}
}

func TestTokenBucket_CapAtMax(t *testing.T) {
	e := &entry{}
	limit := 5
	window := time.Minute

	// Use 1 token
	tokenBucketAllow(e, limit, window)

	// Simulate very long idle — tokens should cap at limit
	e.mu.Lock()
	e.last = time.Now().Add(-1 * time.Hour)
	e.mu.Unlock()

	r := tokenBucketAllow(e, limit, window)
	if !r.Allowed {
		t.Fatal("should be allowed")
	}
	// After capping at 5 and consuming 1, remaining should be 4
	if r.Remaining != limit-1 {
		t.Errorf("remaining = %d, want %d", r.Remaining, limit-1)
	}
}

func TestTokenBucket_RetryAfterAccuracy(t *testing.T) {
	e := &entry{}
	limit := 5
	window := 10 * time.Second

	// Exhaust all tokens
	for i := 0; i < limit; i++ {
		tokenBucketAllow(e, limit, window)
	}

	r := tokenBucketAllow(e, limit, window)
	if r.Allowed {
		t.Fatal("should be rejected")
	}
	if r.RetryAt <= 0 {
		t.Error("RetryAt should be positive when rejected")
	}
	if r.RetryAt > window {
		t.Errorf("RetryAt = %v, should not exceed window %v", r.RetryAt, window)
	}
}

func TestSlidingWindow_WindowRotation(t *testing.T) {
	e := &entry{}
	limit := 10
	window := time.Minute

	// Make 5 requests in current window
	for i := 0; i < 5; i++ {
		slidingWindowAllow(e, limit, window)
	}

	// Simulate window advancement
	e.mu.Lock()
	currentCount := e.count
	e.windowStart = time.Now().Add(-window - time.Second) // just past one window
	e.mu.Unlock()

	// Next request should trigger window rotation
	slidingWindowAllow(e, limit, window)

	e.mu.Lock()
	defer e.mu.Unlock()

	// Previous window's count should be the old current count
	if e.prevCount != currentCount {
		t.Errorf("prevCount = %d, want %d (old current)", e.prevCount, currentCount)
	}
	// New count should be 1 (this request)
	if e.count != 1 {
		t.Errorf("count = %d, want 1", e.count)
	}
}

func TestSlidingWindow_DoubleWindowSkip(t *testing.T) {
	e := &entry{}
	limit := 10
	window := time.Minute

	// Make some requests
	for i := 0; i < 5; i++ {
		slidingWindowAllow(e, limit, window)
	}

	// Simulate 2+ windows passing
	e.mu.Lock()
	e.windowStart = time.Now().Add(-3 * window)
	e.mu.Unlock()

	r := slidingWindowAllow(e, limit, window)
	if !r.Allowed {
		t.Fatal("should be allowed after double window skip")
	}

	// Both counts should be reset
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.prevCount != 0 {
		t.Errorf("prevCount = %d, want 0 after double skip", e.prevCount)
	}
	if e.count != 1 {
		t.Errorf("count = %d, want 1", e.count)
	}
}

func TestSlidingWindow_WeightCalculation(t *testing.T) {
	e := &entry{}
	limit := 10
	window := time.Second // Use 1s window for easier reasoning

	// Fill previous window completely
	e.mu.Lock()
	e.prevCount = 10
	e.count = 0
	e.windowStart = time.Now().Add(-500 * time.Millisecond) // halfway through window
	e.last = time.Now()
	e.mu.Unlock()

	// At midpoint, weight ≈ 0.5, so estimated = 10*0.5 + 0 = 5
	// With limit 10, remaining should be ~5, so this should be allowed
	r := slidingWindowAllow(e, limit, window)
	if !r.Allowed {
		t.Fatal("should be allowed at window midpoint with half-weight prev")
	}
}

func TestSlidingWindow_BoundaryRequest(t *testing.T) {
	e := &entry{}
	limit := 5
	window := time.Second

	// Fill to exactly the limit in current window
	for i := 0; i < limit; i++ {
		r := slidingWindowAllow(e, limit, window)
		if !r.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// Next request at the limit should be rejected
	r := slidingWindowAllow(e, limit, window)
	if r.Allowed {
		t.Fatal("should be rejected at the limit")
	}
	if r.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", r.Remaining)
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore(time.Minute)
	defer store.Stop()

	limit := 100
	window := time.Minute
	goroutines := 100

	var allowed atomic.Int64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			r := store.Allow("same-key", limit, window)
			if r.Allowed {
				allowed.Add(1)
			}
		}()
	}

	wg.Wait()

	// All 100 should be allowed since limit=100
	if got := allowed.Load(); got != int64(goroutines) {
		t.Errorf("allowed = %d, want %d", got, goroutines)
	}
}

func TestMemoryStore_ManyKeys(t *testing.T) {
	store := NewMemoryStore(time.Minute)
	defer store.Stop()

	limit := 2
	window := time.Minute

	// 100 different keys should each get their own quota
	for i := 0; i < 100; i++ {
		key := "client-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		r := store.Allow(key, limit, window)
		if !r.Allowed {
			t.Errorf("key %q first request should be allowed", key)
		}
		if r.Remaining != limit-1 {
			t.Errorf("key %q remaining = %d, want %d", key, r.Remaining, limit-1)
		}
	}
}
