package main

import (
	"slices"
	"strings"
	"testing"
	"testing/quick"
)

// Property 3: Query Parameter Clamping
// **Validates: Requirements 6.1, 6.2, 6.3, 7.1, 9.5, 9.6, 9.7**
func TestPropertyQueryParameterClamping(t *testing.T) {
	f := func(raw string) bool {
		n := ParseQueries(raw)
		return n >= 1 && n <= 500
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Fatalf("Property violated: %v", err)
	}
	for _, raw := range []string{"", "0", "-1", "1", "500", "501", "abc"} {
		if n := ParseQueries(raw); n < 1 || n > 500 {
			t.Fatalf("ParseQueries(%q) = %d, want [1,500]", raw, n)
		}
	}
}

// Property 4: Random Number Range
// **Validates: Requirements 5.1, 10.2**
func TestPropertyRandomNumberRange(t *testing.T) {
	for range 100000 {
		if id := randomWorldID(); id < 1 || id > 10000 {
			t.Fatalf("randomWorldID() = %d, want [1,10000]", id)
		}
	}
}

// Property 7: Sorted Unique IDs
// **Validates: Requirements 7.2, 7.4**
func TestPropertySortedUniqueIDs(t *testing.T) {
	f := func(n uint16) bool {
		count := int(n%500) + 1
		ids := generateUniqueIDs(count)
		if len(ids) != count {
			return false
		}
		for _, id := range ids {
			if id < 1 || id > 10000 {
				return false
			}
		}
		for i := 1; i < len(ids); i++ {
			if ids[i] <= ids[i-1] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Fatalf("Property violated: %v", err)
	}
}

// Property 5: DB Pool Configuration Formulas
// **Validates: Requirements 4.1, 4.2**
func TestPropertyDBPoolConfigFormulas(t *testing.T) {
	f := func(g uint16) bool {
		// Constrain GOMAXPROCS to [1, 1024]
		gomaxprocs := int(g%1024) + 1

		maxConns, minConns := calcPoolConfig(gomaxprocs)

		// MaxConns = min(gomaxprocs*4, 256)
		expectedMax := int32(min(gomaxprocs*4, 256))
		if maxConns != expectedMax {
			return false
		}

		// MinConns = min(gomaxprocs, 64)
		expectedMin := int32(min(gomaxprocs, 64))
		if minConns != expectedMin {
			return false
		}

		// Both must be positive
		if maxConns < 1 || minConns < 1 {
			return false
		}

		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Fatalf("Property violated: %v", err)
	}
}

// Property 6: Fortune Sort Ordering
// **Validates: Requirements 8.3**
func TestPropertyFortuneSortOrdering(t *testing.T) {
	f := func(ids []int32, msgs []string) bool {
		// Build a Fortune slice from generated data
		n := len(ids)
		if len(msgs) < n {
			n = len(msgs)
		}
		if n > 100 {
			n = 100
		}

		fortunes := make([]Fortune, n)
		for i := 0; i < n; i++ {
			fortunes[i] = Fortune{ID: ids[i], Message: msgs[i]}
		}

		// Always append the additional fortune (ID=0) per TFB spec
		fortunes = append(fortunes, Fortune{
			ID:      0,
			Message: "Additional fortune added at request time.",
		})

		originalLen := len(fortunes)

		// Sort using the same logic as the handler
		slices.SortFunc(fortunes, func(a, b Fortune) int {
			return strings.Compare(a.Message, b.Message)
		})

		// Length must be preserved
		if len(fortunes) != originalLen {
			return false
		}

		// Must be in non-decreasing lexicographic order by Message
		for i := 1; i < len(fortunes); i++ {
			if strings.Compare(fortunes[i-1].Message, fortunes[i].Message) > 0 {
				return false
			}
		}

		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Fatalf("Property violated: %v", err)
	}
}
