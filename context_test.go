package kruda

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

// TestShrinkMaps_BelowThreshold verifies that maps at or below their
// threshold are NOT replaced — shrinkMaps should be zero-cost for normal maps.
// Note: params no longer uses a map (uses fixed-size routeParams array),
// so only headers, respHeaders, and locals are tested for map shrink behavior.
func TestShrinkMaps_BelowThreshold(t *testing.T) {
	app := New()
	c := newCtx(app)

	// Fill each map to exactly its threshold (should NOT trigger shrink)
	for i := 0; i < maxHeadersCapacity; i++ {
		c.headers[fmt.Sprintf("h%d", i)] = "v"
	}
	for i := 0; i < maxRespHeadersCapacity; i++ {
		c.respHeaders[fmt.Sprintf("rh%d", i)] = []string{"v"}
	}
	for i := 0; i < maxLocalsCapacity; i++ {
		c.locals[fmt.Sprintf("l%d", i)] = "v"
	}

	// Capture pointers before shrink (using fmt to get map identity)
	headersBefore := c.headers
	respHeadersBefore := c.respHeaders
	localsBefore := c.locals

	c.shrinkMaps()

	// Maps at threshold should be untouched — same reference, same entries
	if len(c.headers) != maxHeadersCapacity {
		t.Errorf("headers: want %d entries, got %d", maxHeadersCapacity, len(c.headers))
	}
	if len(c.respHeaders) != maxRespHeadersCapacity {
		t.Errorf("respHeaders: want %d entries, got %d", maxRespHeadersCapacity, len(c.respHeaders))
	}
	if len(c.locals) != maxLocalsCapacity {
		t.Errorf("locals: want %d entries, got %d", maxLocalsCapacity, len(c.locals))
	}

	// Verify same map reference (not replaced)
	if fmt.Sprintf("%p", headersBefore) != fmt.Sprintf("%p", c.headers) {
		t.Error("headers map was replaced despite being at threshold")
	}
	if fmt.Sprintf("%p", respHeadersBefore) != fmt.Sprintf("%p", c.respHeaders) {
		t.Error("respHeaders map was replaced despite being at threshold")
	}
	if fmt.Sprintf("%p", localsBefore) != fmt.Sprintf("%p", c.locals) {
		t.Error("locals map was replaced despite being at threshold")
	}
}

// TestShrinkMaps_AboveThreshold verifies that maps exceeding their threshold
// are replaced with fresh empty maps of initial capacity.
func TestShrinkMaps_AboveThreshold(t *testing.T) {
	app := New()
	c := newCtx(app)

	// Fill each map beyond its threshold
	for i := 0; i <= maxHeadersCapacity; i++ {
		c.headers[fmt.Sprintf("h%d", i)] = "v"
	}
	for i := 0; i <= maxRespHeadersCapacity; i++ {
		c.respHeaders[fmt.Sprintf("rh%d", i)] = []string{"v"}
	}
	for i := 0; i <= maxLocalsCapacity; i++ {
		c.locals[fmt.Sprintf("l%d", i)] = "v"
	}

	c.shrinkMaps()

	// All maps should now be empty (replaced with fresh maps)
	if len(c.headers) != 0 {
		t.Errorf("headers: want 0 entries after shrink, got %d", len(c.headers))
	}
	if len(c.respHeaders) != 0 {
		t.Errorf("respHeaders: want 0 entries after shrink, got %d", len(c.respHeaders))
	}
	if len(c.locals) != 0 {
		t.Errorf("locals: want 0 entries after shrink, got %d", len(c.locals))
	}
}

// TestShrinkMaps_NotNil verifies that after shrinking oversized maps,
// all maps are still valid (not nil) — required for the next reset() to work.
func TestShrinkMaps_NotNil(t *testing.T) {
	app := New()
	c := newCtx(app)

	// Fill beyond threshold
	for i := 0; i <= maxHeadersCapacity; i++ {
		c.headers[fmt.Sprintf("h%d", i)] = "v"
	}
	for i := 0; i <= maxRespHeadersCapacity; i++ {
		c.respHeaders[fmt.Sprintf("rh%d", i)] = []string{"v"}
	}
	for i := 0; i <= maxLocalsCapacity; i++ {
		c.locals[fmt.Sprintf("l%d", i)] = "v"
	}

	c.shrinkMaps()

	if c.headers == nil {
		t.Error("headers is nil after shrink")
	}
	if c.respHeaders == nil {
		t.Error("respHeaders is nil after shrink")
	}
	if c.locals == nil {
		t.Error("locals is nil after shrink")
	}
}

// TestShrinkMaps_MixedSizes verifies that only oversized maps get replaced
// while maps within threshold remain untouched with their original data.
func TestShrinkMaps_MixedSizes(t *testing.T) {
	app := New()
	c := newCtx(app)

	// params: set some values (routeParams is fixed-size, no shrink needed)
	c.params.set("key1", "val1")
	c.params.set("key2", "val2")

	// headers: above threshold (should be replaced)
	for i := 0; i <= maxHeadersCapacity; i++ {
		c.headers[fmt.Sprintf("h%d", i)] = "v"
	}

	// respHeaders: below threshold (should survive)
	c.respHeaders["X-Custom"] = []string{"a", "b"}
	respHeadersBefore := c.respHeaders

	// locals: above threshold (should be replaced)
	for i := 0; i <= maxLocalsCapacity; i++ {
		c.locals[fmt.Sprintf("l%d", i)] = i
	}

	c.shrinkMaps()

	// params: routeParams is not affected by shrinkMaps — values preserved
	if c.params.count != 2 {
		t.Errorf("params: want 2 entries, got %d", c.params.count)
	}
	if c.params.get("key1") != "val1" || c.params.get("key2") != "val2" {
		t.Error("params entries were modified")
	}

	// headers: replaced — empty
	if len(c.headers) != 0 {
		t.Errorf("headers: want 0 entries after shrink, got %d", len(c.headers))
	}

	// respHeaders: untouched — same reference, same data
	if fmt.Sprintf("%p", respHeadersBefore) != fmt.Sprintf("%p", c.respHeaders) {
		t.Error("respHeaders was replaced despite being below threshold")
	}
	if len(c.respHeaders) != 1 {
		t.Errorf("respHeaders: want 1 entry, got %d", len(c.respHeaders))
	}

	// locals: replaced — empty
	if len(c.locals) != 0 {
		t.Errorf("locals: want 0 entries after shrink, got %d", len(c.locals))
	}
}

// TestRouteParams_SetGetDel verifies basic routeParams operations.
func TestRouteParams_SetGetDel(t *testing.T) {
	var p routeParams

	// Empty: get returns ""
	if got := p.get("id"); got != "" {
		t.Errorf("empty get: want \"\", got %q", got)
	}

	// Set and get
	p.set("id", "42")
	if got := p.get("id"); got != "42" {
		t.Errorf("after set: want \"42\", got %q", got)
	}
	if p.count != 1 {
		t.Errorf("count: want 1, got %d", p.count)
	}

	// Overwrite existing key
	p.set("id", "99")
	if got := p.get("id"); got != "99" {
		t.Errorf("after overwrite: want \"99\", got %q", got)
	}
	if p.count != 1 {
		t.Errorf("count after overwrite: want 1, got %d", p.count)
	}

	// Add second key
	p.set("name", "tiger")
	if p.count != 2 {
		t.Errorf("count: want 2, got %d", p.count)
	}

	// Del
	p.del("id")
	if p.count != 1 {
		t.Errorf("count after del: want 1, got %d", p.count)
	}
	if got := p.get("id"); got != "" {
		t.Errorf("after del: want \"\", got %q", got)
	}
	if got := p.get("name"); got != "tiger" {
		t.Errorf("name after del id: want \"tiger\", got %q", got)
	}
}

// TestRouteParams_Reset verifies that reset clears all entries.
func TestRouteParams_Reset(t *testing.T) {
	var p routeParams
	p.set("a", "1")
	p.set("b", "2")
	p.set("c", "3")

	p.reset()

	if p.count != 0 {
		t.Errorf("count after reset: want 0, got %d", p.count)
	}
	if got := p.get("a"); got != "" {
		t.Errorf("get after reset: want \"\", got %q", got)
	}
}

// TestRouteParams_MaxCapacity verifies that params beyond maxRouteParams are silently ignored.
func TestRouteParams_MaxCapacity(t *testing.T) {
	var p routeParams
	for i := 0; i < maxRouteParams+5; i++ {
		p.set(fmt.Sprintf("p%d", i), "v")
	}
	if p.count != maxRouteParams {
		t.Errorf("count: want %d (max), got %d", maxRouteParams, p.count)
	}
}

// TestSendBytes_SmallResponse verifies that responses ≤4KB use the pooled
// buffer path and produce the correct body in the response.
func TestSendBytes_SmallResponse(t *testing.T) {
	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.Text("hello")
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "hello" {
		t.Errorf("want body %q, got %q", "hello", got)
	}
}

// TestSendBytes_LargeResponse verifies that responses >4KB use the direct
// write path and still produce the correct body.
func TestSendBytes_LargeResponse(t *testing.T) {
	large := make([]byte, responseBufPoolThreshold+1)
	for i := range large {
		large[i] = 'x'
	}

	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.HTML(string(large))
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Body.Len(); got != len(large) {
		t.Errorf("want body length %d, got %d", len(large), got)
	}
}

// TestSendBytes_ExactThreshold verifies that a response of exactly 4096 bytes
// uses the pooled buffer path (boundary condition).
func TestSendBytes_ExactThreshold(t *testing.T) {
	exact := make([]byte, responseBufPoolThreshold)
	for i := range exact {
		exact[i] = 'a'
	}

	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.HTML(string(exact))
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Body.Len(); got != responseBufPoolThreshold {
		t.Errorf("want body length %d, got %d", responseBufPoolThreshold, got)
	}
}

// TestSendBytes_PoolReuse verifies that the pool is correctly reused across
// multiple requests — no data leaks between requests.
func TestSendBytes_PoolReuse(t *testing.T) {
	app := New()
	app.Get("/a", func(c *Ctx) error { return c.Text("response-a") })
	app.Get("/b", func(c *Ctx) error { return c.Text("response-b") })
	app.Compile()

	for i := 0; i < 10; i++ {
		wA := httptest.NewRecorder()
		app.ServeHTTP(wA, httptest.NewRequest("GET", "/a", nil))
		if got := wA.Body.String(); got != "response-a" {
			t.Errorf("iter %d: /a want %q, got %q", i, "response-a", got)
		}

		wB := httptest.NewRecorder()
		app.ServeHTTP(wB, httptest.NewRequest("GET", "/b", nil))
		if got := wB.Body.String(); got != "response-b" {
			t.Errorf("iter %d: /b want %q, got %q", i, "response-b", got)
		}
	}
}
