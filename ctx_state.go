package kruda

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// Set stores a value in the request-scoped locals.
func (c *Ctx) Set(key string, value any) {
	c.dirty |= dirtyLocals
	c.locals[key] = value
}

// Get retrieves a value from the request-scoped locals.
func (c *Ctx) Get(key string) any {
	return c.locals[key]
}

// Next calls the next handler in the middleware chain.
func (c *Ctx) Next() error {
	c.routeIndex++
	if c.routeIndex < len(c.handlers) {
		return c.handlers[c.routeIndex](c)
	}
	return nil
}

// MarkStart records the request start time for latency measurement.
// Called automatically by the Logger middleware. If not called, Latency() returns 0.
// This avoids the ~30ns cost of time.Now() on every request when timing is not needed.
func (c *Ctx) MarkStart() {
	c.startTime = time.Now()
}

// Latency returns the time elapsed since MarkStart() was called.
// Returns 0 if MarkStart() was never called (no Logger middleware).
func (c *Ctx) Latency() time.Duration {
	if c.startTime.IsZero() {
		return 0
	}
	return time.Since(c.startTime)
}

// bgCtx is cached to avoid repeated context.Background() calls.
var bgCtx = context.Background()

// Context returns the context.Context for this request.
func (c *Ctx) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	// Return cached background context — no allocation.
	return bgCtx
}

// SetContext sets the context.Context for this request.
func (c *Ctx) SetContext(ctx context.Context) {
	c.dirty |= dirtyCtx
	c.ctx = ctx
}

// Log returns a request-scoped logger with pre-set attributes: request_id, method, path.
// The logger is lazy-initialized on first call and cached for the request lifetime.
func (c *Ctx) Log() *slog.Logger {
	if c.logger == nil {
		c.logger = c.app.config.Logger.With(
			"request_id", c.Get("request_id"),
			"method", c.method,
			"path", c.path,
		)
	}
	return c.logger
}

// Provide stores a typed value in the request context for later retrieval via Need.
// This is a semantic alias for Set — it signals intent for dependency injection.
func (c *Ctx) Provide(key string, value any) {
	c.dirty |= dirtyLocals
	c.locals[key] = value
}

// Need retrieves a typed value from the request context.
// Returns the value and true if found and castable to T, or zero value and false otherwise.
// This is a package-level generic function because Go methods cannot have type parameters.
func Need[T any](c *Ctx, key string) (T, bool) {
	val, ok := c.locals[key]
	if !ok {
		var zero T
		return zero, false
	}
	typed, ok := val.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return typed, true
}

// Transport returns the transport type string: "nethttp", "fasthttp", or "wing".
// Contrib modules use this to detect transport-specific behavior (e.g. hijack support).
func (c *Ctx) Transport() string {
	return c.app.transportType
}

// ResponseWriter returns the underlying transport.ResponseWriter.
// Used by contrib modules (e.g. ws) that need direct access to the writer for hijacking.
func (c *Ctx) ResponseWriter() transport.ResponseWriter {
	return c.writer
}

// Request returns the underlying transport.Request.
// Used by contrib modules that need access to the raw request.
func (c *Ctx) Request() transport.Request {
	return c.request
}
