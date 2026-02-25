package bench

import (
	"net/http/httptest"
	"testing"

	kruda "github.com/go-kruda/kruda"
)

// BenchmarkKruda_NoSecurityHeaders tests performance without security headers
func BenchmarkKruda_NoSecurityHeaders(b *testing.B) {
	app := kruda.New(
		kruda.WithTransportName("nethttp"),
		kruda.WithSecurityHeaders(false), // Disable security headers
	)
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

// BenchmarkKruda_MinimalPath tests with path traversal check disabled
func BenchmarkKruda_MinimalPath(b *testing.B) {
	app := kruda.New(
		kruda.WithTransportName("nethttp"),
		kruda.WithSecurityHeaders(false),
	)
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
			
			// Call ServeKruda but bypass path cleaning by using a simple path
			app.ServeKruda(tw, tr)
		}
	})
}