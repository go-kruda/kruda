package kruda

import (
	"errors"
	"testing"
)

// --- buildChain unit tests ---

func TestBuildChain_EmptyGlobalAndGroup(t *testing.T) {
	handler := func(c *Ctx) error { return nil }
	chain := buildChain(nil, nil, handler)

	if len(chain) != 1 {
		t.Fatalf("expected chain length 1, got %d", len(chain))
	}
}

func TestBuildChain_OnlyGlobal(t *testing.T) {
	var order []string
	mw1 := func(c *Ctx) error { order = append(order, "g1"); return c.Next() }
	mw2 := func(c *Ctx) error { order = append(order, "g2"); return c.Next() }
	h := func(c *Ctx) error { order = append(order, "h"); return nil }

	chain := buildChain([]HandlerFunc{mw1, mw2}, nil, h)
	if len(chain) != 3 {
		t.Fatalf("expected 3, got %d", len(chain))
	}

	ctx := minimalCtx(chain)
	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, order, []string{"g1", "g2", "h"})
}

func TestBuildChain_OnlyGroup(t *testing.T) {
	var order []string
	mw1 := func(c *Ctx) error { order = append(order, "grp1"); return c.Next() }
	h := func(c *Ctx) error { order = append(order, "h"); return nil }

	chain := buildChain(nil, []HandlerFunc{mw1}, h)
	if len(chain) != 2 {
		t.Fatalf("expected 2, got %d", len(chain))
	}

	ctx := minimalCtx(chain)
	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, order, []string{"grp1", "h"})
}

func TestBuildChain_GlobalThenGroupThenHandler(t *testing.T) {
	var order []string
	g1 := func(c *Ctx) error { order = append(order, "global1"); return c.Next() }
	g2 := func(c *Ctx) error { order = append(order, "global2"); return c.Next() }
	grp1 := func(c *Ctx) error { order = append(order, "group1"); return c.Next() }
	h := func(c *Ctx) error { order = append(order, "handler"); return nil }

	chain := buildChain([]HandlerFunc{g1, g2}, []HandlerFunc{grp1}, h)
	if len(chain) != 4 {
		t.Fatalf("expected 4, got %d", len(chain))
	}

	ctx := minimalCtx(chain)
	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, order, []string{"global1", "global2", "group1", "handler"})
}

func TestBuildChain_CapacityIsExact(t *testing.T) {
	g := []HandlerFunc{func(c *Ctx) error { return nil }, func(c *Ctx) error { return nil }}
	grp := []HandlerFunc{func(c *Ctx) error { return nil }}
	h := func(c *Ctx) error { return nil }

	chain := buildChain(g, grp, h)
	// cap should equal len (allocated exactly once)
	if cap(chain) != len(chain) {
		t.Fatalf("expected cap=%d to equal len=%d", cap(chain), len(chain))
	}
}

// --- c.Next() progression tests ---

func TestNext_ProgressesThroughChain(t *testing.T) {
	var visited []int

	h0 := func(c *Ctx) error { visited = append(visited, 0); return c.Next() }
	h1 := func(c *Ctx) error { visited = append(visited, 1); return c.Next() }
	h2 := func(c *Ctx) error { visited = append(visited, 2); return nil }

	chain := []HandlerFunc{h0, h1, h2}
	ctx := minimalCtx(chain)

	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, visited, []int{0, 1, 2})
}

func TestNext_BeyondChainReturnsNil(t *testing.T) {
	h := func(c *Ctx) error { return nil }
	chain := []HandlerFunc{h}
	ctx := minimalCtx(chain)

	// Manually advance past the end
	ctx.routeIndex = len(chain)
	err := ctx.Next()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// --- Short-circuit on error tests ---

func TestShortCircuit_ErrorSkipsRemainingHandlers(t *testing.T) {
	var visited []string
	sentinel := errors.New("stop here")

	mw1 := func(c *Ctx) error {
		visited = append(visited, "mw1")
		return sentinel // return error WITHOUT calling Next()
	}
	mw2 := func(c *Ctx) error {
		visited = append(visited, "mw2") // must NOT be reached
		return c.Next()
	}
	h := func(c *Ctx) error {
		visited = append(visited, "handler") // must NOT be reached
		return nil
	}

	chain := buildChain([]HandlerFunc{mw1}, []HandlerFunc{mw2}, h)
	ctx := minimalCtx(chain)

	err := chain[0](ctx)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	assertOrder(t, visited, []string{"mw1"})
}

func TestShortCircuit_MiddlewareCanChooseNotToCallNext(t *testing.T) {
	var visited []string

	auth := func(c *Ctx) error {
		visited = append(visited, "auth")
		// Simulate auth check — does NOT call Next, does NOT return error
		// (e.g. already responded). In real usage this would write a response.
		return nil
	}
	h := func(c *Ctx) error {
		visited = append(visited, "handler") // should NOT be reached
		return nil
	}

	chain := buildChain(nil, []HandlerFunc{auth}, h)
	ctx := minimalCtx(chain)

	if err := chain[0](ctx); err != nil {
		t.Fatal(err)
	}
	// handler should not have been called because auth didn't call Next()
	assertOrder(t, visited, []string{"auth"})
}

// --- MiddlewareFunc alias test ---

func TestMiddlewareFuncIsAliasForHandlerFunc(t *testing.T) {
	// MiddlewareFunc = HandlerFunc means they are the same type.
	// This test verifies that a MiddlewareFunc can be used wherever a HandlerFunc is expected.
	var mw MiddlewareFunc = func(c *Ctx) error { return c.Next() }
	var hf HandlerFunc = mw // assignment must compile without conversion

	chain := buildChain([]HandlerFunc{hf}, nil, func(c *Ctx) error { return nil })
	if len(chain) != 2 {
		t.Fatalf("expected 2, got %d", len(chain))
	}
}

// --- helpers ---

// minimalCtx creates a bare Ctx with the given handler chain for testing.
// It does not require a full App — only the fields used by Next() are set.
func minimalCtx(chain []HandlerFunc) *Ctx {
	return &Ctx{
		handlers:   chain,
		routeIndex: 0,
	}
}

func assertOrder[T comparable](t *testing.T, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order length mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d]: got %v, want %v (full: got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}
