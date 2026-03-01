package main

import (
	"net/http"
	"testing"
	"time"
)

// TestPropertyDateHeaderRFC1123Format validates that GetDateHeader() returns
// a byte slice parseable as a valid RFC1123 date.
//
// **Validates: Requirements 13.2**
//
// Property 13: Date Header RFC1123 Format
// For any call to GetDateHeader(), the returned byte slice must be parseable
// as a valid RFC1123 date using http.TimeFormat.
func TestPropertyDateHeaderRFC1123Format(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		header := GetDateHeader()

		if len(header) == 0 {
			t.Fatal("GetDateHeader() returned empty byte slice")
		}

		s := string(header)
		parsed, err := time.Parse(http.TimeFormat, s)
		if err != nil {
			t.Fatalf("GetDateHeader() returned unparseable date %q: %v", s, err)
		}

		// The parsed time should be within a reasonable window of now (±2 seconds
		// to account for ticker lag and test execution time).
		now := time.Now().UTC()
		diff := now.Sub(parsed)
		if diff < -2*time.Second || diff > 2*time.Second {
			t.Fatalf("GetDateHeader() time %v is too far from now %v (diff=%v)", parsed, now, diff)
		}
	}
}
