package ratelimit

import (
	"math/rand"
	"testing"
	"testing/quick"
	"time"
)

// TestPropertyTokenBucket_WithinLimitAlwaysAllowed verifies that requests
// within the configured limit are always allowed (R11.15).
func TestPropertyTokenBucket_WithinLimitAlwaysAllowed(t *testing.T) {
	f := func(limit uint8) bool {
		// Clamp to reasonable range
		max := int(limit)%50 + 1 // 1-50

		store := &MemoryStore{}
		key := "test-client"

		for i := 0; i < max; i++ {
			e := store.getEntry(key)
			result := tokenBucketAllow(e, max, time.Minute)
			if !result.Allowed {
				return false // should always be allowed within limit
			}
			if result.Remaining < 0 {
				return false // remaining should never be negative
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// TestPropertySlidingWindow_WithinLimitAlwaysAllowed verifies the same
// property for the sliding window algorithm.
func TestPropertySlidingWindow_WithinLimitAlwaysAllowed(t *testing.T) {
	f := func(limit uint8) bool {
		max := int(limit)%50 + 1

		store := &MemoryStore{}
		key := "test-client"

		for i := 0; i < max; i++ {
			e := store.getEntry(key)
			result := slidingWindowAllow(e, max, time.Minute)
			if !result.Allowed {
				return false
			}
			if result.Remaining < 0 {
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// TestPropertyTokenBucket_ExceedAlwaysRejects verifies that the (limit+1)th
// request is always rejected when all requests happen within the same window.
func TestPropertyTokenBucket_ExceedAlwaysRejects(t *testing.T) {
	f := func(limit uint8) bool {
		max := int(limit)%50 + 1

		store := &MemoryStore{}
		key := "test-client"

		// Exhaust all tokens
		for i := 0; i < max; i++ {
			e := store.getEntry(key)
			tokenBucketAllow(e, max, time.Hour) // long window so no refill
		}

		// Next request must be rejected
		e := store.getEntry(key)
		result := tokenBucketAllow(e, max, time.Hour)
		return !result.Allowed
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// TestPropertyRemainingNeverExceedsLimit verifies that Remaining never
// exceeds the configured limit for any sequence of requests.
func TestPropertyRemainingNeverExceedsLimit(t *testing.T) {
	f := func(seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		max := rng.Intn(20) + 1
		numRequests := rng.Intn(50) + 1

		store := &MemoryStore{}
		key := "test-client"

		for i := 0; i < numRequests; i++ {
			e := store.getEntry(key)
			result := tokenBucketAllow(e, max, time.Minute)
			if result.Remaining > max {
				return false
			}
			if result.Remaining < 0 {
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}
