package bench

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// ---------------------------------------------------------------------------
// Gin Benchmarks
// ---------------------------------------------------------------------------

// BenchmarkGin_StaticGET benchmarks a simple static GET route.
func BenchmarkGin_StaticGET(b *testing.B) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, World!")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkGin_ParamGET benchmarks a parameterized GET route.
func BenchmarkGin_ParamGET(b *testing.B) {
	r := gin.New()
	r.GET("/users/:id", func(c *gin.Context) {
		c.String(200, c.Param("id"))
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/users/42", nil)
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkGin_POSTJSON benchmarks POST with JSON body parsing and response.
func BenchmarkGin_POSTJSON(b *testing.B) {
	r := gin.New()
	r.POST("/users", func(c *gin.Context) {
		var user struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, user)
	})

	jsonBody := []byte(`{"name":"John","email":"john@example.com"}`)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkGin_Middleware1 benchmarks 1 no-op middleware.
func BenchmarkGin_Middleware1(b *testing.B) {
	benchGinMiddleware(b, 1)
}

// BenchmarkGin_Middleware5 benchmarks 5 no-op middleware.
func BenchmarkGin_Middleware5(b *testing.B) {
	benchGinMiddleware(b, 5)
}

// BenchmarkGin_Middleware10 benchmarks 10 no-op middleware.
func BenchmarkGin_Middleware10(b *testing.B) {
	benchGinMiddleware(b, 10)
}

func benchGinMiddleware(b *testing.B, n int) {
	b.Helper()
	r := gin.New()
	for i := 0; i < n; i++ {
		r.Use(func(c *gin.Context) { c.Next() })
	}
	r.GET("/", func(c *gin.Context) {
		c.String(200, "ok")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			r.ServeHTTP(w, req)
		}
	})
}

// BenchmarkGin_JSONEncode benchmarks JSON encoding throughput.
func BenchmarkGin_JSONEncode(b *testing.B) {
	r := gin.New()
	r.GET("/json", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Hello, World!"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json", nil)
			r.ServeHTTP(w, req)
		}
	})
}
