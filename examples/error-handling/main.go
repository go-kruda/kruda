// Example: Error Handling — Error Mapping, KrudaError, Custom Error Handlers
//
// Demonstrates Kruda's error handling system:
//   - KrudaError: structured errors with HTTP status codes
//   - MapError: map sentinel errors to HTTP responses
//   - MapErrorType: map error types to HTTP responses (generics)
//   - MapErrorFunc: custom error transformation functions
//   - WithErrorHandler: fully custom error response format
//
// Run: go run -tags kruda_stdjson ./examples/error-handling/
// Test:
//
//	curl http://localhost:3000/users/999          → 404 mapped from sentinel
//	curl http://localhost:3000/users/0            → 422 mapped from error type
//	curl -X POST http://localhost:3000/users      → 409 mapped via MapErrorFunc
//	curl http://localhost:3000/admin              → 403 KrudaError directly
//	curl http://localhost:3000/panic              → 500 caught by Recovery
package main

import (
	"errors"
	"fmt"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Domain errors — these are your application-level errors, not HTTP errors.
// Kruda maps them to HTTP responses automatically.
// ---------------------------------------------------------------------------

// Sentinel errors — use errors.New for simple error values.
// These are matched by errors.Is (value equality).
var (
	ErrUserNotFound = errors.New("user not found")
	ErrDuplicate    = errors.New("duplicate entry")
)

// ValidationError is a typed error with structured field information.
// Matched by errors.As (type matching) via MapErrorType.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %s — %s", e.Field, e.Message)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func getUserHandler(c *kruda.Ctx) error {
	id := c.Param("id")

	// Simulate: ID "999" triggers not-found sentinel error.
	// MapError will convert this to HTTP 404 automatically.
	if id == "999" {
		return ErrUserNotFound
	}

	// Simulate: ID "0" triggers a validation error.
	// MapErrorType will convert this to HTTP 422 automatically.
	if id == "0" {
		return &ValidationError{Field: "id", Message: "must be a positive integer"}
	}

	return c.JSON(kruda.Map{
		"id":   id,
		"name": "Alice",
	})
}

func createUserHandler(c *kruda.Ctx) error {
	// Simulate: creating a user that already exists.
	// MapErrorFunc will transform this with custom logic.
	return fmt.Errorf("user alice: %w", ErrDuplicate)
}

func adminHandler(c *kruda.Ctx) error {
	// Return a KrudaError directly — this bypasses all error mapping
	// and goes straight to the response with the specified status code.
	return kruda.Forbidden("admin access requires elevated privileges")
}

func panicHandler(_ *kruda.Ctx) error {
	// Recovery middleware catches this and returns 500
	panic("something went terribly wrong")
}

func homeHandler(c *kruda.Ctx) error {
	return c.JSON(kruda.Map{
		"message": "Error handling example",
		"try": kruda.Map{
			"GET /users/1":   "success — returns user",
			"GET /users/999": "404 — sentinel error mapped via MapError",
			"GET /users/0":   "422 — type error mapped via MapErrorType",
			"POST /users":    "409 — transformed via MapErrorFunc",
			"GET /admin":     "403 — direct KrudaError",
			"GET /panic":     "500 — panic caught by Recovery middleware",
		},
	})
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	app := kruda.New(
		kruda.NetHTTP(),
		// Custom error handler — controls the JSON shape of ALL error responses.
		// Without this, Kruda uses its default: {"code": N, "message": "..."}
		kruda.WithErrorHandler(func(c *kruda.Ctx, err *kruda.KrudaError) {
			c.Status(err.Code).JSON(kruda.Map{
				"error": kruda.Map{
					"code":    err.Code,
					"message": err.Message,
					"detail":  err.Detail,
				},
			})
		}),
	)

	app.Use(middleware.Recovery()) // Catches panics → 500
	app.Use(middleware.Logger())   // Logs requests

	// -----------------------------------------------------------------------
	// Error Mapping — register BEFORE routes
	// -----------------------------------------------------------------------

	// 1. MapError: map a sentinel error value to an HTTP response.
	//    When any handler returns ErrUserNotFound (or wraps it),
	//    Kruda responds with 404 and the specified message.
	app.MapError(ErrUserNotFound, 404, "the requested user was not found")

	// 2. MapErrorType: map an error TYPE to an HTTP response.
	//    When any handler returns a *ValidationError (or wraps one),
	//    Kruda responds with 422 and the specified message.
	//    Note: this is a free function because Go doesn't support generic methods.
	kruda.MapErrorType[*ValidationError](app, 422, "validation error")

	// 3. MapErrorFunc: custom error transformation with full control.
	//    When any handler returns an error wrapping ErrDuplicate,
	//    this function produces the KrudaError response.
	kruda.MapErrorFunc(app, ErrDuplicate, func(err error) *kruda.KrudaError {
		return &kruda.KrudaError{
			Code:    409,
			Message: "resource already exists",
			Detail:  err.Error(), // includes the wrapped context
		}
	})

	// -----------------------------------------------------------------------
	// Routes
	// -----------------------------------------------------------------------
	app.Get("/", homeHandler)
	app.Get("/users/:id", getUserHandler)
	app.Post("/users", createUserHandler)
	app.Get("/admin", adminHandler)
	app.Get("/panic", panicHandler)

	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
