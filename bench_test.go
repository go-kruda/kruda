package kruda

import (
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// ---------------------------------------------------------------------------
// Benchmark helpers
// ---------------------------------------------------------------------------

// benchRequest is a minimal transport.Request for benchmarks (zero alloc).
type benchRequest struct {
	method string
	path   string
}

func (r *benchRequest) Method() string           { return r.method }
func (r *benchRequest) Path() string             { return r.path }
func (r *benchRequest) Header(string) string     { return "" }
func (r *benchRequest) Body() ([]byte, error)    { return nil, nil }
func (r *benchRequest) QueryParam(string) string { return "" }
func (r *benchRequest) RemoteAddr() string       { return "127.0.0.1" }
func (r *benchRequest) Cookie(string) string     { return "" }
func (r *benchRequest) RawRequest() any          { return nil }

// benchResponseWriter is a no-op transport.ResponseWriter for benchmarks.
type benchResponseWriter struct{}

func (w *benchResponseWriter) WriteHeader(int)                {}
func (w *benchResponseWriter) Header() transport.HeaderMap    { return &benchHeaderMap{} }
func (w *benchResponseWriter) Write(data []byte) (int, error) { return len(data), nil }

type benchHeaderMap struct{}

func (m *benchHeaderMap) Set(string, string) {}
func (m *benchHeaderMap) Add(string, string) {}
func (m *benchHeaderMap) Get(string) string  { return "" }
func (m *benchHeaderMap) Del(string)         {}

// ---------------------------------------------------------------------------
// Benchmarks: raw framework overhead (ServeKruda hot path)
// ---------------------------------------------------------------------------

// BenchmarkPlaintext measures the simplest possible handler — c.Text("Hello, World!")
// This is the standard TechEmpower plaintext benchmark pattern.
func BenchmarkPlaintext(b *testing.B) {
	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.Text("Hello, World!")
	})
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkJSON measures JSON serialization overhead.
func BenchmarkJSON(b *testing.B) {
	app := New()
	app.Get("/json", func(c *Ctx) error {
		return c.JSON(Map{"message": "Hello, World!"})
	})
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/json"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkRouterStatic measures static route lookup with 100 routes registered.
func BenchmarkRouterStatic(b *testing.B) {
	app := New()
	paths := []string{
		"/users", "/users/list", "/users/create", "/users/update", "/users/delete",
		"/posts", "/posts/list", "/posts/create", "/posts/update", "/posts/delete",
		"/api/v1/users", "/api/v1/posts", "/api/v1/comments",
		"/api/v2/users", "/api/v2/posts", "/api/v2/comments",
		"/health", "/ready", "/metrics", "/debug/pprof",
	}
	handler := func(c *Ctx) error { return c.Text("ok") }
	for _, p := range paths {
		app.Get(p, handler)
	}
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/api/v1/comments"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkRouterParam measures parameterized route lookup.
func BenchmarkRouterParam(b *testing.B) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return c.Text(c.Param("id"))
	})
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/users/42"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkMiddleware1 measures overhead of 1 middleware in the chain.
func BenchmarkMiddleware1(b *testing.B) {
	app := New()
	app.Use(func(c *Ctx) error { return c.Next() })
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkMiddleware5 measures overhead of 5 middleware in the chain.
func BenchmarkMiddleware5(b *testing.B) {
	app := New()
	noop := func(c *Ctx) error { return c.Next() }
	for i := 0; i < 5; i++ {
		app.Use(noop)
	}
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkTypedHandler measures typed handler with param + query binding.
func BenchmarkTypedHandler(b *testing.B) {
	app := New()
	type In struct {
		ID int `param:"id"`
	}
	type Out struct {
		ID int `json:"id"`
	}
	Get[In, Out](app, "/users/:id", func(c *C[In]) (*Out, error) {
		return &Out{ID: c.In.ID}, nil
	})
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/users/42"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}

// BenchmarkContextPoolAcquireRelease measures context pool get/put cycle overhead.
func BenchmarkContextPoolAcquireRelease(b *testing.B) {
	app := New()
	req := &benchRequest{method: "GET", path: "/"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := app.ctxPool.Get().(*Ctx)
		c.reset(w, req)
		c.cleanup()
		app.ctxPool.Put(c)
	}
}

// BenchmarkTypedHandlerValidation measures typed handler with validation.
func BenchmarkTypedHandlerValidation(b *testing.B) {
	app := New(WithValidator(NewValidator()))
	type In struct {
		ID int `param:"id" validate:"required,min=1"`
	}
	type Out struct {
		ID int `json:"id"`
	}
	Get[In, Out](app, "/users/:id", func(c *C[In]) (*Out, error) {
		return &Out{ID: c.In.ID}, nil
	})
	app.router.Compile()

	req := &benchRequest{method: "GET", path: "/users/42"}
	w := &benchResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.ServeKruda(w, req)
	}
}
