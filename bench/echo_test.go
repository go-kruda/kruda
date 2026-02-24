package bench

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// ---------------------------------------------------------------------------
// Echo Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkEcho_StaticGET benchmarks a simple static GET route.
func BenchmarkEcho_StaticGET(b *testing.B) {
	e := echo.New()
	e.HideBanner = true
	e.GET("/", func(c echo.Context) error {
		return c.String(200, "Hello, World!")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			e.ServeHTTP(w, req)
		}
	})
}

// BenchmarkEcho_ParamGET benchmarks a parameterized GET route.
func BenchmarkEcho_ParamGET(b *testing.B) {
	e := echo.New()
	e.HideBanner = true
	e.GET("/users/:id", func(c echo.Context) error {
		return c.String(200, c.Param("id"))
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/users/42", nil)
			e.ServeHTTP(w, req)
		}
	})
}

// BenchmarkEcho_POSTJSON benchmarks POST with JSON body parsing and response.
func BenchmarkEcho_POSTJSON(b *testing.B) {
	e := echo.New()
	e.HideBanner = true
	e.POST("/users", func(c echo.Context) error {
		var user struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := c.Bind(&user); err != nil {
			return err
		}
		return c.JSON(200, user)
	})

	jsonBody := []byte(`{"name":"John","email":"john@example.com"}`)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			e.ServeHTTP(w, req)
		}
	})
}

// BenchmarkEcho_Middleware1 benchmarks 1 no-op middleware.
func BenchmarkEcho_Middleware1(b *testing.B) {
	benchEchoMiddleware(b, 1)
}

// BenchmarkEcho_Middleware5 benchmarks 5 no-op middleware.
func BenchmarkEcho_Middleware5(b *testing.B) {
	benchEchoMiddleware(b, 5)
}

// BenchmarkEcho_Middleware10 benchmarks 10 no-op middleware.
func BenchmarkEcho_Middleware10(b *testing.B) {
	benchEchoMiddleware(b, 10)
}

func benchEchoMiddleware(b *testing.B, n int) {
	b.Helper()
	e := echo.New()
	e.HideBanner = true
	for i := 0; i < n; i++ {
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { return next(c) }
		})
	}
	e.GET("/", func(c echo.Context) error {
		return c.String(200, "ok")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			e.ServeHTTP(w, req)
		}
	})
}

// BenchmarkEcho_JSONEncode benchmarks JSON encoding throughput.
func BenchmarkEcho_JSONEncode(b *testing.B) {
	e := echo.New()
	e.HideBanner = true
	e.GET("/json", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"message": "Hello, World!"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json", nil)
			e.ServeHTTP(w, req)
		}
	})
}
