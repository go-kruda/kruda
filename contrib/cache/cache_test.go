package cache

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport"
)

// --- Mock transport types (same pattern as contrib/session) ---

// mockRequest implements transport.Request for testing.
type mockRequest struct {
	method  string
	path    string
	headers map[string]string
	body    []byte
}

func (r *mockRequest) Method() string               { return r.method }
func (r *mockRequest) Path() string                 { return r.path }
func (r *mockRequest) Header(key string) string     { return r.headers[key] }
func (r *mockRequest) Body() ([]byte, error)        { return r.body, nil }
func (r *mockRequest) QueryParam(key string) string { return "" }
func (r *mockRequest) RemoteAddr() string           { return "127.0.0.1" }
func (r *mockRequest) RawRequest() any              { return nil }
func (r *mockRequest) Context() context.Context     { return context.Background() }
func (r *mockRequest) Cookie(name string) string    { return "" }

func (r *mockRequest) MultipartForm(int64) (*multipart.Form, error) {
	return nil, fmt.Errorf("not supported")
}

// Compile-time interface check.
var _ transport.Request = (*mockRequest)(nil)

// mockResponseWriter implements transport.ResponseWriter for testing.
type mockResponseWriter struct {
	statusCode int
	headers    *mockHeaderMap
	body       []byte
}

func newMockResponse() *mockResponseWriter {
	return &mockResponseWriter{
		statusCode: 200,
		headers:    &mockHeaderMap{h: make(map[string]string)},
	}
}

func (w *mockResponseWriter) WriteHeader(code int)        { w.statusCode = code }
func (w *mockResponseWriter) Header() transport.HeaderMap { return w.headers }
func (w *mockResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

// Compile-time interface check.
var _ transport.ResponseWriter = (*mockResponseWriter)(nil)

// mockHeaderMap implements transport.HeaderMap for testing.
type mockHeaderMap struct {
	h map[string]string
}

func (m *mockHeaderMap) Set(key, value string) { m.h[http.CanonicalHeaderKey(key)] = value }
func (m *mockHeaderMap) Get(key string) string { return m.h[http.CanonicalHeaderKey(key)] }
func (m *mockHeaderMap) Add(key, value string) {
	key = http.CanonicalHeaderKey(key)
	if existing := m.h[key]; existing != "" {
		m.h[key] = existing + ", " + value
	} else {
		m.h[key] = value
	}
}
func (m *mockHeaderMap) Del(key string) { delete(m.h, http.CanonicalHeaderKey(key)) }

// Compile-time interface check.
var _ transport.HeaderMap = (*mockHeaderMap)(nil)

// --- Test helpers ---

func newReq(method, path string) *mockRequest {
	return &mockRequest{
		method:  method,
		path:    path,
		headers: make(map[string]string),
	}
}

// --- Tests ---

func TestCache_Miss_ThenHit(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store: store,
		TTL:   5 * time.Minute,
	}

	handler := func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"data": "hello"})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/data", handler)

	// First request: MISS
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/data"))

	if resp1.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp1.statusCode)
	}

	xcache1 := resp1.headers.Get("X-Cache")
	if xcache1 != "MISS" {
		t.Fatalf("expected X-Cache: MISS on first request, got %q", xcache1)
	}

	if len(resp1.body) == 0 {
		t.Fatal("expected non-empty body on first request")
	}

	// Verify the response was cached.
	if store.Len() != 1 {
		t.Fatalf("expected 1 cached entry, got %d", store.Len())
	}

	// Second request: HIT
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/data"))

	if resp2.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.statusCode)
	}

	xcache2 := resp2.headers.Get("X-Cache")
	if xcache2 != "HIT" {
		t.Fatalf("expected X-Cache: HIT on second request, got %q", xcache2)
	}

	if len(resp2.body) == 0 {
		t.Fatal("expected non-empty body on cache hit")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store: store,
		TTL:   1 * time.Millisecond, // Very short TTL
	}

	callCount := 0
	handler := func(c *kruda.Ctx) error {
		callCount++
		return CacheJSON(c, kruda.Map{"call": callCount})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/data", handler)

	// First request: MISS
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/data"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatalf("expected MISS on first request")
	}
	if callCount != 1 {
		t.Fatalf("expected handler called once, got %d", callCount)
	}

	// Wait for TTL to expire.
	time.Sleep(10 * time.Millisecond)

	// Second request: should be MISS again because TTL expired.
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/data"))

	if resp2.headers.Get("X-Cache") != "MISS" {
		t.Fatalf("expected MISS after TTL expiry, got %q", resp2.headers.Get("X-Cache"))
	}
	if callCount != 2 {
		t.Fatalf("expected handler called twice after TTL expiry, got %d", callCount)
	}
}

func TestCache_SkipNonGET(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store: store,
	}

	callCount := 0
	handler := func(c *kruda.Ctx) error {
		callCount++
		return CacheJSON(c, kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Post("/api/data", handler)

	// POST request should bypass cache entirely.
	resp := newMockResponse()
	app.ServeKruda(resp, newReq("POST", "/api/data"))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}

	// Should NOT have X-Cache header since POST is not in default Methods.
	xcache := resp.headers.Get("X-Cache")
	if xcache != "" {
		t.Fatalf("expected no X-Cache header for POST, got %q", xcache)
	}

	// Store should be empty (nothing cached).
	if store.Len() != 0 {
		t.Fatalf("expected 0 cached entries for POST, got %d", store.Len())
	}

	if callCount != 1 {
		t.Fatalf("expected handler called once, got %d", callCount)
	}
}

func TestCache_CustomKeyFunc(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store: store,
		KeyFunc: func(c *kruda.Ctx) string {
			return "custom:" + c.Path()
		},
	}

	handler := func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"data": "value"})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/data", handler)

	// First request: MISS -- should cache with custom key.
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/data"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatalf("expected MISS on first request")
	}

	// Verify custom key was used.
	cached, err := store.Get("custom:/api/data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cached == nil {
		t.Fatal("expected entry with custom key 'custom:/api/data'")
	}

	// Default key should NOT exist.
	defaultCached, _ := store.Get("GET:/api/data")
	if defaultCached != nil {
		t.Fatal("expected no entry with default key pattern")
	}

	// Second request: HIT using custom key.
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/data"))

	if resp2.headers.Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT on second request with custom key")
	}
}

func TestCache_XCacheHeaders(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{Store: store}

	handler := func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/data", handler)

	// MISS
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/data"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatalf("expected X-Cache: MISS, got %q", resp1.headers.Get("X-Cache"))
	}

	// HIT
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/data"))

	if resp2.headers.Get("X-Cache") != "HIT" {
		t.Fatalf("expected X-Cache: HIT, got %q", resp2.headers.Get("X-Cache"))
	}
}

func TestCache_AgeHeader(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{Store: store}

	handler := func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/data", handler)

	// First request: MISS
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/data"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS")
	}

	// Wait a bit so Age > 0.
	time.Sleep(1100 * time.Millisecond)

	// Second request: HIT -- should have Age header.
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/data"))

	if resp2.headers.Get("X-Cache") != "HIT" {
		t.Fatal("expected HIT")
	}

	age := resp2.headers.Get("Age")
	if age == "" {
		t.Fatal("expected Age header on cache HIT")
	}
	if age == "0" {
		t.Fatal("expected Age > 0 after waiting 1 second")
	}
}

func TestCache_CustomStatusCodes(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store:       store,
		StatusCodes: []int{200, 404},
	}

	handler := func(c *kruda.Ctx) error {
		c.Status(404)
		return CacheJSON(c, kruda.Map{"error": "not found"})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/missing", handler)

	// First request: MISS -- 404 should be cached.
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/missing"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS")
	}

	// Verify 404 was cached.
	if store.Len() != 1 {
		t.Fatalf("expected 1 cached entry for 404, got %d", store.Len())
	}

	// Second request: HIT
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/missing"))

	if resp2.headers.Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT for cached 404, got %q", resp2.headers.Get("X-Cache"))
	}
	if resp2.statusCode != 404 {
		t.Fatalf("expected status 404, got %d", resp2.statusCode)
	}
}

func TestCache_NonCacheableStatus(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store:       store,
		StatusCodes: []int{200}, // Only 200 is cacheable.
	}

	handler := func(c *kruda.Ctx) error {
		c.Status(500)
		return CacheJSON(c, kruda.Map{"error": "internal"})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/fail", handler)

	// First request: MISS -- 500 should NOT be cached.
	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/api/fail"))

	if resp.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS")
	}

	// Store should be empty -- 500 is not in StatusCodes.
	if store.Len() != 0 {
		t.Fatalf("expected 0 cached entries for 500, got %d", store.Len())
	}
}

func TestCache_Skip(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store: store,
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/health"
		},
	}

	handler := func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/health", handler)
	app.Get("/api/data", handler)

	// /health should be skipped -- no X-Cache header.
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/health"))

	xcache := resp1.headers.Get("X-Cache")
	if xcache != "" {
		t.Fatalf("expected no X-Cache header for skipped path, got %q", xcache)
	}

	// /api/data should be cached normally.
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/data"))

	if resp2.headers.Get("X-Cache") != "MISS" {
		t.Fatalf("expected MISS for non-skipped path")
	}
}

func TestCache_CacheBytes(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{Store: store}

	handler := func(c *kruda.Ctx) error {
		return CacheBytes(c, "text/plain; charset=utf-8", []byte("hello world"))
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/text", handler)

	// First request: MISS
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/text"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS")
	}
	if string(resp1.body) != "hello world" {
		t.Fatalf("expected body 'hello world', got %q", resp1.body)
	}

	// Second request: HIT
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/text"))

	if resp2.headers.Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT, got %q", resp2.headers.Get("X-Cache"))
	}
	if string(resp2.body) != "hello world" {
		t.Fatalf("expected body 'hello world', got %q", resp2.body)
	}
}

func TestCache_DefaultConfig(t *testing.T) {
	// Verify default config values are applied.
	var cfg Config
	cfg.defaults()

	if cfg.TTL != 5*time.Minute {
		t.Fatalf("expected default TTL 5m, got %v", cfg.TTL)
	}
	if cfg.KeyFunc == nil {
		t.Fatal("expected default KeyFunc")
	}
	if len(cfg.Methods) != 1 || cfg.Methods[0] != "GET" {
		t.Fatalf("expected default Methods [GET], got %v", cfg.Methods)
	}
	if len(cfg.StatusCodes) != 1 || cfg.StatusCodes[0] != 200 {
		t.Fatalf("expected default StatusCodes [200], got %v", cfg.StatusCodes)
	}
}

func TestCache_MultiplePaths(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{Store: store}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/api/a", func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"path": "a"})
	})
	app.Get("/api/b", func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"path": "b"})
	})

	// Request /api/a
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/api/a"))
	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS for /api/a")
	}

	// Request /api/b
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/api/b"))
	if resp2.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS for /api/b")
	}

	// Both should be cached now.
	if store.Len() != 2 {
		t.Fatalf("expected 2 cached entries, got %d", store.Len())
	}

	// Request /api/a again -- HIT
	resp3 := newMockResponse()
	app.ServeKruda(resp3, newReq("GET", "/api/a"))
	if resp3.headers.Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT for /api/a, got %q", resp3.headers.Get("X-Cache"))
	}

	// Request /api/b again -- HIT
	resp4 := newMockResponse()
	app.ServeKruda(resp4, newReq("GET", "/api/b"))
	if resp4.headers.Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT for /api/b, got %q", resp4.headers.Get("X-Cache"))
	}
}

func TestCache_NoMiddleware_CacheJSONFallback(t *testing.T) {
	// When cache middleware is not active, CacheJSON should still work
	// (just doesn't cache).
	app := kruda.New()
	app.Get("/api/data", func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"data": "no-cache"})
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/api/data"))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
	if len(resp.body) == 0 {
		t.Fatal("expected non-empty body")
	}

	// No X-Cache header since middleware is not active.
	xcache := resp.headers.Get("X-Cache")
	if xcache != "" {
		t.Fatalf("expected no X-Cache header without middleware, got %q", xcache)
	}
}

func TestCache_CustomMethods(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{
		Store:   store,
		Methods: []string{"GET", "POST"},
	}

	handler := func(c *kruda.Ctx) error {
		return CacheJSON(c, kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Post("/api/search", handler)

	// POST should be cacheable with custom Methods.
	resp := newMockResponse()
	app.ServeKruda(resp, newReq("POST", "/api/search"))

	if resp.headers.Get("X-Cache") != "MISS" {
		t.Fatalf("expected MISS for POST with custom Methods, got %q", resp.headers.Get("X-Cache"))
	}

	if store.Len() != 1 {
		t.Fatalf("expected 1 cached entry for POST, got %d", store.Len())
	}
}

func TestCache_HitReplaysHeaders(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	cfg := Config{Store: store}

	handler := func(c *kruda.Ctx) error {
		return CacheBytes(c, "text/html; charset=utf-8", []byte("<h1>hello</h1>"))
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/page", handler)

	// MISS
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/page"))

	if resp1.headers.Get("X-Cache") != "MISS" {
		t.Fatal("expected MISS")
	}

	// HIT -- should replay Content-Type header.
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/page"))

	if resp2.headers.Get("X-Cache") != "HIT" {
		t.Fatal("expected HIT")
	}

	ct := resp2.headers.Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected Content-Type 'text/html; charset=utf-8', got %q", ct)
	}

	if string(resp2.body) != "<h1>hello</h1>" {
		t.Fatalf("expected body '<h1>hello</h1>', got %q", resp2.body)
	}
}
