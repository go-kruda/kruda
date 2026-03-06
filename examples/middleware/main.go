// Example: Middleware — Built-in and Custom Middleware
//
// Demonstrates Kruda's middleware system including:
//   - Built-in middleware: Logger, Recovery, CORS, RequestID, Timeout
//   - Custom middleware: authentication, request timing
//   - Middleware ordering and group-scoped middleware
//
// Run: go run -tags kruda_stdjson ./examples/middleware/
// Test: curl -v http://localhost:3000/
//
//	curl -v http://localhost:3000/admin/dashboard -H "Authorization: Bearer secret-token"
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Custom middleware: request timing
// ---------------------------------------------------------------------------

// Timer returns middleware that measures request duration and adds it
// as an X-Response-Time header. This shows how to write middleware that
// wraps the handler chain.
func Timer() kruda.HandlerFunc {
	return func(c *kruda.Ctx) error {
		start := time.Now()

		// Call the next handler in the chain
		err := c.Next()

		// After handler completes, record the duration
		duration := time.Since(start)
		c.SetHeader("X-Response-Time", duration.String())

		return err
	}
}

// ---------------------------------------------------------------------------
// Custom middleware: simple auth
// ---------------------------------------------------------------------------

// Auth returns middleware that checks for a Bearer token in the
// Authorization header. In a real app you'd validate JWTs or session tokens.
func Auth(validToken string) kruda.HandlerFunc {
	return func(c *kruda.Ctx) error {
		auth := c.Header("Authorization")
		if auth == "" {
			return kruda.Unauthorized("missing authorization header")
		}

		// Extract Bearer token
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			// No "Bearer " prefix found
			return kruda.Unauthorized("invalid authorization format, expected: Bearer <token>")
		}

		if token != validToken {
			return kruda.Unauthorized("invalid token")
		}

		// Store user info in context for downstream handlers
		c.Set("user", "admin")
		return c.Next()
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func homeHandler(c *kruda.Ctx) error {
	return c.JSON(kruda.Map{
		"message": "Welcome to Kruda middleware example",
		"endpoints": []string{
			"GET  /              — this page",
			"GET  /public        — public endpoint (no auth)",
			"GET  /admin/dashboard — protected (requires Bearer token)",
			"GET  /admin/users   — protected (requires Bearer token)",
			"GET  /slow          — slow endpoint (demonstrates timeout)",
		},
	})
}

func publicHandler(c *kruda.Ctx) error {
	// RequestID middleware stores the ID in context
	requestID, _ := c.Get("request_id").(string)
	return c.JSON(kruda.Map{
		"message":    "This is a public endpoint",
		"request_id": requestID,
	})
}

func dashboardHandler(c *kruda.Ctx) error {
	user, _ := c.Get("user").(string)
	return c.JSON(kruda.Map{
		"message": fmt.Sprintf("Welcome to the dashboard, %s!", user),
	})
}

func usersHandler(c *kruda.Ctx) error {
	return c.JSON(kruda.Map{
		"users": []string{"alice", "bob", "charlie"},
	})
}

func slowHandler(c *kruda.Ctx) error {
	// Simulate a slow operation — Timeout middleware will cancel if too slow
	select {
	case <-time.After(500 * time.Millisecond):
		return c.JSON(kruda.Map{"message": "completed (was slow but within timeout)"})
	case <-c.Context().Done():
		return kruda.NewError(503, "request timed out")
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	app := kruda.New()

	// -----------------------------------------------------------------------
	// Global middleware — applied to ALL routes in registration order.
	// Order matters: Recovery should be first to catch panics from any layer.
	// -----------------------------------------------------------------------

	// 1. Recovery catches panics and returns 500 instead of crashing
	app.Use(middleware.Recovery())

	// 2. Logger logs each request with method, path, status, and duration
	app.Use(middleware.Logger())

	// 3. RequestID ensures every request has a unique X-Request-ID header
	app.Use(middleware.RequestID())

	// 4. CORS allows cross-origin requests from specific origins
	app.Use(middleware.CORS(middleware.CORSConfig{
		AllowOrigins: []string{"https://example.com", "https://app.example.com"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       3600,
	}))

	// 5. Custom timer middleware — measures response time
	app.Use(Timer())

	// -----------------------------------------------------------------------
	// Public routes — no auth required
	// -----------------------------------------------------------------------
	app.Get("/", homeHandler)
	app.Get("/public", publicHandler)

	// Slow endpoint with a per-route timeout.
	// Since app.Get takes a single handler, wrap the timeout as a group.
	slow := app.Group("/slow")
	slow.Use(middleware.Timeout(2 * time.Second))
	slow.Get("", slowHandler)

	// -----------------------------------------------------------------------
	// Admin group — Auth middleware scoped to this group only.
	// Group middleware runs AFTER global middleware but BEFORE group handlers.
	// -----------------------------------------------------------------------
	admin := app.Group("/admin")
	admin.Use(Auth("secret-token"))

	admin.Get("/dashboard", dashboardHandler)
	admin.Get("/users", usersHandler)

	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
