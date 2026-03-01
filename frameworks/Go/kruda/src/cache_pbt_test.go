package main

import (
	"testing"
	"testing/quick"
)

// For any ID in [1,10000], after warmup, Get(id) returns World with matching
// ID and RandomNumber in [1,10000].
func TestPropertyCacheLookupCorrectness(t *testing.T) {
	// Manually populate the cache (no real DB needed for property testing).
	var cache WorldCache
	for id := int32(1); id <= 10000; id++ {
		// Deterministic but varied random numbers: (id*7 % 10000) + 1
		rn := (id*7)%10000 + 1
		cache.data[id] = World{ID: id, RandomNumber: rn}
	}

	f := func(rawID uint16) bool {
		// Map uint16 [0,65535] into valid ID range [1,10000]
		id := int(rawID%10000) + 1

		w := cache.Get(id)

		// ID must match the requested ID
		if int(w.ID) != id {
			t.Logf("Get(%d): expected ID=%d, got ID=%d", id, id, w.ID)
			return false
		}

		// RandomNumber must be in [1, 10000]
		if w.RandomNumber < 1 || w.RandomNumber > 10000 {
			t.Logf("Get(%d): RandomNumber=%d out of range [1,10000]", id, w.RandomNumber)
			return false
		}

		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

// Out-of-range IDs return zero-value World.
func TestPropertyCacheBoundsCheck(t *testing.T) {
	var cache WorldCache
	// Populate valid range
	for id := int32(1); id <= 10000; id++ {
		cache.data[id] = World{ID: id, RandomNumber: (id*3)%10000 + 1}
	}

	// Test out-of-range IDs
	outOfRange := []int{0, -1, -100, 10001, 10002, 99999}
	for _, id := range outOfRange {
		w := cache.Get(id)
		if w.ID != 0 || w.RandomNumber != 0 {
			t.Errorf("Get(%d): expected zero World, got %+v", id, w)
		}
	}
}
