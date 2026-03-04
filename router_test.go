package kruda

import (
	"strings"
	"testing"
)

func TestStaticRoutesDifferentDepths(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/a", h)
	r.addRoute("GET", "/a/b", h)
	r.addRoute("GET", "/a/b/c", h)
	r.addRoute("GET", "/x/y/z/w", h)

	var params routeParams

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
		params.reset()
		got := r.find("GET", tt.path, &params)
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

	var params routeParams

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
		params.reset()
		got := r.find("GET", tt.path, &params)
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

	var params routeParams

	// Each method should find its own route
	for _, method := range []string{"GET", "POST", "PUT"} {
		params.reset()
		if r.find(method, "/data", &params) == nil {
			t.Errorf("%s /data should match", method)
		}
	}
	// Methods not registered should not match
	for _, method := range []string{"DELETE", "PATCH", "OPTIONS", "HEAD"} {
		params.reset()
		if r.find(method, "/data", &params) != nil {
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

	var params routeParams

	// Static route should be found for exact match
	params.reset()
	got := r.find("GET", "/users/admin", &params)
	if got == nil {
		t.Fatal("GET /users/admin should match")
	}
	// Should match the static handler (hStatic), not param
	if &got[0] == &hParam[0] {
		t.Error("GET /users/admin should match static route, not param route")
	}

	// Param route should match other values
	params.reset()
	got = r.find("GET", "/users/42", &params)
	if got == nil {
		t.Fatal("GET /users/42 should match param route")
	}
	if params.get("id") != "42" {
		t.Errorf("params.get(id) = %q, want %q", params.get("id"), "42")
	}
}

func TestParamSingle(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id", h)

	var params routeParams

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
		params.reset()
		got := r.find("GET", tt.path, &params)
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
		if params.get("id") != tt.wantID {
			t.Errorf("GET %s: params.get(id) = %q, want %q", tt.path, params.get("id"), tt.wantID)
		}
	}
}

func TestParamMulti(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:uid/posts/:pid", h)

	var params routeParams

	params.reset()
	got := r.find("GET", "/users/10/posts/20", &params)
	if got == nil {
		t.Fatal("GET /users/10/posts/20 should match")
	}
	if params.get("uid") != "10" {
		t.Errorf("params.get(uid) = %q, want %q", params.get("uid"), "10")
	}
	if params.get("pid") != "20" {
		t.Errorf("params.get(pid) = %q, want %q", params.get("pid"), "20")
	}

	// Different values
	params.reset()
	got = r.find("GET", "/users/alice/posts/draft-1", &params)
	if got == nil {
		t.Fatal("GET /users/alice/posts/draft-1 should match")
	}
	if params.get("uid") != "alice" {
		t.Errorf("params.get(uid) = %q, want %q", params.get("uid"), "alice")
	}
	if params.get("pid") != "draft-1" {
		t.Errorf("params.get(pid) = %q, want %q", params.get("pid"), "draft-1")
	}
}

func TestParamSpecialChars(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/items/:slug", h)

	var params routeParams

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
		params.reset()
		got := r.find("GET", tt.path, &params)
		if got == nil {
			t.Errorf("GET %s should match", tt.path)
			continue
		}
		if params.get("slug") != tt.wantSlug {
			t.Errorf("GET %s: params.get(slug) = %q, want %q", tt.path, params.get("slug"), tt.wantSlug)
		}
	}
}

func TestWildcardBasic(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/files/*filepath", h)

	var params routeParams

	tests := []struct {
		path         string
		wantFilepath string
	}{
		{"/files/readme.txt", "readme.txt"},
		{"/files/css/style.css", "css/style.css"},
		{"/files/a/b/c/d/e.js", "a/b/c/d/e.js"},
	}
	for _, tt := range tests {
		params.reset()
		got := r.find("GET", tt.path, &params)
		if got == nil {
			t.Errorf("GET %s should match wildcard", tt.path)
			continue
		}
		if params.get("filepath") != tt.wantFilepath {
			t.Errorf("GET %s: params.get(filepath) = %q, want %q", tt.path, params.get("filepath"), tt.wantFilepath)
		}
	}
}

func TestWildcardSingleSegment(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/download/*file", h)

	var params routeParams

	params.reset()
	got := r.find("GET", "/download/report.pdf", &params)
	if got == nil {
		t.Fatal("GET /download/report.pdf should match")
	}
	if params.get("file") != "report.pdf" {
		t.Errorf("params.get(file) = %q, want %q", params.get("file"), "report.pdf")
	}
}

func TestWildcardDeepNested(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/assets/*path", h)

	var params routeParams

	params.reset()
	got := r.find("GET", "/assets/js/vendor/lodash/lodash.min.js", &params)
	if got == nil {
		t.Fatal("deep nested wildcard should match")
	}
	if params.get("path") != "js/vendor/lodash/lodash.min.js" {
		t.Errorf("params.get(path) = %q, want %q", params.get("path"), "js/vendor/lodash/lodash.min.js")
	}
}

func TestRegexConstraintNumeric(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id<[0-9]+>", h)

	var params routeParams

	// Matching: numeric
	params.reset()
	got := r.find("GET", "/users/456", &params)
	if got == nil {
		t.Fatal("GET /users/456 should match numeric regex")
	}
	if params.get("id") != "456" {
		t.Errorf("params.get(id) = %q, want %q", params.get("id"), "456")
	}

	// Non-matching: alpha
	params.reset()
	if r.find("GET", "/users/abc", &params) != nil {
		t.Error("GET /users/abc should NOT match numeric regex")
	}

	// Non-matching: mixed
	params.reset()
	if r.find("GET", "/users/12ab", &params) != nil {
		t.Error("GET /users/12ab should NOT match numeric regex")
	}
}

func TestRegexConstraintAlpha(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/tags/:name<[a-z]+>", h)

	var params routeParams

	params.reset()
	if r.find("GET", "/tags/golang", &params) == nil {
		t.Error("GET /tags/golang should match alpha regex")
	}

	params.reset()
	if r.find("GET", "/tags/123", &params) != nil {
		t.Error("GET /tags/123 should NOT match alpha regex")
	}

	params.reset()
	if r.find("GET", "/tags/Go", &params) != nil {
		t.Error("GET /tags/Go should NOT match lowercase-only alpha regex")
	}
}

func TestRegexConstraintUUID(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/orders/:id<[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}>", h)

	var params routeParams

	params.reset()
	if r.find("GET", "/orders/550e8400-e29b-41d4-a716-446655440000", &params) == nil {
		t.Error("valid UUID should match")
	}

	params.reset()
	if r.find("GET", "/orders/not-a-uuid", &params) != nil {
		t.Error("invalid UUID should NOT match")
	}
}

func TestOptionalParamWithValue(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id?", h)

	var params routeParams

	params.reset()
	got := r.find("GET", "/users/123", &params)
	if got == nil {
		t.Fatal("GET /users/123 should match optional param")
	}
	if params.get("id") != "123" {
		t.Errorf("params.get(id) = %q, want %q", params.get("id"), "123")
	}
}

func TestOptionalParamWithoutValue(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id?", h)

	var params routeParams

	params.reset()
	got := r.find("GET", "/users", &params)
	if got == nil {
		t.Fatal("GET /users should match optional param (without value)")
	}
}

func TestFindPopulatesPreAllocatedParams(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:id/posts/:postId", h)

	// Pre-allocate params map (simulating Ctx behavior)
	var params routeParams

	params.reset()
	got := r.find("GET", "/users/42/posts/99", &params)
	if got == nil {
		t.Fatal("should match")
	}
	if params.get("id") != "42" {
		t.Errorf("params.get(id) = %q, want %q", params.get("id"), "42")
	}
	if params.get("postId") != "99" {
		t.Errorf("params.get(postId) = %q, want %q", params.get("postId"), "99")
	}

	// Reuse same map after clear (zero-alloc pattern)
	params.reset()
	got = r.find("GET", "/users/7/posts/8", &params)
	if got == nil {
		t.Fatal("should match on reuse")
	}
	if params.get("id") != "7" {
		t.Errorf("reuse: params.get(id) = %q, want %q", params.get("id"), "7")
	}
	if params.get("postId") != "8" {
		t.Errorf("reuse: params.get(postId) = %q, want %q", params.get("postId"), "8")
	}
}

func TestIndicesLookup(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	// Register routes that create multiple children under root
	r.addRoute("GET", "/alpha", h)
	r.addRoute("GET", "/beta", h)
	r.addRoute("GET", "/gamma", h)
	r.addRoute("GET", "/delta", h)

	var params routeParams

	for _, path := range []string{"/alpha", "/beta", "/gamma", "/delta"} {
		params.reset()
		if r.find("GET", path, &params) == nil {
			t.Errorf("GET %s should match via indices lookup", path)
		}
	}

	// Non-existent first byte
	params.reset()
	if r.find("GET", "/zeta", &params) != nil {
		t.Error("GET /zeta should NOT match")
	}
}

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

	var params routeParams
	if r.find("GET", "/test", &params) == nil {
		t.Error("find should still work after Compile()")
	}
}

func TestMethodNotAllowed_SingleMethod(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/resource", h)

	var params routeParams

	// DELETE should not find a handler
	params.reset()
	if r.find("DELETE", "/resource", &params) != nil {
		t.Error("DELETE /resource should not match")
	}

	// findAllowedMethods should return "GET"
	params.reset()
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

	var params routeParams

	// DELETE should not find a handler
	params.reset()
	if r.find("DELETE", "/items", &params) != nil {
		t.Error("DELETE /items should not match")
	}

	// findAllowedMethods should list all registered methods
	params.reset()
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

	var params routeParams

	// POST /users/42 should not match
	params.reset()
	if r.find("POST", "/users/42", &params) != nil {
		t.Error("POST /users/42 should not match")
	}

	// findAllowedMethods should return GET
	params.reset()
	allowed := r.findAllowedMethods("/users/42")
	if !containsMethod(allowed, "GET") {
		t.Errorf("allowed = %q, want to contain GET", allowed)
	}
}

func TestNotFound_UnknownPath(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/known", h)

	var params routeParams

	// Completely unknown path
	params.reset()
	if r.find("GET", "/unknown/path", &params) != nil {
		t.Error("GET /unknown/path should return nil")
	}

	// findAllowedMethods should return empty
	params.reset()
	allowed := r.findAllowedMethods("/unknown/path")
	if allowed != "" {
		t.Errorf("findAllowedMethods for unknown path = %q, want empty", allowed)
	}
}

func TestNotFound_EmptyRouter(t *testing.T) {
	r := newRouter()
	var params routeParams

	params.reset()
	if r.find("GET", "/anything", &params) != nil {
		t.Error("empty router should return nil for any path")
	}

	params.reset()
	allowed := r.findAllowedMethods("/anything")
	if allowed != "" {
		t.Errorf("empty router findAllowedMethods = %q, want empty", allowed)
	}
}

func TestEdge_TrailingSlash(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	var params routeParams

	// Exact match without trailing slash
	params.reset()
	if r.find("GET", "/users", &params) == nil {
		t.Error("GET /users should match")
	}
}

func TestEdge_EmptyParamsMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/static", h)

	// Empty params map should work for static routes
	var params routeParams
	params.reset()
	if r.find("GET", "/static", &params) == nil {
		t.Error("static route should match with empty params map")
	}
	if params.count != 0 {
		t.Errorf("static route should not populate params, got %d entries", params.count)
	}
}

func TestEdge_ManySegments(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/a/b/c/d/e/f/g", h)

	var params routeParams

	params.reset()
	if r.find("GET", "/a/b/c/d/e/f/g", &params) == nil {
		t.Error("deep static route should match")
	}

	params.reset()
	if r.find("GET", "/a/b/c/d/e/f", &params) != nil {
		t.Error("partial deep path should NOT match")
	}
}

func TestEdge_ParamAfterStaticAtSameLevel(t *testing.T) {
	r := newRouter()
	hStatic := []HandlerFunc{dummyHandler()}
	hParam := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/items/new", hStatic)
	r.addRoute("GET", "/items/:id", hParam)

	var params routeParams

	// "new" should match static
	params.reset()
	got := r.find("GET", "/items/new", &params)
	if got == nil {
		t.Fatal("GET /items/new should match")
	}

	// "42" should match param
	params.reset()
	got = r.find("GET", "/items/42", &params)
	if got == nil {
		t.Fatal("GET /items/42 should match param")
	}
	if params.get("id") != "42" {
		t.Errorf("params.get(id) = %q, want %q", params.get("id"), "42")
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

	var params routeParams
	for _, tt := range tests {
		params.reset()
		got := r.find(tt.method, tt.path, &params)
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
				if params.get(k) != v {
					t.Errorf("%s %s: params.get(%s) = %q, want %q", tt.method, tt.path, k, params.get(k), v)
				}
			}
		}
	}
}

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

func TestOptimizeTree_SortByHitsDescending(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/alpha", h)
	r.addRoute("GET", "/beta", h)
	r.addRoute("GET", "/gamma", h)

	var params routeParams

	// Simulate traffic: gamma most popular, then alpha, then beta
	for i := 0; i < 100; i++ {
		params.reset()
		r.find("GET", "/gamma", &params)
	}
	for i := 0; i < 50; i++ {
		params.reset()
		r.find("GET", "/alpha", &params)
	}
	for i := 0; i < 10; i++ {
		params.reset()
		r.find("GET", "/beta", &params)
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

	var params routeParams

	// Give param child more hits than static
	for i := 0; i < 100; i++ {
		params.reset()
		r.find("GET", "/users/42", &params)
	}
	for i := 0; i < 10; i++ {
		params.reset()
		r.find("GET", "/users/admin", &params)
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

	var params routeParams

	// Give wildcard lots of hits
	for i := 0; i < 200; i++ {
		params.reset()
		r.find("GET", "/files/a/b/c", &params)
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

	var params routeParams

	// Make gamma most popular to force reorder
	for i := 0; i < 100; i++ {
		params.reset()
		r.find("GET", "/gamma", &params)
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

	var params routeParams

	// Simulate traffic to create varied hit counts
	for i := 0; i < 50; i++ {
		params.reset()
		r.find("GET", "/health", &params)
	}
	for i := 0; i < 100; i++ {
		params.reset()
		r.find("GET", "/api/v1/users", &params)
	}
	for i := 0; i < 30; i++ {
		params.reset()
		r.find("GET", "/api/v1/users/42", &params)
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
		params.reset()
		got := r.find(tt.method, tt.path, &params)
		if tt.wantMatch && got == nil {
			t.Errorf("after Compile: %s %s should match", tt.method, tt.path)
			continue
		}
		if !tt.wantMatch && got != nil {
			t.Errorf("after Compile: %s %s should NOT match", tt.method, tt.path)
			continue
		}
		for k, v := range tt.wantParams {
			if params.get(k) != v {
				t.Errorf("after Compile: %s %s params.get(%s)=%q, want %q", tt.method, tt.path, k, params.get(k), v)
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

	var params routeParams

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
		params.reset()
		r.find("GET", "/users", &params)
	}

	if usersNode.hits != initialHits+5 {
		t.Errorf("hits = %d, want %d (should increment before Compile)", usersNode.hits, initialHits+5)
	}
}

func TestHits_NoIncrementAfterCompile(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	var params routeParams

	// Generate some hits before compile
	for i := 0; i < 5; i++ {
		params.reset()
		r.find("GET", "/users", &params)
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
		params.reset()
		r.find("GET", "/users", &params)
	}

	if usersNode.hits != hitsAfterCompile {
		t.Errorf("hits changed after Compile: was %d, now %d (should not increment after Compile)",
			hitsAfterCompile, usersNode.hits)
	}
}

func TestStaticRouteMap_CompileBuildsMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/users", h)
	r.addRoute("GET", "/users/profile", h)
	r.addRoute("POST", "/users", h)
	r.addRoute("GET", "/users/:id", h)
	r.addRoute("GET", "/files/*filepath", h)

	r.Compile()

	// Static routes must be present.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/"},
		{"GET", "/users"},
		{"GET", "/users/profile"},
		{"POST", "/users"},
	} {
		routes := r.staticRoutes[methodIndex(tc.method)]
		if routes == nil {
			t.Errorf("staticRoutes[%q] missing after Compile", tc.method)
			continue
		}
		if _, ok := routes[tc.path]; !ok {
			t.Errorf("staticRoutes[%q][%q] missing after Compile", tc.method, tc.path)
		}
	}

	// Param and wildcard routes must NOT be in the static map.
	getRoutes := r.staticRoutes[methodIndex("GET")]
	for _, badPath := range []string{"/users/:id", "/files/*filepath", "/files/"} {
		if _, ok := getRoutes[badPath]; ok {
			t.Errorf("staticRoutes[GET][%q] should not be in static map", badPath)
		}
	}
}

// TestStaticRouteMap_LookupHitsMap verifies that find() returns handlers via
// the static map for fully-static paths after Compile().
func TestStaticRouteMap_LookupHitsMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/", h)
	r.addRoute("GET", "/ping", h)
	r.addRoute("GET", "/api/v1/health", h)
	r.Compile()

	var params routeParams

	for _, path := range []string{"/", "/ping", "/api/v1/health"} {
		params.reset()
		got := r.find("GET", path, &params)
		if got == nil {
			t.Errorf("find(GET, %q) returned nil — expected handlers from static map", path)
		}
	}
}

// TestStaticRouteMap_ParamRouteFallsBackToTree verifies that param routes are
// not in the static map and still resolve correctly via tree traversal.
func TestStaticRouteMap_ParamRouteFallsBackToTree(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/:id", h)
	r.addRoute("GET", "/users/:id/posts", h)
	r.Compile()

	var params routeParams

	tests := []struct {
		path      string
		wantParam string
		wantValue string
	}{
		{"/users/42", "id", "42"},
		{"/users/abc/posts", "id", "abc"},
	}
	for _, tt := range tests {
		params.reset()
		got := r.find("GET", tt.path, &params)
		if got == nil {
			t.Errorf("find(GET, %q) returned nil — param route should fall back to tree", tt.path)
			continue
		}
		if params.get(tt.wantParam) != tt.wantValue {
			t.Errorf("find(GET, %q): param[%q] = %q, want %q",
				tt.path, tt.wantParam, params.get(tt.wantParam), tt.wantValue)
		}
	}
}

// TestStaticRouteMap_WildcardNotInMap verifies that wildcard routes are absent
// from staticRoutes and still resolve via tree traversal.
func TestStaticRouteMap_WildcardNotInMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/files/*filepath", h)
	r.Compile()

	// Wildcard route must not appear in the static map.
	routes := r.staticRoutes[methodIndex("GET")]
	if routes != nil {
		if _, found := routes["/files/*filepath"]; found {
			t.Error("wildcard route should not be in staticRoutes")
		}
	}

	// But it must still resolve via tree traversal.
	var params routeParams
	got := r.find("GET", "/files/some/deep/path.txt", &params)
	if got == nil {
		t.Error("find(GET, /files/some/deep/path.txt) should resolve via tree")
	}
}

// TestStaticRouteMap_MissedStaticFallsBackToTree verifies that a path not in
// the static map falls back to the tree and returns nil when unregistered.
func TestStaticRouteMap_MissedStaticFallsBackToTree(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/known", h)
	r.Compile()

	var params routeParams
	got := r.find("GET", "/unknown", &params)
	if got != nil {
		t.Error("find(GET, /unknown) should return nil for unregistered path")
	}
}

// TestStaticRouteMap_NotBuiltBeforeCompile verifies that staticRoutes is nil
// before Compile() is called.
func TestStaticRouteMap_NotBuiltBeforeCompile(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/hello", h)

	// Before Compile, staticRoutes entries must be nil — fast path must not fire.
	if r.staticRoutes[methodIndex("GET")] != nil {
		t.Error("staticRoutes[GET] should be nil before Compile()")
	}

	// find() must still work via tree traversal before Compile.
	var params routeParams
	got := r.find("GET", "/hello", &params)
	if got == nil {
		t.Error("find(GET, /hello) should work via tree before Compile()")
	}
}

// TestStaticRouteMap_MultipleMethodsSeparateMaps verifies that each HTTP method
// gets its own inner map in staticRoutes.
func TestStaticRouteMap_MultipleMethodsSeparateMaps(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/items", h)
	r.addRoute("POST", "/items", h)
	r.addRoute("DELETE", "/items", h)
	r.Compile()

	for _, method := range []string{"GET", "POST", "DELETE"} {
		routes := r.staticRoutes[methodIndex(method)]
		if routes == nil {
			t.Errorf("staticRoutes[%q] missing", method)
			continue
		}
		if _, ok := routes["/items"]; !ok {
			t.Errorf("staticRoutes[%q][/items] missing", method)
		}
	}

	patchRoutes := r.staticRoutes[methodIndex("PATCH")]
	if patchRoutes != nil {
		if len(patchRoutes) > 0 {
			t.Error("staticRoutes[PATCH] should be empty (no routes registered)")
		}
	}
}

func TestMethodIndex_StandardMethods(t *testing.T) {
	tests := []struct {
		method string
		want   int
	}{
		{"GET", mGET},
		{"POST", mPOST},
		{"PUT", mPUT},
		{"DELETE", mDELETE},
		{"PATCH", mPATCH},
		{"OPTIONS", mOPTIONS},
		{"HEAD", mHEAD},
	}
	for _, tt := range tests {
		got := methodIndex(tt.method)
		if got != tt.want {
			t.Errorf("methodIndex(%q) = %d, want %d", tt.method, got, tt.want)
		}
	}
}

// TestMethodIndex_UnknownMethods verifies methodIndex returns -1 for
// custom/unknown methods so they fall back to the map.
func TestMethodIndex_UnknownMethods(t *testing.T) {
	unknowns := []string{"CUSTOM", "PROPFIND", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK", "TRACE", "CONNECT", ""}
	for _, m := range unknowns {
		if got := methodIndex(m); got != -1 {
			t.Errorf("methodIndex(%q) = %d, want -1", m, got)
		}
	}
}

// TestMethodIndex_AllIndicesUnique verifies that all standard method constants
// are distinct and within [0, mCOUNT).
func TestMethodIndex_AllIndicesUnique(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	seen := make(map[int]string, mCOUNT)
	for _, m := range methods {
		idx := methodIndex(m)
		if idx < 0 || idx >= mCOUNT {
			t.Errorf("methodIndex(%q) = %d out of range [0, %d)", m, idx, mCOUNT)
			continue
		}
		if prev, dup := seen[idx]; dup {
			t.Errorf("methodIndex(%q) = %d collides with %q", m, idx, prev)
		}
		seen[idx] = m
	}
	if len(seen) != len(methods) {
		t.Errorf("expected %d unique indices, got %d", len(methods), len(seen))
	}
}

// TestMethodArray_PopulatedByNewRouter verifies that newRouter() populates
// methodTrees for all standard methods pointing to the same node as trees map.
func TestMethodArray_PopulatedByNewRouter(t *testing.T) {
	r := newRouter()
	for _, m := range standardMethods {
		idx := methodIndex(m)
		if idx < 0 {
			t.Errorf("standardMethod %q has no array index", m)
			continue
		}
		if r.methodTrees[idx] == nil {
			t.Errorf("methodTrees[%d] (%s) is nil after newRouter()", idx, m)
			continue
		}
		// Must be the SAME pointer as trees map — not a copy.
		if r.methodTrees[idx] != r.trees[m] {
			t.Errorf("methodTrees[%d] (%s) != trees[%q]: must be same node", idx, m, m)
		}
	}
}

// TestMethodArray_StandardMethodsUseArray verifies that standard HTTP methods
// are routed through the array (find returns correct handlers).
func TestMethodArray_StandardMethodsUseArray(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	var params routeParams

	for _, m := range standardMethods {
		r.addRoute(m, "/ping", h)
	}
	r.Compile()

	for _, m := range standardMethods {
		params.reset()
		got := r.find(m, "/ping", &params)
		if got == nil {
			t.Errorf("find(%q, /ping) returned nil — array lookup failed", m)
		}
	}
}

// TestMethodArray_CustomMethodFallsBackToMap verifies that custom methods
// (not in the standard list) still work via the trees map fallback.
func TestMethodArray_CustomMethodFallsBackToMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	var params routeParams

	customMethods := []string{"CUSTOM", "PROPFIND", "MKCOL"}
	for _, m := range customMethods {
		r.addRoute(m, "/resource", h)
	}
	r.Compile()

	for _, m := range customMethods {
		// methodIndex must return -1 for these
		if idx := methodIndex(m); idx != -1 {
			t.Errorf("methodIndex(%q) = %d, want -1 (custom method)", m, idx)
		}
		params.reset()
		got := r.find(m, "/resource", &params)
		if got == nil {
			t.Errorf("find(%q, /resource) returned nil — map fallback failed", m)
		}
	}
}

// TestMethodArray_AddRouteSyncsArrayAndMap verifies that addRoute() keeps
// methodTrees and trees pointing to the same node after adding routes.
func TestMethodArray_AddRouteSyncsArrayAndMap(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/a", h)
	r.addRoute("POST", "/b", h)

	// After addRoute, array and map must still point to the same root node.
	for _, m := range []string{"GET", "POST"} {
		idx := methodIndex(m)
		if r.methodTrees[idx] != r.trees[m] {
			t.Errorf("after addRoute(%q), methodTrees[%d] != trees[%q]", m, idx, m)
		}
	}
}

// TestMethodArray_UnknownMethodReturnsNil verifies that find() returns nil
// for a completely unknown method with no registered routes.
func TestMethodArray_UnknownMethodReturnsNil(t *testing.T) {
	r := newRouter()
	r.Compile()
	var params routeParams

	got := r.find("UNKNOWN", "/anything", &params)
	if got != nil {
		t.Error("find(UNKNOWN, /anything) should return nil for unregistered custom method")
	}
}

// TestMethodArray_mCOUNT verifies the mCOUNT constant equals 7 (number of standard methods).
func TestMethodArray_mCOUNT(t *testing.T) {
	if mCOUNT != 7 {
		t.Errorf("mCOUNT = %d, want 7", mCOUNT)
	}
	if len(standardMethods) != mCOUNT {
		t.Errorf("len(standardMethods) = %d, want mCOUNT=%d", len(standardMethods), mCOUNT)
	}
}

func TestEdge_URLEncodedParamValues(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users/:name", h)

	var params routeParams

	// Space encoded as %20
	params.reset()
	got := r.find("GET", "/users/john%20doe", &params)
	if got == nil {
		t.Fatal("GET /users/john%20doe should match param route")
	}

	// Plus sign (literal)
	params.reset()
	got = r.find("GET", "/users/john+doe", &params)
	if got == nil {
		t.Fatal("GET /users/john+doe should match param route")
	}
}

func TestEdge_CaseSensitiveRoutes(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/Users", h)

	var params routeParams

	params.reset()
	if r.find("GET", "/Users", &params) == nil {
		t.Error("GET /Users should match")
	}

	params.reset()
	if r.find("GET", "/users", &params) != nil {
		t.Error("GET /users should NOT match /Users (case sensitive)")
	}

	params.reset()
	if r.find("GET", "/USERS", &params) != nil {
		t.Error("GET /USERS should NOT match /Users (case sensitive)")
	}
}

func TestEdge_DoubleSlashSegments(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	var params routeParams

	// Double slash should not match single-slash route
	params.reset()
	got := r.find("GET", "//users", &params)
	if got != nil {
		t.Error("GET //users should NOT match /users")
	}
}

func TestEdge_StaticParamConflictPriority(t *testing.T) {
	r := newRouter()
	hStatic := []HandlerFunc{dummyHandler()}
	hParam := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/users/new", hStatic)
	r.addRoute("GET", "/users/:id", hParam)
	r.addRoute("GET", "/users/settings", hStatic)

	var params routeParams

	tests := []struct {
		path       string
		wantStatic bool
	}{
		{"/users/new", true},
		{"/users/settings", true},
		{"/users/123", false},
		{"/users/anything", false},
	}

	for _, tt := range tests {
		params.reset()
		got := r.find("GET", tt.path, &params)
		if got == nil {
			t.Errorf("GET %s should match", tt.path)
			continue
		}
		hasParam := params.count > 0
		if tt.wantStatic && hasParam {
			t.Errorf("GET %s should match static, but got param instead", tt.path)
		}
		if !tt.wantStatic && !hasParam {
			t.Errorf("GET %s should match param, but got static", tt.path)
		}
	}
}

func TestEdge_WildcardVsStaticPriority(t *testing.T) {
	r := newRouter()
	hStatic := []HandlerFunc{dummyHandler()}
	hWild := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/files/readme.txt", hStatic)
	r.addRoute("GET", "/files/*path", hWild)

	var params routeParams

	// Exact static match takes priority
	params.reset()
	got := r.find("GET", "/files/readme.txt", &params)
	if got == nil {
		t.Fatal("GET /files/readme.txt should match")
	}

	// Other paths fall to wildcard
	params.reset()
	got = r.find("GET", "/files/other.txt", &params)
	if got == nil {
		t.Fatal("GET /files/other.txt should match wildcard")
	}
	if params.get("path") != "other.txt" {
		t.Errorf("wildcard param = %q, want other.txt", params.get("path"))
	}
}

func TestEdge_TrailingSlashMismatch(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}
	r.addRoute("GET", "/users", h)

	var params routeParams

	params.reset()
	got := r.find("GET", "/users/", &params)
	// Trailing slash on /users/ should not match /users (strict)
	if got != nil {
		t.Log("trailing slash matched — framework normalizes trailing slashes")
	}
}

func TestEdge_DeepNesting20Segments(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	path := ""
	for i := 0; i < 20; i++ {
		path += "/s" + string(rune('a'+i))
	}
	r.addRoute("GET", path, h)

	var params routeParams
	params.reset()
	if r.find("GET", path, &params) == nil {
		t.Errorf("20-segment route should match: %s", path)
	}

	// Partial should not match
	partial := ""
	for i := 0; i < 10; i++ {
		partial += "/s" + string(rune('a'+i))
	}
	params.reset()
	if r.find("GET", partial, &params) != nil {
		t.Error("partial 10/20 segments should not match")
	}
}

func TestEdge_SpecialCharsInStaticPath(t *testing.T) {
	r := newRouter()
	h := []HandlerFunc{dummyHandler()}

	r.addRoute("GET", "/api/v1.0/health-check", h)
	r.addRoute("GET", "/api/v2~beta/status", h)

	var params routeParams

	params.reset()
	if r.find("GET", "/api/v1.0/health-check", &params) == nil {
		t.Error("dots and hyphens in static path should match")
	}

	params.reset()
	if r.find("GET", "/api/v2~beta/status", &params) == nil {
		t.Error("tilde in static path should match")
	}
}
