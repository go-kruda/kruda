//go:build kruda_stdjson || !sonic_avail

package bench

import (
	"bytes"
	"net/http/httptest"
	"testing"

	kruda "github.com/go-kruda/kruda"
)

// ---------------------------------------------------------------------------
// Kruda Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkKruda_StaticGET benchmarks a simple static GET route returning text.
func BenchmarkKruda_StaticGET(b *testing.B) {
	app := kruda.New(kruda.WithTransportName("nethttp"))
	app.Get("/", func(c *kruda.Ctx) error {
		return c.Text("Hello, World!")
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			tr := newTestRequest(r)
			tw := newTestResponseWriter(w)
			app.ServeKruda(tw, tr)
		}
	})
}

// BenchmarkKruda_ParamGET benchmarks a parameterized GET route with param extraction.
func BenchmarkKruda_ParamGET(b *testing.B) {
	app := kruda.New(kruda.WithTransportName("nethttp"))
	app.Get("/users/:id", func(c *kruda.Ctx) error {
		return c.Text(c.Param("id"))
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/users/42", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			tr := newTestRequest(r)
			tw := newTestResponseWriter(w)
			app.ServeKruda(tw, tr)
		}
	})
}

// BenchmarkKruda_POSTJSON benchmarks POST with JSON body parsing and JSON response.
func BenchmarkKruda_POSTJSON(b *testing.B) {
	app := kruda.New(kruda.WithTransportName("nethttp"))
	app.Post("/users", func(c *kruda.Ctx) error {
		var user struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := c.Bind(&user); err != nil {
			return err
		}
		return c.JSON(user)
	})
	app.Compile()

	jsonBody := []byte(`{"name":"John","email":"john@example.com"}`)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonBody))
			r.Header.Set("Content-Type", "application/json")
			tr := newTestRequest(r)
			tw := newTestResponseWriter(w)
			app.ServeKruda(tw, tr)
		}
	})
}

// BenchmarkKruda_Middleware1 benchmarks 1 no-op middleware in the chain.
func BenchmarkKruda_Middleware1(b *testing.B) {
	benchKrudaMiddleware(b, 1)
}

// BenchmarkKruda_Middleware5 benchmarks 5 no-op middleware in the chain.
func BenchmarkKruda_Middleware5(b *testing.B) {
	benchKrudaMiddleware(b, 5)
}

// BenchmarkKruda_Middleware10 benchmarks 10 no-op middleware in the chain.
func BenchmarkKruda_Middleware10(b *testing.B) {
	benchKrudaMiddleware(b, 10)
}

func benchKrudaMiddleware(b *testing.B, n int) {
	b.Helper()
	app := kruda.New(kruda.WithTransportName("nethttp"))
	for i := 0; i < n; i++ {
		app.Use(func(c *kruda.Ctx) error { return c.Next() })
	}
	app.Get("/", func(c *kruda.Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			tr := newTestRequest(r)
			tw := newTestResponseWriter(w)
			app.ServeKruda(tw, tr)
		}
	})
}

// BenchmarkKruda_JSONEncode benchmarks JSON encoding throughput.
func BenchmarkKruda_JSONEncode(b *testing.B) {
	app := kruda.New(kruda.WithTransportName("nethttp"))
	app.Get("/json", func(c *kruda.Ctx) error {
		return c.JSON(map[string]string{"message": "Hello, World!"})
	})
	app.Compile()

	r := httptest.NewRequest("GET", "/json", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			tr := newTestRequest(r)
			tw := newTestResponseWriter(w)
			app.ServeKruda(tw, tr)
		}
	})
}
