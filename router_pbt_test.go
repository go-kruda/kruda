package kruda

// Property: For any registered static route, find(method, path) always
// returns the registered handler chain, and find returns nil for unregistered paths.
//
// For any registered param route `/x/:name`, find returns handler
// and params.get("name") equals the path segment.

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
		var params routeParams

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
			params.reset()
			if got := r.find("GET", path, &params); got == nil {
				t.Fatalf("iter %d: find(GET, %q) returned nil, want non-nil", i, path)
			}
		}

		// Generate random unregistered paths and verify they return nil
		for j := 0; j < 5; j++ {
			unregistered := randomStaticPath(rng, 1, 4)
			if registered[unregistered] {
				continue // skip if it happens to collide
			}
			params.reset()
			if got := r.find("GET", unregistered, &params); got != nil {
				t.Fatalf("iter %d: find(GET, %q) returned non-nil for unregistered path", i, unregistered)
			}
		}
	}
}

// TestRouterParamFindProperty verifies that for any registered param route
// `/prefix/:paramName`, find returns the handler and params.get(paramName) equals
// the actual path segment value.
func TestRouterParamFindProperty(t *testing.T) {
	const iterations = 500
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < iterations; i++ {
		r := newRouter()
		var params routeParams

		// Generate a random prefix and param name
		prefix := "/" + randomAlphaSegment(rng, 1, 6)
		paramName := randomAlphaSegment(rng, 2, 8)
		pattern := prefix + "/:" + paramName

		h := []HandlerFunc{dummyHandler()}
		r.addRoute("GET", pattern, h)

		// Generate a random value and verify it's extracted correctly
		value := randomAlphaSegment(rng, 1, 10)
		requestPath := prefix + "/" + value

		params.reset()
		got := r.find("GET", requestPath, &params)
		if got == nil {
			t.Fatalf("iter %d: find(GET, %q) returned nil for pattern %q", i, requestPath, pattern)
		}
		if params.get(paramName) != value {
			t.Fatalf("iter %d: params.get(%q) = %q, want %q (pattern=%q path=%q)",
				i, paramName, params.get(paramName), value, pattern, requestPath)
		}
	}
}

// Property: AOT optimization preserves route matching correctness.
// For any set of registered routes, Compile() (optimize + flatten)
// does not change which routes match or what params are extracted.

// TestPropertyAOTOptimizationPreservesRouteMatching generates random route sets,
// verifies matching before Compile, then verifies identical matching after Compile.
func TestPropertyAOTOptimizationPreservesRouteMatching(t *testing.T) {
	const iterations = 300
	rng := rand.New(rand.NewSource(12345))

	for iter := 0; iter < iterations; iter++ {
		r := newRouter()
		var params routeParams

		// Generate 2-15 unique routes (mix of static, param, wildcard)
		numRoutes := 2 + rng.Intn(14)
		type route struct {
			method string
			path   string
		}
		registered := make([]route, 0, numRoutes)
		seen := make(map[string]bool)

		methods := []string{"GET", "POST", "PUT", "DELETE"}

		for j := 0; j < numRoutes; j++ {
			method := methods[rng.Intn(len(methods))]
			path := randomMixedPath(rng)
			key := method + " " + path
			if seen[key] {
				continue
			}
			seen[key] = true

			h := []HandlerFunc{dummyHandler()}
			func() {
				defer func() { recover() }() // skip if duplicate/conflict
				r.addRoute(method, path, h)
				registered = append(registered, route{method, path})
			}()
		}

		if len(registered) == 0 {
			continue
		}

		// Generate test requests: registered paths + random values for params
		type testCase struct {
			method    string
			path      string
			wantMatch bool
			wantParam routeParams
		}
		var tests []testCase

		for _, rt := range registered {
			// Build a concrete request path from the pattern
			reqPath, expectedParams := buildRequestPath(rng, rt.path)
			tests = append(tests, testCase{
				method:    rt.method,
				path:      reqPath,
				wantMatch: true,
				wantParam: expectedParams,
			})
		}

		// Also add some random unregistered paths
		for j := 0; j < 3; j++ {
			unregistered := "/" + randomAlphaSegment(rng, 8, 12) + "/" + randomAlphaSegment(rng, 8, 12)
			tests = append(tests, testCase{
				method:    methods[rng.Intn(len(methods))],
				path:      unregistered,
				wantMatch: false, // might match by coincidence, so we just record
			})
		}

		// Record results BEFORE Compile
		type result struct {
			matched bool
			params  routeParams
		}
		beforeResults := make([]result, len(tests))
		for i, tc := range tests {
			params.reset()
			got := r.find(tc.method, tc.path, &params)
			beforeResults[i] = result{
				matched: got != nil,
				params:  params, // copy by value (fixed-size array)
			}
		}

		// Compile (triggers optimize + flatten)
		r.Compile()

		// Verify results AFTER Compile match BEFORE
		for i, tc := range tests {
			params.reset()
			got := r.find(tc.method, tc.path, &params)
			afterMatched := got != nil

			if afterMatched != beforeResults[i].matched {
				t.Fatalf("iter %d: %s %s match changed after Compile: before=%v after=%v",
					iter, tc.method, tc.path, beforeResults[i].matched, afterMatched)
			}

			if afterMatched {
				bp := &beforeResults[i].params
				for j := 0; j < bp.count; j++ {
					k, v := bp.items[j].Key, bp.items[j].Value
					if params.get(k) != v {
						t.Fatalf("iter %d: %s %s params.get(%s) changed after Compile: before=%q after=%q",
							iter, tc.method, tc.path, k, v, params.get(k))
					}
				}
			}
		}
	}
}

// randomMixedPath generates a random path that may include static, param, or wildcard segments.
func randomMixedPath(rng *rand.Rand) string {
	numSegments := 1 + rng.Intn(4)
	path := ""
	for i := 0; i < numSegments; i++ {
		roll := rng.Intn(10)
		switch {
		case roll < 7: // 70% static
			path += "/" + randomAlphaSegment(rng, 1, 6)
		case roll < 9: // 20% param
			path += "/:" + randomAlphaSegment(rng, 2, 5)
			// After a param, no more segments (to avoid complex conflicts)
			return path
		default: // 10% wildcard (must be last)
			path += "/*" + randomAlphaSegment(rng, 2, 5)
			return path
		}
	}
	return path
}

// buildRequestPath converts a route pattern into a concrete request path,
// replacing :param with random values and *wildcard with random paths.
func buildRequestPath(rng *rand.Rand, pattern string) (string, routeParams) {
	var params routeParams
	parts := splitPathParts(pattern)
	result := ""
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		if part[0] == '*' {
			// Wildcard — generate a random multi-segment value
			val := randomAlphaSegment(rng, 1, 4) + "/" + randomAlphaSegment(rng, 1, 4)
			params.set(part[1:], val)
			result += "/" + val
			break
		}
		if part[0] == ':' {
			name := part[1:]
			// Strip optional marker and regex
			if idx := indexByte(name, '<'); idx >= 0 {
				name = name[:idx]
			}
			if len(name) > 0 && name[len(name)-1] == '?' {
				name = name[:len(name)-1]
			}
			val := randomAlphaSegment(rng, 1, 6)
			params.set(name, val)
			result += "/" + val
		} else {
			result += "/" + part
		}
	}
	if result == "" {
		result = "/"
	}
	return result, params
}

func splitPathParts(path string) []string {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func copyMap(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
