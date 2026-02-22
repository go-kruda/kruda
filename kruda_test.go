package kruda

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// ---------------------------------------------------------------------------
// Mock transport types for App core tests
// ---------------------------------------------------------------------------

// mockRequest implements transport.Request.
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

// mockResponseWriter implements transport.ResponseWriter.
type mockResponseWriter struct {
	statusCode int
	headers    mockHeaderMap
	body       []byte
}

func newMockResponse() *mockResponseWriter {
	return &mockResponseWriter{headers: mockHeaderMap{h: make(map[string]string)}}
}

func (w *mockResponseWriter) WriteHeader(code int)        { w.statusCode = code }
func (w *mockResponseWriter) Header() transport.HeaderMap { return &w.headers }
func (w *mockResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

// mockHeaderMap implements transport.HeaderMap.
type mockHeaderMap struct {
	h map[string]string
}

func (m *mockHeaderMap) Set(key, value string) { m.h[key] = value }
func (m *mockHeaderMap) Get(key string) string { return m.h[key] }
func (m *mockHeaderMap) Add(key, value string) {
	if existing := m.h[key]; existing != "" {
		m.h[key] = existing + ", " + value
	} else {
		m.h[key] = value
	}
}
func (m *mockHeaderMap) Del(key string) { delete(m.h, key) }

// ---------------------------------------------------------------------------
// Req 5.1, 5.2: New() defaults and functional options
// ---------------------------------------------------------------------------

func TestNew_Defaults(t *testing.T) {
	app := New()

	if app == nil {
		t.Fatal("New() returned nil")
	}
	if app.router == nil {
		t.Error("router should not be nil")
	}
	if app.transport == nil {
		t.Error("transport should not be nil")
	}
	if app.errorMap == nil {
		t.Error("errorMap should not be nil")
	}

	// Default config values
	if app.config.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", app.config.ReadTimeout)
	}
	if app.config.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", app.config.WriteTimeout)
	}
	if app.config.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", app.config.IdleTimeout)
	}
	if app.config.BodyLimit != 4*1024*1024 {
		t.Errorf("BodyLimit = %d, want 4MB", app.config.BodyLimit)
	}
	if app.config.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", app.config.ShutdownTimeout)
	}
	if app.config.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if app.config.JSONEncoder == nil {
		t.Error("JSONEncoder should not be nil")
	}
	if app.config.JSONDecoder == nil {
		t.Error("JSONDecoder should not be nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	app := New(
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(10*time.Second),
		WithBodyLimit(1024),
		WithShutdownTimeout(30*time.Second),
	)

	if app.config.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", app.config.ReadTimeout)
	}
	if app.config.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", app.config.WriteTimeout)
	}
	if app.config.BodyLimit != 1024 {
		t.Errorf("BodyLimit = %d, want 1024", app.config.BodyLimit)
	}
	if app.config.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", app.config.ShutdownTimeout)
	}
}

// ---------------------------------------------------------------------------
// Req 5.6: Route registration methods
// ---------------------------------------------------------------------------

func TestApp_RouteRegistration(t *testing.T) {
	app := New()
	h := func(c *Ctx) error { return nil }

	app.Get("/get", h)
	app.Post("/post", h)
	app.Put("/put", h)
	app.Delete("/delete", h)
	app.Patch("/patch", h)

	params := make(map[string]string, 4)
	methods := map[string]string{
		"GET":    "/get",
		"POST":   "/post",
		"PUT":    "/put",
		"DELETE": "/delete",
		"PATCH":  "/patch",
	}
	for method, path := range methods {
		clear(params)
		if app.router.find(method, path, params) == nil {
			t.Errorf("%s %s should be registered", method, path)
		}
	}
}

// ---------------------------------------------------------------------------
// Req 5.7: Use() appends global middleware
// ---------------------------------------------------------------------------

func TestApp_Use(t *testing.T) {
	app := New()
	mw1 := func(c *Ctx) error { return c.Next() }
	mw2 := func(c *Ctx) error { return c.Next() }

	app.Use(mw1, mw2)

	if len(app.middleware) != 2 {
		t.Fatalf("middleware count = %d, want 2", len(app.middleware))
	}
}

// ---------------------------------------------------------------------------
// Req 5.6: Method chaining — route methods return *App
// ---------------------------------------------------------------------------

func TestApp_MethodChaining(t *testing.T) {
	app := New()
	h := func(c *Ctx) error { return nil }

	ret := app.Get("/a", h).Post("/b", h).Put("/c", h).Delete("/d", h).Patch("/e", h)
	if ret != app {
		t.Error("chained route methods should return the same *App")
	}
}

// ---------------------------------------------------------------------------
// Req 5.4, 5.5: ServeKruda — success path
// ---------------------------------------------------------------------------

func TestApp_ServeKruda_Success(t *testing.T) {
	app := New()

	called := false
	app.Get("/hello", func(c *Ctx) error {
		called = true
		return c.JSON(Map{"msg": "ok"})
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/hello"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if !called {
		t.Error("handler should have been called")
	}
	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}
	// Verify JSON body
	var body map[string]any
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["msg"] != "ok" {
		t.Errorf("body[msg] = %v, want ok", body["msg"])
	}
}

// ---------------------------------------------------------------------------
// Req 5.5: ServeKruda — 404 Not Found
// ---------------------------------------------------------------------------

func TestApp_ServeKruda_404(t *testing.T) {
	app := New()
	app.Get("/exists", func(c *Ctx) error { return c.Text("ok") })
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/nope"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 404 {
		t.Errorf("status = %d, want 404", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 404 {
		t.Errorf("body.Code = %d, want 404", body.Code)
	}
}

// ---------------------------------------------------------------------------
// Req 5.5: ServeKruda — 405 Method Not Allowed
// ---------------------------------------------------------------------------

func TestApp_ServeKruda_405(t *testing.T) {
	app := New()
	app.Get("/resource", func(c *Ctx) error { return c.Text("ok") })
	app.router.Compile()

	req := &mockRequest{method: "POST", path: "/resource"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 405 {
		t.Errorf("status = %d, want 405", resp.statusCode)
	}

	// Verify Allow header is set
	allow := resp.headers.h["Allow"]
	if !strings.Contains(allow, "GET") {
		t.Errorf("Allow header = %q, want to contain GET", allow)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 405 {
		t.Errorf("body.Code = %d, want 405", body.Code)
	}
}

// ---------------------------------------------------------------------------
// Req 5.12: handleError — error handling produces correct JSON
// ---------------------------------------------------------------------------

func TestApp_HandleError(t *testing.T) {
	app := New()

	// Register a route that returns a KrudaError
	app.Get("/fail", func(c *Ctx) error {
		return BadRequest("invalid input")
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Errorf("status = %d, want 400", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 400 {
		t.Errorf("body.Code = %d, want 400", body.Code)
	}
	if body.Message != "invalid input" {
		t.Errorf("body.Message = %q, want %q", body.Message, "invalid input")
	}
}

// ---------------------------------------------------------------------------
// Req 5.10: OnShutdown registers hooks
// ---------------------------------------------------------------------------

func TestApp_OnShutdown(t *testing.T) {
	app := New()

	called := 0
	app.OnShutdown(func() { called++ })
	app.OnShutdown(func() { called++ })

	if len(app.hooks.OnShutdown) != 2 {
		t.Fatalf("OnShutdown hooks = %d, want 2", len(app.hooks.OnShutdown))
	}

	// Execute hooks to verify they work
	for _, fn := range app.hooks.OnShutdown {
		fn()
	}
	if called != 2 {
		t.Errorf("called = %d, want 2", called)
	}
}

// ---------------------------------------------------------------------------
// Req 5.6: All() registers on all standard methods
// ---------------------------------------------------------------------------

func TestApp_All(t *testing.T) {
	app := New()
	app.All("/any", func(c *Ctx) error { return nil })

	params := make(map[string]string, 4)
	for _, method := range standardMethods {
		clear(params)
		if app.router.find(method, "/any", params) == nil {
			t.Errorf("%s /any should be registered via All()", method)
		}
	}
}

// ---------------------------------------------------------------------------
// Req 5.5, 5.7: ServeKruda with global middleware — execution order
// ---------------------------------------------------------------------------

func TestApp_ServeKruda_WithMiddleware(t *testing.T) {
	app := New()

	var order []string

	app.Use(func(c *Ctx) error {
		order = append(order, "mw1")
		return c.Next()
	})
	app.Use(func(c *Ctx) error {
		order = append(order, "mw2")
		return c.Next()
	})
	app.Get("/test", func(c *Ctx) error {
		order = append(order, "handler")
		return c.JSON(Map{"ok": true})
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}

	want := []string{"mw1", "mw2", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order[%d] = %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}
