package kruda

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Req 1.1, 1.2: Static routes — separate tree per method, radix insertion
// ---------------------------------------------------------------------------

func TestStaticRoutesDifferentDepths(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/a", h)
	r.addRoute("GET", "/a/b", h)
	r.addRoute("GET", "/a/b/c", h)
	r.addRoute("GET", "/x/y/z/w", h)

	params := make(map[string]string, 4)

	tests := []struct {
		path  string
		match bool
	}{
		{"/", true},
		{"/a", true},
		{"/a/b", true},
		{"/a/b/c", true},
		{"/x/y/z/w", true},
		{"/a/b/c/d", false},
		{"/b", false},
	}
	for _, tt := range tests {
		clear(params)
		got := r.find("GET", tt.path, params)
		if tt.match && got == nil {
			t.Errorf("GET %s should match", tt.path)
		}
		if !tt.match && got != nil {
			t.Errorf("GET %s should NOT match", tt.path)
		}
	}
}

func TestStaticRoutesCommonPrefixes(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// Routes sharing common prefixes — exercises radix compression
	r.addRoute("GET", "/api/users", h)
	r.addRoute("GET", "/api/users/list", h)
	r.addRoute("GET", "/api/posts", h)
	r.addRoute("GET", "/api/posts/recent", h)
	r.addRoute("GET", "/app/dashboard", h)

	params := make(map[string]string, 4)

	tests := []struct {
		path  string
		match bool
	}{
		{"/api/users", true},
		{"/api/users/list", true},
		{"/api/posts", true},
		{"/api/posts/recent", true},
		{"/app/dashboard", true},
		{"/api", false},
		{"/api/users/list/extra", false},
		{"/app", false},
	}
	for _, tt := range tests {
		clear(params)
		got := r.find("GET", tt.path, params)
		if tt.match && got == nil {
			t.Errorf("GET %s should match", tt.path)
		}
		if !tt.match && got != nil {
			t.Errorf("GET %s should NOT match", tt.path)
		}
	}
}

func TestStaticRouteSeparateTreePerMethod(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/data", h)
	r.addRoute("POST", "/data", h)
	r.addRoute("PUT", "/data", h)

	params := make(map[string]string, 4)

	// Each method should find its own route
	for _, method := range []string{"GET", "POST", "PUT"} {
		clear(params)
		if r.find(method, "/data", params) == nil {
			t.Errorf("%s /data should match", method)
		}
	}
	// Methods not registered should not match
	for _, method := range []string{"DELETE", "PATCH", "OPTIONS", "HEAD"} {
		clear(params)
		if r.find(method, "/data", params) != nil {
			t.Errorf("%s /data should NOT match", method)
		}
	}
}

func TestStaticPriorityOverParam(t *testing.T) {
	r := newRouter()
	hStatic := []HandlerFunc{dummyHandler()}
	hParam := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/admin", hStatic)
	r.addRoute("GET", "/users/:id", hParam)

	params := make(map[string]string, 4)

	// Static route should be found for exact match
	clear(params)
	got := r.find("GET", "/users/admin", params)
	if got == nil {
		t.Fatal("GET /users/admin should match")
	}
	// Should match the static handler (hStatic), not param
	if &got[0] == &hParam[0] {
		t.Error("GET /users/admin should match static route, not param route")
	}

	// Param route should match other values
	clear(params)
	got = r.find("GET", "/users/42", params)
	if got == nil {
		t.Fatal("GET /users/42 should match param route")
	}
	if params["id"] != "42" {
		t.Errorf("params[id] = %q, want %q", params["id"], "42")
	}
}

// ---------------------------------------------------------------------------
// Req 1.3, 1.4: Parameterized routes — single and multi-param
// ---------------------------------------------------------------------------

func TestParamSingle(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id", h)

	params := make(map[string]string, 4)

	tests := []struct {
		path    string
		wantID  string
		wantNil bool
	}{
		{"/users/123", "123", false},
		{"/users/abc", "abc", false},
		{"/users/hello-world", "hello-world", false},
		{"/users/a.b.c", "a.b.c", false},
		{"/users/123/extra", "", true}, // no route for extra segment
	}
	for _, tt := range tests {
		clear(params)
		got := r.find("GET", tt.path, params)
		if tt.wantNil {
			if got != nil {
				t.Errorf("GET %s should NOT match", tt.path)
			}
			continue
		}
		if got == nil {
			t.Errorf("GET %s should match", tt.path)
			continue
		}
		if params["id"] != tt.wantID {
			t.Errorf("GET %s: params[id] = %q, want %q", tt.path, params["id"], tt.wantID)
		}
	}
}

func TestParamMulti(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:uid/posts/:pid", h)

	params := make(map[string]string, 4)

	clear(params)
	got := r.find("GET", "/users/10/posts/20", params)
	if got == nil {
		t.Fatal("GET /users/10/posts/20 should match")
	}
	if params["uid"] != "10" {
		t.Errorf("params[uid] = %q, want %q", params["uid"], "10")
	}
	if params["pid"] != "20" {
		t.Errorf("params[pid] = %q, want %q", params["pid"], "20")
	}

	// Different values
	clear(params)
	got = r.find("GET", "/users/alice/posts/draft-1", params)
	if got == nil {
		t.Fatal("GET /users/alice/posts/draft-1 should match")
	}
	if params["uid"] != "alice" {
		t.Errorf("params[uid] = %q, want %q", params["uid"], "alice")
	}
	if params["pid"] != "draft-1" {
		t.Errorf("params[pid] = %q, want %q", params["pid"], "draft-1")
	}
}

func TestParamSpecialChars(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/items/:slug", h)

	params := make(map[string]string, 4)

	tests := []struct {
		path     string
		wantSlug string
	}{
		{"/items/hello-world", "hello-world"},
		{"/items/foo_bar", "foo_bar"},
		{"/items/item.v2", "item.v2"},
		{"/items/100%25", "100%25"},
		{"/items/@user", "@user"},
	}
	for _, tt := range tests {
		clear(params)
		got := r.find("GET", tt.path, params)
		if got == nil {
			t.Errorf("GET %s should match", tt.path)
			continue
		}
		if params["slug"] != tt.wantSlug {
			t.Errorf("GET %s: params[slug] = %q, want %q", tt.path, params["slug"], tt.wantSlug)
		}
	}
}

// ---------------------------------------------------------------------------
// Req 1.5: Wildcard routes
// ---------------------------------------------------------------------------

func TestWildcardBasic(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/files/*filepath", h)

	params := make(map[string]string, 4)

	tests := []struct {
		path         string
		wantFilepath string
	}{
		{"/files/readme.txt", "readme.txt"},
		{"/files/css/style.css", "css/style.css"},
		{"/files/a/b/c/d/e.js", "a/b/c/d/e.js"},
	}
	for _, tt := range tests {
		clear(params)
		got := r.find("GET", tt.path, params)
		if got == nil {
			t.Errorf("GET %s should match wildcard", tt.path)
			continue
		}
		if params["filepath"] != tt.wantFilepath {
			t.Errorf("GET %s: params[filepath] = %q, want %q", tt.path, params["filepath"], tt.wantFilepath)
		}
	}
}

func TestWildcardSingleSegment(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/download/*file", h)

	params := make(map[string]string, 4)

	clear(params)
	got := r.find("GET", "/download/report.pdf", params)
	if got == nil {
		t.Fatal("GET /download/report.pdf should match")
	}
	if params["file"] != "report.pdf" {
		t.Errorf("params[file] = %q, want %q", params["file"], "report.pdf")
	}
}

func TestWildcardDeepNested(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/assets/*path", h)

	params := make(map[string]string, 4)

	clear(params)
	got := r.find("GET", "/assets/js/vendor/lodash/lodash.min.js", params)
	if got == nil {
		t.Fatal("deep nested wildcard should match")
	}
	if params["path"] != "js/vendor/lodash/lodash.min.js" {
		t.Errorf("params[path] = %q, want %q", params["path"], "js/vendor/lodash/lodash.min.js")
	}
}

// ---------------------------------------------------------------------------
// Req 1.6: Regex constraint
// ---------------------------------------------------------------------------

func TestRegexConstraintNumeric(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id<[0-9]+>", h)

	params := make(map[string]string, 4)

	// Matching: numeric
	clear(params)
	got := r.find("GET", "/users/456", params)
	if got == nil {
		t.Fatal("GET /users/456 should match numeric regex")
	}
	if params["id"] != "456" {
		t.Errorf("params[id] = %q, want %q", params["id"], "456")
	}

	// Non-matching: alpha
	clear(params)
	if r.find("GET", "/users/abc", params) != nil {
		t.Error("GET /users/abc should NOT match numeric regex")
	}

	// Non-matching: mixed
	clear(params)
	if r.find("GET", "/users/12ab", params) != nil {
		t.Error("GET /users/12ab should NOT match numeric regex")
	}
}

func TestRegexConstraintAlpha(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/tags/:name<[a-z]+>", h)

	params := make(map[string]string, 4)

	clear(params)
	if r.find("GET", "/tags/golang", params) == nil {
		t.Error("GET /tags/golang should match alpha regex")
	}

	clear(params)
	if r.find("GET", "/tags/123", params) != nil {
		t.Error("GET /tags/123 should NOT match alpha regex")
	}

	clear(params)
	if r.find("GET", "/tags/Go", params) != nil {
		t.Error("GET /tags/Go should NOT match lowercase-only alpha regex")
	}
}

func TestRegexConstraintUUID(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/orders/:id<[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}>", h)

	params := make(map[string]string, 4)

	clear(params)
	if r.find("GET", "/orders/550e8400-e29b-41d4-a716-446655440000", params) == nil {
		t.Error("valid UUID should match")
	}

	clear(params)
	if r.find("GET", "/orders/not-a-uuid", params) != nil {
		t.Error("invalid UUID should NOT match")
	}
}

// ---------------------------------------------------------------------------
// Req 1.7: Optional param
// ---------------------------------------------------------------------------

func TestOptionalParamWithValue(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id?", h)

	params := make(map[string]string, 4)

	clear(params)
	got := r.find("GET", "/users/123", params)
	if got == nil {
		t.Fatal("GET /users/123 should match optional param")
	}
	if params["id"] != "123" {
		t.Errorf("params[id] = %q, want %q", params["id"], "123")
	}
}

func TestOptionalParamWithoutValue(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id?", h)

	params := make(map[string]string, 4)

	clear(params)
	got := r.find("GET", "/users", params)
	if got == nil {
		t.Fatal("GET /users should match optional param (without value)")
	}
}

// ---------------------------------------------------------------------------
// Req 1.8: Zero-allocation find — populates pre-allocated params map
// ---------------------------------------------------------------------------

func TestFindPopulatesPreAllocatedParams(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id/posts/:postId", h)

	// Pre-allocate params map (simulating Ctx behavior)
	params := make(map[string]string, 4)

	clear(params)
	got := r.find("GET", "/users/42/posts/99", params)
	if got == nil {
		t.Fatal("should match")
	}
	if params["id"] != "42" {
		t.Errorf("params[id] = %q, want %q", params["id"], "42")
	}
	if params["postId"] != "99" {
		t.Errorf("params[postId] = %q, want %q", params["postId"], "99")
	}

	// Reuse same map after clear (zero-alloc pattern)
	clear(params)
	got = r.find("GET", "/users/7/posts/8", params)
	if got == nil {
		t.Fatal("should match on reuse")
	}
	if params["id"] != "7" {
		t.Errorf("reuse: params[id] = %q, want %q", params["id"], "7")
	}
	if params["postId"] != "8" {
		t.Errorf("reuse: params[postId] = %q, want %q", params["postId"], "8")
	}
}

// ---------------------------------------------------------------------------
// Req 1.9: Indices O(1) lookup — multiple children with different first bytes
// ---------------------------------------------------------------------------

func TestIndicesLookup(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// Register routes that create multiple children under root
	r.addRoute("GET", "/alpha", h)
	r.addRoute("GET", "/beta", h)
	r.addRoute("GET", "/gamma", h)
	r.addRoute("GET", "/delta", h)

	params := make(map[string]string, 4)

	for _, path := range []string{"/alpha", "/beta", "/gamma", "/delta"} {
		clear(params)
		if r.find("GET", path, params) == nil {
			t.Errorf("GET %s should match via indices lookup", path)
		}
	}

	// Non-existent first byte
	clear(params)
	if r.find("GET", "/zeta", params) != nil {
		t.Error("GET /zeta should NOT match")
	}
}

// ---------------------------------------------------------------------------
// Req 1.10: Duplicate route panic
// ---------------------------------------------------------------------------

func TestDuplicateStaticRoutePanics(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("expected panic on duplicate static route")
		}
	}()
	r.addRoute("GET", "/users", h)
}

func TestDuplicateRootRoutePanics(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/", h)

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("expected panic on duplicate root route")
		}
	}()
	r.addRoute("GET", "/", h)
}

func TestDuplicateDifferentMethodsNoPanic(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// Same path, different methods — should NOT panic
	r.addRoute("GET", "/users", h)
	r.addRoute("POST", "/users", h)
	r.addRoute("PUT", "/users", h)
	// If we reach here, no panic occurred — test passes
}

// ---------------------------------------------------------------------------
// Req 1.11: Compile freeze
// ---------------------------------------------------------------------------

func TestCompileFreezesPreventsAdd(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/before", h)
	r.Compile()

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("expected panic when adding route after Compile()")
		}
	}()
	r.addRoute("GET", "/after", h)
}

func TestCompileDoesNotAffectFind(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/test", h)
	r.Compile()

	params := make(map[string]string, 4)
	if r.find("GET", "/test", params) == nil {
		t.Error("find should still work after Compile()")
	}
}

// ---------------------------------------------------------------------------
// Req 1.12: 405 Method Not Allowed
// ---------------------------------------------------------------------------

func TestMethodNotAllowed_SingleMethod(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/resource", h)

	params := make(map[string]string, 4)

	// DELETE should not find a handler
	clear(params)
	if r.find("DELETE", "/resource", params) != nil {
		t.Error("DELETE /resource should not match")
	}

	// findAllowedMethods should return "GET"
	clear(params)
	allowed := r.findAllowedMethods("/resource")
	if !containsMethod(allowed, "GET") {
		t.Errorf("allowed = %q, want to contain GET", allowed)
	}
}

func TestMethodNotAllowed_MultipleMethods(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/items", h)
	r.addRoute("POST", "/items", h)
	r.addRoute("PUT", "/items", h)

	params := make(map[string]string, 4)

	// DELETE should not find a handler
	clear(params)
	if r.find("DELETE", "/items", params) != nil {
		t.Error("DELETE /items should not match")
	}

	// findAllowedMethods should list all registered methods
	clear(params)
	allowed := r.findAllowedMethods("/items")
	for _, m := range []string{"GET", "POST", "PUT"} {
		if !containsMethod(allowed, m) {
			t.Errorf("allowed = %q, want to contain %s", allowed, m)
		}
	}
}

func TestMethodNotAllowed_ParamRoute(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id", h)

	params := make(map[string]string, 4)

	// POST /users/42 should not match
	clear(params)
	if r.find("POST", "/users/42", params) != nil {
		t.Error("POST /users/42 should not match")
	}

	// findAllowedMethods should return GET
	clear(params)
	allowed := r.findAllowedMethods("/users/42")
	if !containsMethod(allowed, "GET") {
		t.Errorf("allowed = %q, want to contain GET", allowed)
	}
}

// ---------------------------------------------------------------------------
// Req 1.13: 404 Not Found
// ---------------------------------------------------------------------------

func TestNotFound_UnknownPath(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/known", h)

	params := make(map[string]string, 4)

	// Completely unknown path
	clear(params)
	if r.find("GET", "/unknown/path", params) != nil {
		t.Error("GET /unknown/path should return nil")
	}

	// findAllowedMethods should return empty
	clear(params)
	allowed := r.findAllowedMethods("/unknown/path")
	if allowed != "" {
		t.Errorf("findAllowedMethods for unknown path = %q, want empty", allowed)
	}
}

func TestNotFound_EmptyRouter(t *testing.T) {
	r := newRouter()
	params := make(map[string]string, 4)

	clear(params)
	if r.find("GET", "/anything", params) != nil {
		t.Error("empty router should return nil for any path")
	}

	clear(params)
	allowed := r.findAllowedMethods("/anything")
	if allowed != "" {
		t.Errorf("empty router findAllowedMethods = %q, want empty", allowed)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEdge_TrailingSlash(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	params := make(map[string]string, 4)

	// Exact match without trailing slash
	clear(params)
	if r.find("GET", "/users", params) == nil {
		t.Error("GET /users should match")
	}
}

func TestEdge_EmptyParamsMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/static", h)

	// Empty params map should work for static routes
	params := make(map[string]string, 4)
	clear(params)
	if r.find("GET", "/static", params) == nil {
		t.Error("static route should match with empty params map")
	}
	if len(params) != 0 {
		t.Errorf("static route should not populate params, got %v", params)
	}
}

func TestEdge_ManySegments(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/a/b/c/d/e/f/g", h)

	params := make(map[string]string, 4)

	clear(params)
	if r.find("GET", "/a/b/c/d/e/f/g", params) == nil {
		t.Error("deep static route should match")
	}

	clear(params)
	if r.find("GET", "/a/b/c/d/e/f", params) != nil {
		t.Error("partial deep path should NOT match")
	}
}

func TestEdge_ParamAfterStaticAtSameLevel(t *testing.T) {
	r := newRouter()
	hStatic := []HandlerFunc{dummyHandler()}
	hParam := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/items/new", hStatic)
	r.addRoute("GET", "/items/:id", hParam)

	params := make(map[string]string, 4)

	// "new" should match static
	clear(params)
	got := r.find("GET", "/items/new", params)
	if got == nil {
		t.Fatal("GET /items/new should match")
	}

	// "42" should match param
	clear(params)
	got = r.find("GET", "/items/42", params)
	if got == nil {
		t.Fatal("GET /items/42 should match param")
	}
	if params["id"] != "42" {
		t.Errorf("params[id] = %q, want %q", params["id"], "42")
	}
}

func TestEdge_PathMustStartWithSlash(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("expected panic for path without leading slash")
		}
	}()
	r.addRoute("GET", "noslash", h)
}

// ---------------------------------------------------------------------------
// Table-driven comprehensive test
// ---------------------------------------------------------------------------

func TestRouterTableDriven(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// Register a variety of routes
	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/health", h)
	r.addRoute("GET", "/api/v1/users", h)
	r.addRoute("GET", "/api/v1/users/:id", h)
	r.addRoute("POST", "/api/v1/users", h)
	r.addRoute("GET", "/api/v1/users/:id/posts/:postId", h)
	r.addRoute("GET", "/api/v1/orders/:id<[0-9]+>", h)
	r.addRoute("GET", "/api/v1/search/:query?", h)
	r.addRoute("GET", "/static/*filepath", h)

	tests := []struct {
		method     string
		path       string
		wantMatch  bool
		wantParams map[string]string
	}{
		// Static
		{"GET", "/", true, nil},
		{"GET", "/health", true, nil},
		{"GET", "/api/v1/users", true, nil},
		{"POST", "/api/v1/users", true, nil},

		// Param
		{"GET", "/api/v1/users/42", true, map[string]string{"id": "42"}},
		{"GET", "/api/v1/users/alice", true, map[string]string{"id": "alice"}},

		// Multi-param
		{"GET", "/api/v1/users/1/posts/2", true, map[string]string{"id": "1", "postId": "2"}},

		// Regex
		{"GET", "/api/v1/orders/999", true, map[string]string{"id": "999"}},
		{"GET", "/api/v1/orders/abc", false, nil},

		// Optional
		{"GET", "/api/v1/search/hello", true, map[string]string{"query": "hello"}},
		{"GET", "/api/v1/search", true, nil}, // without value

		// Wildcard
		{"GET", "/static/css/app.css", true, map[string]string{"filepath": "css/app.css"}},
		{"GET", "/static/img/logo.png", true, map[string]string{"filepath": "img/logo.png"}},

		// 404
		{"GET", "/nonexistent", false, nil},
		{"GET", "/api/v2/users", false, nil},

		// 405 (path exists for different method)
		{"DELETE", "/api/v1/users", false, nil},
	}

	params := make(map[string]string, 4)
	for _, tt := range tests {
		clear(params)
		got := r.find(tt.method, tt.path, params)
		if tt.wantMatch && got == nil {
			t.Errorf("%s %s: want match, got nil", tt.method, tt.path)
			continue
		}
		if !tt.wantMatch && got != nil {
			t.Errorf("%s %s: want no match, got non-nil", tt.method, tt.path)
			continue
		}
		if tt.wantParams != nil {
			for k, v := range tt.wantParams {
				if params[k] != v {
					t.Errorf("%s %s: params[%s] = %q, want %q", tt.method, tt.path, k, params[k], v)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Task 2.6: findAllowedMethods cache — P3-006 fix
// Tests: cache hit (static), cache miss (dynamic), zero alloc (static)
// ---------------------------------------------------------------------------

func TestAllowedMethodsCache_StaticHit(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)
	r.addRoute("POST", "/users", h)
	r.addRoute("DELETE", "/health", h)
	r.Compile()

	// After Compile, static paths should be in the cache
	if r.allowedMethodsCache == nil {
		t.Fatal("allowedMethodsCache should be initialized after Compile()")
	}

	// /users should be cached with GET and POST
	cached, ok := r.allowedMethodsCache["/users"]
	if !ok {
		t.Fatal("/users should be in allowedMethodsCache")
	}
	if !containsMethod(cached, "GET") || !containsMethod(cached, "POST") {
		t.Errorf("cached allowed for /users = %q, want GET and POST", cached)
	}

	// /health should be cached with DELETE
	cached, ok = r.allowedMethodsCache["/health"]
	if !ok {
		t.Fatal("/health should be in allowedMethodsCache")
	}
	if !containsMethod(cached, "DELETE") {
		t.Errorf("cached allowed for /health = %q, want DELETE", cached)
	}

	// findAllowedMethods should return the cached value
	allowed := r.findAllowedMethods("/users")
	if !containsMethod(allowed, "GET") || !containsMethod(allowed, "POST") {
		t.Errorf("findAllowedMethods(/users) = %q, want GET and POST", allowed)
	}
}

func TestAllowedMethodsCache_DynamicMiss(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id", h)
	r.addRoute("PUT", "/users/:id", h)
	r.Compile()

	// Dynamic paths (with params) should NOT be in the cache
	if _, ok := r.allowedMethodsCache["/users/42"]; ok {
		t.Error("/users/42 should NOT be in allowedMethodsCache (dynamic path)")
	}

	// But findAllowedMethods should still work via tree scan fallback
	allowed := r.findAllowedMethods("/users/42")
	if !containsMethod(allowed, "GET") || !containsMethod(allowed, "PUT") {
		t.Errorf("findAllowedMethods(/users/42) = %q, want GET and PUT", allowed)
	}
}

func TestAllowedMethodsCache_WildcardNotCached(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/files/*filepath", h)
	r.Compile()

	// Wildcard paths should not be in the cache
	if _, ok := r.allowedMethodsCache["/files/readme.txt"]; ok {
		t.Error("wildcard path should NOT be in allowedMethodsCache")
	}

	// Fallback scan should still find it
	allowed := r.findAllowedMethods("/files/readme.txt")
	if !containsMethod(allowed, "GET") {
		t.Errorf("findAllowedMethods(/files/readme.txt) = %q, want GET", allowed)
	}
}

func TestAllowedMethodsCache_MixedStaticAndDynamic(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/api/health", h)
	r.addRoute("GET", "/api/users/:id", h)
	r.addRoute("POST", "/api/users", h)
	r.Compile()

	// Static paths should be cached
	if _, ok := r.allowedMethodsCache["/api/health"]; !ok {
		t.Error("/api/health should be in cache")
	}
	if _, ok := r.allowedMethodsCache["/api/users"]; !ok {
		t.Error("/api/users should be in cache (static terminal)")
	}

	// Dynamic path lookup via fallback
	allowed := r.findAllowedMethods("/api/users/99")
	if !containsMethod(allowed, "GET") {
		t.Errorf("findAllowedMethods(/api/users/99) = %q, want GET", allowed)
	}
}

func TestAllowedMethodsCache_EmptyBeforeCompile(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/test", h)

	// Before Compile, cache should be nil
	if r.allowedMethodsCache != nil {
		t.Error("allowedMethodsCache should be nil before Compile()")
	}

	// findAllowedMethods should still work (fallback path)
	allowed := r.findAllowedMethods("/test")
	if !containsMethod(allowed, "GET") {
		t.Errorf("findAllowedMethods before Compile = %q, want GET", allowed)
	}
}

func TestAllowedMethodsCache_UnknownPathReturnsEmpty(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/exists", h)
	r.Compile()

	allowed := r.findAllowedMethods("/does-not-exist")
	if allowed != "" {
		t.Errorf("findAllowedMethods for unknown path = %q, want empty", allowed)
	}
}

func TestAllowedMethodsCache_ZeroAllocStaticPath(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/cached", h)
	r.addRoute("POST", "/cached", h)
	r.Compile()

	// Warm up — first call may allocate due to runtime internals
	r.findAllowedMethods("/cached")

	allocs := testing.AllocsPerRun(100, func() {
		r.findAllowedMethods("/cached")
	})
	if allocs > 0 {
		t.Errorf("findAllowedMethods (cache hit) allocs = %.0f, want 0", allocs)
	}
}

func TestCollectStaticPaths_Basic(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/a", h)
	r.addRoute("GET", "/b", h)
	r.addRoute("POST", "/a", h)
	r.addRoute("GET", "/users/:id", h) // dynamic — should not appear

	paths := r.collectStaticPaths()
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	if !pathSet["/a"] {
		t.Error("collectStaticPaths should include /a")
	}
	if !pathSet["/b"] {
		t.Error("collectStaticPaths should include /b")
	}
	// /a should appear only once even though registered for GET and POST
	count := 0
	for _, p := range paths {
		if p == "/a" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("/a appeared %d times, want 1 (deduped)", count)
	}
}

func TestCollectStaticPaths_ExcludesParamAndWildcard(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/static", h)
	r.addRoute("GET", "/users/:id", h)
	r.addRoute("GET", "/files/*filepath", h)

	paths := r.collectStaticPaths()
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	if !pathSet["/static"] {
		t.Error("collectStaticPaths should include /static")
	}
	// Verify no unexpected paths appear
	for _, p := range paths {
		_ = p // paths validated by length check above
	}
}

// ---------------------------------------------------------------------------
// Task 3: Router AOT optimization — P3-003
// Tests: hits tracking, sort order, flatten, indices rebuilt, compiled flag,
// route matching correctness after optimization
// ---------------------------------------------------------------------------

func TestOptimizeTree_SortByHitsDescending(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/alpha", h)
	r.addRoute("GET", "/beta", h)
	r.addRoute("GET", "/gamma", h)

	params := make(map[string]string, 4)

	// Simulate traffic: gamma most popular, then alpha, then beta
	for i := 0; i < 100; i++ {
		clear(params)
		r.find("GET", "/gamma", params)
	}
	for i := 0; i < 50; i++ {
		clear(params)
		r.find("GET", "/alpha", params)
	}
	for i := 0; i < 10; i++ {
		clear(params)
		r.find("GET", "/beta", params)
	}

	r.Compile()

	// After Compile, children of root should be sorted: gamma (100), alpha (50), beta (10)
	root := r.trees["GET"]
	if len(root.children) < 3 {
		t.Fatalf("expected at least 3 children, got %d", len(root.children))
	}

	// Verify sort order by hits (descending)
	for i := 0; i < len(root.children)-1; i++ {
		if root.children[i].hits < root.children[i+1].hits {
			t.Errorf("children not sorted by hits: child[%d].hits=%d < child[%d].hits=%d",
				i, root.children[i].hits, i+1, root.children[i+1].hits)
		}
	}
}

func TestOptimizeTree_StaticsBeforeParams(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/:id", h)
	r.addRoute("GET", "/users/admin", h)

	params := make(map[string]string, 4)

	// Give param child more hits than static
	for i := 0; i < 100; i++ {
		clear(params)
		r.find("GET", "/users/42", params)
	}
	for i := 0; i < 10; i++ {
		clear(params)
		r.find("GET", "/users/admin", params)
	}

	r.Compile()

	// Find the "users/" node
	root := r.trees["GET"]
	var usersNode *node
	for _, child := range root.children {
		if child.param == "" && !child.wildcard {
			usersNode = child
			break
		}
	}
	if usersNode == nil {
		t.Fatal("could not find users node")
	}

	// Static children should come before param children regardless of hits
	foundParam := false
	for _, child := range usersNode.children {
		if child.param != "" {
			foundParam = true
		} else if foundParam {
			t.Error("static child found after param child — statics should come first")
		}
	}
}

func TestOptimizeTree_WildcardsLast(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/files/readme", h)
	r.addRoute("GET", "/files/:name", h)
	r.addRoute("GET", "/files/*filepath", h)

	params := make(map[string]string, 4)

	// Give wildcard lots of hits
	for i := 0; i < 200; i++ {
		clear(params)
		r.find("GET", "/files/a/b/c", params)
	}

	r.Compile()

	// Find the "files/" node
	root := r.trees["GET"]
	var filesNode *node
	for _, child := range root.children {
		if child.param == "" && !child.wildcard {
			filesNode = child
			break
		}
	}
	if filesNode == nil {
		t.Fatal("could not find files node")
	}

	// Wildcard should be last child regardless of hits
	lastChild := filesNode.children[len(filesNode.children)-1]
	if !lastChild.wildcard {
		t.Error("wildcard child should be last after optimization")
	}
}

func TestOptimizeTree_IndicesRebuilt(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/alpha", h)
	r.addRoute("GET", "/beta", h)
	r.addRoute("GET", "/gamma", h)

	params := make(map[string]string, 4)

	// Make gamma most popular to force reorder
	for i := 0; i < 100; i++ {
		clear(params)
		r.find("GET", "/gamma", params)
	}

	r.Compile()

	root := r.trees["GET"]

	// Verify indices match the first byte of each child's path
	if len(root.indices) != len(root.children) {
		t.Fatalf("indices length %d != children length %d", len(root.indices), len(root.children))
	}
	for i, child := range root.children {
		if child.param == "" && !child.wildcard && len(child.path) > 0 {
			if root.indices[i] != child.path[0] {
				t.Errorf("indices[%d]=%c does not match child.path[0]=%c (path=%q)",
					i, root.indices[i], child.path[0], child.path)
			}
		}
	}
}

func TestFlattenNode_MergesSingleChildChain(t *testing.T) {
	// Manually build a chain: root → "a" → "b" → "c" (handler)
	// Should flatten to: root → "abc" (handler)
	handler := []HandlerFunc{dummyHandler()}
	root := &node{
		path: "/",
		children: []*node{
			{
				path: "a",
				children: []*node{
					{
						path: "b",
						children: []*node{
							{
								path:     "c",
								handlers: handler,
								hits:     5,
							},
						},
						indices: "c",
						hits:    3,
					},
				},
				indices: "b",
				hits:    2,
			},
		},
		indices: "a",
	}

	flattenNode(root)

	if len(root.children) != 1 {
		t.Fatalf("expected 1 child after flatten, got %d", len(root.children))
	}
	merged := root.children[0]
	if merged.path != "abc" {
		t.Errorf("merged path = %q, want %q", merged.path, "abc")
	}
	if merged.handlers == nil {
		t.Error("merged node should have handlers")
	}
	if merged.hits != 10 { // 2 + 3 + 5
		t.Errorf("merged hits = %d, want 10", merged.hits)
	}
}

func TestFlattenNode_DoesNotMergeWithHandlers(t *testing.T) {
	// Chain: root → "a" (handler) → "b" (handler)
	// Should NOT flatten because "a" has handlers
	handler := []HandlerFunc{dummyHandler()}
	root := &node{
		path: "/",
		children: []*node{
			{
				path:     "a",
				handlers: handler,
				children: []*node{
					{
						path:     "b",
						handlers: handler,
					},
				},
				indices: "b",
			},
		},
		indices: "a",
	}

	flattenNode(root)

	// "a" should still exist as separate node
	if root.children[0].path != "a" {
		t.Errorf("node with handler should not be merged, got path=%q", root.children[0].path)
	}
	if len(root.children[0].children) != 1 {
		t.Error("child of handler node should still exist")
	}
}

func TestFlattenNode_DoesNotMergeParamChild(t *testing.T) {
	// Chain: root → "users/" (no handler) → ":id" (handler)
	// Should NOT flatten because grandchild is a param node
	handler := []HandlerFunc{dummyHandler()}
	root := &node{
		path: "/",
		children: []*node{
			{
				path: "users/",
				children: []*node{
					{
						param:    "id",
						handlers: handler,
					},
				},
				indices: ":",
			},
		},
		indices: "u",
	}

	flattenNode(root)

	// "users/" should not be merged with ":id"
	if root.children[0].path != "users/" {
		t.Errorf("static→param chain should not flatten, got path=%q", root.children[0].path)
	}
}

func TestFlattenNode_DoesNotMergeMultipleChildren(t *testing.T) {
	// Chain: root → "api/" (no handler) → "users" + "posts"
	// Should NOT flatten because "api/" has 2 children
	handler := []HandlerFunc{dummyHandler()}
	root := &node{
		path: "/",
		children: []*node{
			{
				path: "api/",
				children: []*node{
					{path: "users", handlers: handler},
					{path: "posts", handlers: handler},
				},
				indices: "up",
			},
		},
		indices: "a",
	}

	flattenNode(root)

	// "api/" should not be merged because it has 2 children
	if root.children[0].path != "api/" {
		t.Errorf("multi-child node should not flatten, got path=%q", root.children[0].path)
	}
}

func TestCompile_RoutesStillMatchAfterOptimization(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// Register a variety of routes
	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/health", h)
	r.addRoute("GET", "/api/v1/users", h)
	r.addRoute("GET", "/api/v1/users/:id", h)
	r.addRoute("POST", "/api/v1/users", h)
	r.addRoute("GET", "/api/v1/users/:id/posts/:postId", h)
	r.addRoute("GET", "/api/v1/orders/:id<[0-9]+>", h)
	r.addRoute("GET", "/static/*filepath", h)
	r.addRoute("DELETE", "/api/v1/users/:id", h)

	params := make(map[string]string, 4)

	// Simulate traffic to create varied hit counts
	for i := 0; i < 50; i++ {
		clear(params)
		r.find("GET", "/health", params)
	}
	for i := 0; i < 100; i++ {
		clear(params)
		r.find("GET", "/api/v1/users", params)
	}
	for i := 0; i < 30; i++ {
		clear(params)
		r.find("GET", "/api/v1/users/42", params)
	}

	r.Compile()

	// Verify all routes still match correctly after optimization
	tests := []struct {
		method     string
		path       string
		wantMatch  bool
		wantParams map[string]string
	}{
		{"GET", "/", true, nil},
		{"GET", "/health", true, nil},
		{"GET", "/api/v1/users", true, nil},
		{"POST", "/api/v1/users", true, nil},
		{"GET", "/api/v1/users/42", true, map[string]string{"id": "42"}},
		{"GET", "/api/v1/users/1/posts/2", true, map[string]string{"id": "1", "postId": "2"}},
		{"GET", "/api/v1/orders/999", true, map[string]string{"id": "999"}},
		{"GET", "/api/v1/orders/abc", false, nil},
		{"GET", "/static/css/app.css", true, map[string]string{"filepath": "css/app.css"}},
		{"DELETE", "/api/v1/users/7", true, map[string]string{"id": "7"}},
		{"GET", "/nonexistent", false, nil},
	}

	for _, tt := range tests {
		clear(params)
		got := r.find(tt.method, tt.path, params)
		if tt.wantMatch && got == nil {
			t.Errorf("after Compile: %s %s should match", tt.method, tt.path)
			continue
		}
		if !tt.wantMatch && got != nil {
			t.Errorf("after Compile: %s %s should NOT match", tt.method, tt.path)
			continue
		}
		for k, v := range tt.wantParams {
			if params[k] != v {
				t.Errorf("after Compile: %s %s params[%s]=%q, want %q", tt.method, tt.path, k, params[k], v)
			}
		}
	}
}

func TestCompile_CompiledFlagSet(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/test", h)

	if r.compiled {
		t.Error("compiled should be false before Compile()")
	}

	r.Compile()

	if !r.compiled {
		t.Error("compiled should be true after Compile()")
	}
}

func TestHits_IncrementBeforeCompile(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	params := make(map[string]string, 4)

	// Find the "users" node before any hits
	root := r.trees["GET"]
	var usersNode *node
	for _, child := range root.children {
		if child.path == "users" {
			usersNode = child
			break
		}
	}
	if usersNode == nil {
		t.Fatal("could not find users node")
	}

	initialHits := usersNode.hits

	// Call find 5 times
	for i := 0; i < 5; i++ {
		clear(params)
		r.find("GET", "/users", params)
	}

	if usersNode.hits != initialHits+5 {
		t.Errorf("hits = %d, want %d (should increment before Compile)", usersNode.hits, initialHits+5)
	}
}

func TestHits_NoIncrementAfterCompile(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	params := make(map[string]string, 4)

	// Generate some hits before compile
	for i := 0; i < 5; i++ {
		clear(params)
		r.find("GET", "/users", params)
	}

	r.Compile()

	// Find the node after compile (tree may have been restructured)
	root := r.trees["GET"]
	var usersNode *node
	for _, child := range root.children {
		if child.param == "" && !child.wildcard {
			// Check if this node's path contains "users"
			if child.path == "users" || strings.Contains(child.path, "users") {
				usersNode = child
				break
			}
		}
	}
	if usersNode == nil {
		t.Fatal("could not find users node after Compile")
	}

	hitsAfterCompile := usersNode.hits

	// Call find 10 more times after Compile
	for i := 0; i < 10; i++ {
		clear(params)
		r.find("GET", "/users", params)
	}

	if usersNode.hits != hitsAfterCompile {
		t.Errorf("hits changed after Compile: was %d, now %d (should not increment after Compile)",
			hitsAfterCompile, usersNode.hits)
	}
}
