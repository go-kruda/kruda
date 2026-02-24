package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport"
)

// --- Mock transport types ---

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
func (r *mockRequest) Cookie(name string) string    { return "" }
func (r *mockRequest) RawRequest() any              { return nil }

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

// --- Test helpers ---

// serve creates an App, applies middleware and a handler, then serves a mock request.
func serve(t *testing.T, mw kruda.HandlerFunc, handler kruda.HandlerFunc, req *mockRequest) *mockResponseWriter {
	t.Helper()
	app := kruda.New()
	app.Use(mw)
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, req)
	return resp
}

func getReq(path string, headers map[string]string) *mockRequest {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &mockRequest{method: "GET", path: path, headers: headers}
}

func optionsReq(headers map[string]string) *mockRequest {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &mockRequest{method: "OPTIONS", path: "/test", headers: headers}
}

// parseJSONBody parses the response body as a JSON map.
func parseJSONBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("failed to parse JSON body: %v (body=%q)", err, string(body))
	}
	return m
}

// =====================================================================
// RequestID Tests
// =====================================================================

func TestRequestID_GeneratesUUID(t *testing.T) {
	var gotLocal any
	handler := func(c *kruda.Ctx) error {
		gotLocal = c.Get("request_id")
		return c.JSON(kruda.Map{"ok": true})
	}

	resp := serve(t, RequestID(), handler, getReq("/test", nil))

	// Response header should be set.
	rid := resp.headers.Get("X-Request-ID")
	if rid == "" {
		t.Fatal("expected X-Request-ID response header to be set")
	}
	// UUID v4 format: 8-4-4-4-12 hex chars.
	if len(rid) != 36 || rid[8] != '-' || rid[13] != '-' || rid[18] != '-' || rid[23] != '-' {
		t.Fatalf("expected UUID v4 format, got %q", rid)
	}
	// Locals should match.
	if gotLocal != rid {
		t.Fatalf("expected local request_id=%q, got %v", rid, gotLocal)
	}
}

func TestRequestID_PassthroughExisting(t *testing.T) {
	existing := "my-existing-id-123"
	var gotLocal any
	handler := func(c *kruda.Ctx) error {
		gotLocal = c.Get("request_id")
		return c.JSON(kruda.Map{"ok": true})
	}

	req := getReq("/test", map[string]string{"X-Request-ID": existing})
	resp := serve(t, RequestID(), handler, req)

	rid := resp.headers.Get("X-Request-ID")
	if rid != existing {
		t.Fatalf("expected passthrough %q, got %q", existing, rid)
	}
	if gotLocal != existing {
		t.Fatalf("expected local=%q, got %v", existing, gotLocal)
	}
}

func TestRequestID_CustomHeader(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(RequestID(RequestIDConfig{Header: "X-Trace-ID"}))
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, getReq("/test", nil))

	if resp.headers.Get("X-Trace-ID") == "" {
		t.Fatal("expected X-Trace-ID header to be set")
	}
	if resp.headers.Get("X-Request-ID") != "" {
		t.Fatal("expected X-Request-ID to NOT be set when custom header is used")
	}
}

func TestRequestID_CustomGenerator(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(RequestID(RequestIDConfig{
		Generator: func() string { return "custom-id-42" },
	}))
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, getReq("/test", nil))

	rid := resp.headers.Get("X-Request-ID")
	if rid != "custom-id-42" {
		t.Fatalf("expected custom-id-42, got %q", rid)
	}
}

// =====================================================================
// Logger Tests
// =====================================================================

func TestLogger_CallsNext(t *testing.T) {
	called := false
	handler := func(c *kruda.Ctx) error {
		called = true
		return c.JSON(kruda.Map{"ok": true})
	}

	serve(t, Logger(), handler, getReq("/test", nil))

	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestLogger_SkipPaths(t *testing.T) {
	called := false
	handler := func(c *kruda.Ctx) error {
		called = true
		return c.JSON(kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(Logger(LoggerConfig{SkipPaths: []string{"/health"}}))
	app.Get("/health", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, getReq("/health", nil))

	// Handler should still be called even though logging is skipped.
	if !called {
		t.Fatal("expected handler to be called even on skip path")
	}
	// Verify the response is still valid.
	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
}

// =====================================================================
// Recovery Tests
// =====================================================================

func TestRecovery_NoPanic(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	resp := serve(t, Recovery(), handler, getReq("/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
	body := parseJSONBody(t, resp.body)
	if body["ok"] != true {
		t.Fatalf("expected ok=true, got %v", body)
	}
}

func TestRecovery_CatchesPanic(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		panic("something went wrong")
	}

	resp := serve(t, Recovery(), handler, getReq("/test", nil))

	if resp.statusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.statusCode)
	}
	body := parseJSONBody(t, resp.body)
	if body["code"] != float64(500) {
		t.Fatalf("expected code=500, got %v", body["code"])
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "internal server error") {
		t.Fatalf("expected internal server error message, got %q", msg)
	}
}

func TestRecovery_CustomPanicHandler(t *testing.T) {
	var panicValue any
	handler := func(c *kruda.Ctx) error {
		panic("custom panic")
	}

	app := kruda.New()
	app.Use(Recovery(RecoveryConfig{
		PanicHandler: func(c *kruda.Ctx, v any) {
			panicValue = v
			c.Status(503)
			_ = c.JSON(kruda.Map{"custom": true})
		},
	}))
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, getReq("/test", nil))

	if panicValue != "custom panic" {
		t.Fatalf("expected panic value 'custom panic', got %v", panicValue)
	}
	if resp.statusCode != 503 {
		t.Fatalf("expected 503, got %d", resp.statusCode)
	}
	body := parseJSONBody(t, resp.body)
	if body["custom"] != true {
		t.Fatalf("expected custom=true, got %v", body)
	}
}

// =====================================================================
// CORS Tests
// =====================================================================

func TestCORS_PreflightRequest(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(CORS())
	app.Get("/test", handler)
	// Register OPTIONS route so the router can find it.
	app.Options("/test", handler)

	req := optionsReq(map[string]string{
		"Origin":                        "http://example.com",
		"Access-Control-Request-Method": "POST",
	})
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	// Preflight should return 204 and NOT call the handler.
	if resp.statusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.statusCode)
	}
	if resp.headers.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected Allow-Origin=*, got %q", resp.headers.Get("Access-Control-Allow-Origin"))
	}
	if resp.headers.Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected Access-Control-Allow-Methods to be set")
	}
	if resp.headers.Get("Access-Control-Max-Age") == "" {
		t.Fatal("expected Access-Control-Max-Age to be set")
	}
}

func TestCORS_NonPreflightRequest(t *testing.T) {
	called := false
	handler := func(c *kruda.Ctx) error {
		called = true
		return c.JSON(kruda.Map{"ok": true})
	}

	req := getReq("/test", map[string]string{"Origin": "http://example.com"})
	resp := serve(t, CORS(), handler, req)

	if !called {
		t.Fatal("expected handler to be called for non-preflight")
	}
	if resp.headers.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected Allow-Origin=*, got %q", resp.headers.Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_DefaultConfig(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(CORS())
	app.Options("/test", handler)

	req := optionsReq(map[string]string{
		"Origin":                        "http://example.com",
		"Access-Control-Request-Method": "GET",
	})
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	methods := resp.headers.Get("Access-Control-Allow-Methods")
	for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if !strings.Contains(methods, m) {
			t.Fatalf("expected default methods to contain %s, got %q", m, methods)
		}
	}
	headers := resp.headers.Get("Access-Control-Allow-Headers")
	for _, h := range []string{"Origin", "Content-Type", "Accept", "Authorization"} {
		if !strings.Contains(headers, h) {
			t.Fatalf("expected default headers to contain %s, got %q", h, headers)
		}
	}
	if resp.headers.Get("Access-Control-Max-Age") != "86400" {
		t.Fatalf("expected MaxAge=86400, got %q", resp.headers.Get("Access-Control-Max-Age"))
	}
}

func TestCORS_AllowCredentialsPanicOnWildcard(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when AllowCredentials=true with wildcard origin")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "AllowCredentials") {
			t.Fatalf("expected panic about AllowCredentials, got %v", r)
		}
	}()

	CORS(CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	})
}

func TestCORS_SpecificOrigin(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	app := kruda.New()
	app.Use(CORS(CORSConfig{
		AllowOrigins: []string{"http://allowed.com"},
	}))
	app.Get("/test", handler)

	// Matching origin.
	req1 := getReq("/test", map[string]string{"Origin": "http://allowed.com"})
	resp1 := newMockResponse()
	app.ServeKruda(resp1, req1)

	if resp1.headers.Get("Access-Control-Allow-Origin") != "http://allowed.com" {
		t.Fatalf("expected Allow-Origin=http://allowed.com, got %q", resp1.headers.Get("Access-Control-Allow-Origin"))
	}

	// Non-matching origin — no CORS headers.
	req2 := getReq("/test", map[string]string{"Origin": "http://evil.com"})
	resp2 := newMockResponse()
	app.ServeKruda(resp2, req2)

	if resp2.headers.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no Allow-Origin for non-matching origin, got %q", resp2.headers.Get("Access-Control-Allow-Origin"))
	}
}

// =====================================================================
// Timeout Tests
// =====================================================================

func TestTimeout_HandlerCompletesInTime(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	resp := serve(t, Timeout(1*time.Second), handler, getReq("/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
	body := parseJSONBody(t, resp.body)
	if body["ok"] != true {
		t.Fatalf("expected ok=true, got %v", body)
	}
}

func TestTimeout_HandlerExceedsTimeout(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		// Simulate a context-aware operation that respects the deadline
		select {
		case <-c.Context().Done():
			// Context timed out — the middleware will handle the 503
			return nil
		case <-time.After(200 * time.Millisecond):
			return c.JSON(kruda.Map{"ok": true})
		}
	}

	resp := serve(t, Timeout(50*time.Millisecond), handler, getReq("/test", nil))

	if resp.statusCode != 503 {
		t.Fatalf("expected 503, got %d", resp.statusCode)
	}
	body := parseJSONBody(t, resp.body)
	if body["code"] != float64(503) {
		t.Fatalf("expected code=503, got %v", body["code"])
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "timeout") {
		t.Fatalf("expected timeout message, got %q", msg)
	}
}
