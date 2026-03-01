package main

import (
	"testing"
	"testing/quick"
)

// For any non-negative size, GetBuffer returns a buffer from the correct tier;
// PutBuffer discards buffers > 64KB.
func TestPropertyBufferPoolTierSelection(t *testing.T) {
	f := func(size uint16) bool {
		expectedSize := int(size) // [0, 65535] covers all tiers

		bp := GetBuffer(expectedSize)
		if bp == nil {
			t.Log("GetBuffer returned nil")
			return false
		}

		buf := *bp
		// Buffer must have length 0 (reset on get)
		if len(buf) != 0 {
			t.Logf("expected len 0, got %d", len(buf))
			return false
		}

		c := cap(buf)

		// Verify tier selection: capacity must be >= the tier minimum
		switch {
		case expectedSize <= 1024:
			if c < 1024 {
				t.Logf("small tier: size=%d, cap=%d < 1024", expectedSize, c)
				return false
			}
		case expectedSize <= 8192:
			if c < 8192 {
				t.Logf("medium tier: size=%d, cap=%d < 8192", expectedSize, c)
				return false
			}
		default:
			if c < 32768 {
				t.Logf("large tier: size=%d, cap=%d < 32768", expectedSize, c)
				return false
			}
		}

		// Return buffer to pool (should not panic)
		PutBuffer(bp)
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}

// PutBuffer discards buffers with capacity > 64KB.
func TestPropertyBufferPoolOversizeDiscard(t *testing.T) {
	// Create an oversized buffer (cap > 64KB) and put it back.
	// Then get a fresh buffer — it should come from the pool's New func
	// (i.e., have the default tier capacity), proving the oversized one was discarded.
	oversized := make([]byte, 0, 100_000) // 100KB > 64KB limit
	bp := &oversized
	PutBuffer(bp) // should discard, not return to any pool

	// Get from large pool — should get a fresh 32KB buffer, not the 100KB one.
	// We can't guarantee this due to sync.Pool GC behavior, but we can verify
	// that PutBuffer doesn't panic on oversized buffers.
	got := GetBuffer(32769) // large tier
	if got == nil {
		t.Fatal("GetBuffer returned nil after oversized PutBuffer")
	}
	PutBuffer(got)
}

// PutBuffer with nil must not panic.
func TestPropertyBufferPoolNilSafety(t *testing.T) {
	PutBuffer(nil) // must not panic
}
