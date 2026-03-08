package kruda

// Property: For any combination of global MW, group MW, and handler,
// buildChain always produces a slice where global MW come first,
// then group MW, then handler last.

import (
	"math/rand"
	"testing"
)

// makeHandlers creates a slice of n distinct HandlerFuncs, each tagged with
// a unique index written into a shared []int so tests can verify call order.
func makeHandlers(n int, log *[]int, offset int) []HandlerFunc {
	handlers := make([]HandlerFunc, n)
	for i := 0; i < n; i++ {
		idx := offset + i // capture
		handlers[i] = func(c *Ctx) error {
			*log = append(*log, idx)
			return c.Next()
		}
	}
	return handlers
}

// makeHandler creates a single HandlerFunc tagged with the given index.
func makeHandler(log *[]int, idx int) HandlerFunc {
	return func(c *Ctx) error {
		*log = append(*log, idx)
		return nil
	}
}

// TestBuildChainOrderingProperty verifies the ordering property of buildChain
// across many randomly-sized combinations of global, group, and handler slices.
//
// Property: chain = buildChain(global, group, handler)
//   - chain[0 : len(global)]                  == global (in order)
//   - chain[len(global) : len(global)+len(group)] == group  (in order)
//   - chain[len(global)+len(group)]            == handler (last)
func TestBuildChainOrderingProperty(t *testing.T) {
	const iterations = 1000
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < iterations; i++ {
		// Random sizes: 0–7 global, 0–7 group
		nGlobal := rng.Intn(8)
		nGroup := rng.Intn(8)

		// Build identity-tagged handlers so we can verify positions.
		// Global handlers get indices 0..nGlobal-1
		// Group  handlers get indices nGlobal..nGlobal+nGroup-1
		// Route  handler  gets index  nGlobal+nGroup
		var log []int
		global := makeHandlers(nGlobal, &log, 0)
		group := makeHandlers(nGroup, &log, nGlobal)
		handler := makeHandler(&log, nGlobal+nGroup)

		chain := buildChain(global, group, handler)

		// --- Structural assertions ---

		expectedLen := nGlobal + nGroup + 1
		if len(chain) != expectedLen {
			t.Fatalf("iter %d: len(chain)=%d, want %d (nGlobal=%d nGroup=%d)",
				i, len(chain), expectedLen, nGlobal, nGroup)
		}

		// Verify handler is last by executing the chain and checking call order.
		// We need a minimal Ctx to drive c.Next().
		callLog := make([]int, 0, expectedLen)
		globalTagged := makeHandlers(nGlobal, &callLog, 0)
		groupTagged := makeHandlers(nGroup, &callLog, nGlobal)
		handlerTagged := makeHandler(&callLog, nGlobal+nGroup)

		builtChain := buildChain(globalTagged, groupTagged, handlerTagged)

		ctx := &Ctx{
			handlers:   builtChain,
			routeIndex: 0,
		}

		// Execute first handler (which calls Next() internally)
		if len(builtChain) > 0 {
			if err := builtChain[0](ctx); err != nil {
				t.Fatalf("iter %d: unexpected error from chain: %v", i, err)
			}
		}

		// Verify call order: 0, 1, ..., nGlobal-1, nGlobal, ..., nGlobal+nGroup-1, nGlobal+nGroup
		if len(callLog) != expectedLen {
			t.Fatalf("iter %d: callLog len=%d, want %d", i, len(callLog), expectedLen)
		}
		for j, got := range callLog {
			if got != j {
				t.Fatalf("iter %d: callLog[%d]=%d, want %d (global=%d group=%d)",
					i, j, got, j, nGlobal, nGroup)
			}
		}

		// Verify handler is the last element in the chain (index nGlobal+nGroup)
		lastIdx := nGlobal + nGroup
		if lastIdx < len(callLog) && callLog[lastIdx] != lastIdx {
			t.Fatalf("iter %d: handler not last: callLog[%d]=%d, want %d",
				i, lastIdx, callLog[lastIdx], lastIdx)
		}
	}
}
