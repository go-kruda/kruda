package bench

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// ---------------------------------------------------------------------------
// Fiber Benchmarks
// ---------------------------------------------------------------------------

// fiberHandler returns a Fiber app as an http.Handler via fiber.App.Test.
// Fiber doesn't implement http.Handler natively, so we use its built-in
// Test method which processes requests without network I/O.

// BenchmarkFiber_StaticGET benchmarks a simple static GET route.
func BenchmarkFiber_StaticGET(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkFiber_ParamGET benchmarks a parameterized GET route.
func BenchmarkFiber_ParamGET(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/users/:id", func(c *fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/users/42", nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkFiber_POSTJSON benchmarks POST with JSON body parsing and response.
func BenchmarkFiber_POSTJSON(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/users", func(c *fiber.Ctx) error {
		var user struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := c.BodyParser(&user); err != nil {
			return err
		}
		return c.JSON(user)
	})

	jsonBody := []byte(`{"name":"John","email":"john@example.com"}`)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkFiber_Middleware1 benchmarks 1 no-op middleware.
func BenchmarkFiber_Middleware1(b *testing.B) {
	benchFiberMiddleware(b, 1)
}

// BenchmarkFiber_Middleware5 benchmarks 5 no-op middleware.
func BenchmarkFiber_Middleware5(b *testing.B) {
	benchFiberMiddleware(b, 5)
}

// BenchmarkFiber_Middleware10 benchmarks 10 no-op middleware.
func BenchmarkFiber_Middleware10(b *testing.B) {
	benchFiberMiddleware(b, 10)
}

func benchFiberMiddleware(b *testing.B, n int) {
	b.Helper()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	for i := 0; i < n; i++ {
		app.Use(func(c *fiber.Ctx) error { return c.Next() })
	}
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkFiber_JSONEncode benchmarks JSON encoding throughput.
func BenchmarkFiber_JSONEncode(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/json", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello, World!"})
	})

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/json", nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}
