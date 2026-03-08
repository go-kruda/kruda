package bench

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func BenchmarkEcho_StaticGET(b *testing.B) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(200, "Hello, World!")
	})

	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
		}
	})
}

func BenchmarkEcho_ParamGET(b *testing.B) {
	e := echo.New()
	e.GET("/users/:id", func(c echo.Context) error {
		return c.String(200, c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/users/42", nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
		}
	})
}

func BenchmarkEcho_POSTJSON(b *testing.B) {
	e := echo.New()
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
