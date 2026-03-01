package bench

import (
	"bytes"
	"net/http/httptest"
	"testing"

	kruda "github.com/go-kruda/kruda"
)

// Kruda benchmarks now use ServeHTTP for fair comparison with Echo/Gin

func BenchmarkKruda_StaticGET(b *testing.B) {
	app := kruda.New(kruda.NetHTTP())
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
			app.ServeHTTP(w, r)
		}
	})
}

func BenchmarkKruda_ParamGET(b *testing.B) {
	app := kruda.New(kruda.NetHTTP())
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
			app.ServeHTTP(w, r)
		}
	})
}

func BenchmarkKruda_POSTJSON(b *testing.B) {
	app := kruda.New(kruda.NetHTTP())
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
			app.ServeHTTP(w, r)
		}
	})
}

func BenchmarkKruda_Middleware5(b *testing.B) {
	app := kruda.New(kruda.NetHTTP())
	for i := 0; i < 5; i++ {
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
			app.ServeHTTP(w, r)
		}
	})
}
