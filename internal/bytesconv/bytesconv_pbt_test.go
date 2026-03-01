package bytesconv

// Property: For all byte slices b, UnsafeBytes(UnsafeString(b)) produces a slice
// with content identical to b (round-trip consistency).

import (
	"bytes"
	"testing"
	"testing/quick"
)

func TestRoundTripProperty(t *testing.T) {
	// Property: for all []byte b, bytes.Equal(UnsafeBytes(UnsafeString(b)), b) is true.
	// Note: UnsafeString([]byte{}) returns "" and UnsafeBytes("") returns nil.
	// bytes.Equal(nil, []byte{}) returns true, so the empty-slice edge case is handled correctly.
	property := func(b []byte) bool {
		result := UnsafeBytes(UnsafeString(b))
		return bytes.Equal(result, b)
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(property, cfg); err != nil {
		t.Errorf("round-trip property failed: %v", err)
	}
}
