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

func BenchmarkGin_StaticGET(b *testing.B) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, World!")
	})

	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkGin_ParamGET(b *testing.B) {
	r := gin.New()
	r.GET("/users/:id", func(c *gin.Context) {
		c.String(200, c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/42", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkGin_POSTJSON(b *testing.B) {
	r := gin.New()
	r.POST("/users", func(c *gin.Context) {
		var user struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&user); err != nil {
			c.Status(400)
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
