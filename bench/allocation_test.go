package bench

import (
	"net/http/httptest"
	"testing"

	kruda "github.com/go-kruda/kruda"
)

// BenchmarkHttpTestRecorder measures the overhead of httptest.NewRecorder.
func BenchmarkHttpTestRecorder(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			_ = w
		}
	})
}

// BenchmarkTestAdapters measures the overhead of our test adapters.
func BenchmarkTestAdapters(b *testing.B) {
	r := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			tr := newTestRequest(r)
			tw := newTestResponseWriter(w)
			_ = tr
			_ = tw
		}
	})
}

// BenchmarkKrudaJustText measures just the Text() method overhead.
func BenchmarkKrudaJustText(b *testing.B) {
	app := kruda.New()
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
