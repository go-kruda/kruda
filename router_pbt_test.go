package kruda

// Validates: Requirements 1.2, 1.3, 1.8
//
// Property 1: For any registered static route, find(method, path) always
// returns the registered handler chain, and find returns nil for unregistered paths.
//
// Property 2: For any registered param route `/x/:name`, find returns handler
// and params["name"] equals the path segment.

import (
	"math/rand"
	"testing"
)

// randomAlphaSegment generates a random lowercase alpha string of length [minLen, maxLen].
func randomAlphaSegment(rng *rand.Rand, minLen, maxLen int) string {
	n := minLen + rng.Intn(maxLen-minLen+1)
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(rng.Intn(26))
	}
	return string(b)
}

// randomStaticPath generates a random path like "/abc/def/ghi" with
// [minSegments, maxSegments] segments.
func randomStaticPath(rng *rand.Rand, minSegments, maxSegments int) string {
	n := minSegments + rng.Intn(maxSegments-minSegments+1)
	path := ""
	for i := 0; i < n; i++ {
		path += "/" + randomAlphaSegment(rng, 1, 8)
	}
	if path == "" {
		path = "/" + randomAlphaSegment(rng, 1, 8)
	}
	return path
}

// TestRouterStaticFindProperty verifies that for any registered static route,
// find(method, path) always returns a non-nil handler chain, and find returns
// nil for paths that were never registered.
func TestRouterStaticFindProperty(t *testing.T) {
	const iterations = 500
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < iterations; i++ {
		r := newRouter()
		params := make(map[string]string, 4)

		// Generate 1–10 unique static paths
		numRoutes := 1 + rng.Intn(10)
		registered := make(map[string]bool, numRoutes)
		for j := 0; j < numRoutes; j++ {
			path := randomStaticPath(rng, 1, 4)
			if registered[path] {
				continue // skip duplicates
			}
			registered[path] = true
			h := []HandlerFunc{dummyHandler()}
			r.addRoute("GET", path, h)
		}

		// Every registered path must be found
		for path := range registered {
			clear(params)
			if got := r.find("GET", path, params); got == nil {
				t.Fatalf("iter %d: find(GET, %q) returned nil, want non-nil", i, path)
			}
		}

		// Generate random unregistered paths and verify they return nil
		for j := 0; j < 5; j++ {
			unregistered := randomStaticPath(rng, 1, 4)
			if registered[unregistered] {
				continue // skip if it happens to collide
			}
			clear(params)
			if got := r.find("GET", unregistered, params); got != nil {
				t.Fatalf("iter %d: find(GET, %q) returned non-nil for unregistered path", i, unregistered)
			}
		}
	}
}

// TestRouterParamFindProperty verifies that for any registered param route
// `/prefix/:paramName`, find returns the handler and params[paramName] equals
// the actual path segment value.
func TestRouterParamFindProperty(t *testing.T) {
	const iterations = 500
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < iterations; i++ {
		r := newRouter()
		params := make(map[string]string, 4)

		// Generate a random prefix and param name
		prefix := "/" + randomAlphaSegment(rng, 1, 6)
		paramName := randomAlphaSegment(rng, 2, 8)
		pattern := prefix + "/:" + paramName

		h := []HandlerFunc{dummyHandler()}
		r.addRoute("GET", pattern, h)

		// Generate a random value and verify it's extracted correctly
		value := randomAlphaSegment(rng, 1, 10)
		requestPath := prefix + "/" + value

		clear(params)
		got := r.find("GET", requestPath, params)
		if got == nil {
			t.Fatalf("iter %d: find(GET, %q) returned nil for pattern %q", i, requestPath, pattern)
		}
		if params[paramName] != value {
			t.Fatalf("iter %d: params[%q] = %q, want %q (pattern=%q path=%q)",
				i, paramName, params[paramName], value, pattern, requestPath)
		}
	}
}
