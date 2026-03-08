// Example: Rate Limiting — Protect API endpoints using contrib/ratelimit
//
// Demonstrates rate limiting with Kruda:
//   - Global rate limit (100 req/min)
//   - Stricter per-route limit on login (5 req/min)
//   - Custom key function (by API key)
//   - Skip function (bypass health checks)
//   - Rate limit response headers
//
// Run: go run ./examples/ratelimit/
// Test:
//
//	curl -v http://localhost:3000/api/data     → check X-RateLimit-* headers
//	for i in $(seq 1 6); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000/api/login; done
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/contrib/ratelimit"
	"github.com/go-kruda/kruda/middleware"
)

func main() {
	app := kruda.New()

	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	// Global rate limit: 100 requests per minute per IP
	app.Use(ratelimit.New(ratelimit.Config{
		Max:    100,
		Window: time.Minute,
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/health" // bypass health checks
		},
	}))

	// Stricter limit on login: 5 requests per minute
	app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))

	app.Get("/health", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"status": "ok"})
	})

	app.Get("/api/data", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"items": []string{"a", "b", "c"}})
	})

	app.Post("/api/login", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"token": "example-token"})
	})

	fmt.Println("Rate limit example listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
