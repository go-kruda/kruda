package kruda

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// App is the main framework struct that holds config, router, middleware,
// hooks, error mappings, transport, and the context pool.
type App struct {
	config     Config
	router     *Router
	middleware []HandlerFunc
	hooks      Hooks
	errorMap   map[error]ErrorMapping
	transport  transport.Transport
	ctxPool    sync.Pool
	routeInfos []routeInfo // collected from typed handler registrations for OpenAPI
}

// New creates a new App with default config and applies the provided options.
// If no Transport option is given, it defaults to the net/http transport
// configured with the App's timeouts and body limit.
func New(opts ...Option) *App {
	app := &App{
		config:   defaultConfig(),
		router:   newRouter(),
		errorMap: defaultErrorMap(),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}

	// Default transport if none was set via options
	if app.config.Transport == nil {
		app.config.Transport = transport.NewNetHTTP(transport.NetHTTPConfig{
			ReadTimeout:    app.config.ReadTimeout,
			WriteTimeout:   app.config.WriteTimeout,
			IdleTimeout:    app.config.IdleTimeout,
			MaxBodySize:    app.config.BodyLimit,
			MaxHeaderBytes: app.config.HeaderLimit,
			TrustProxy:     app.config.TrustProxy,
		})
	}
	app.transport = app.config.Transport

	// Set up context pool
	app.ctxPool = sync.Pool{
		New: func() any {
			return newCtx(app)
		},
	}

	return app
}

// ServeKruda implements transport.Handler. It acquires a Ctx from the pool,
// finds the matching route, executes the handler chain, handles errors,
// and releases the Ctx back to the pool.
// M4: includes panic recovery to prevent server crashes.
func (app *App) ServeKruda(w transport.ResponseWriter, r transport.Request) {
	c := app.ctxPool.Get().(*Ctx)
	c.reset(w, r)
	defer func() {
		// M4: recover from panics not caught by Recovery middleware
		if rec := recover(); rec != nil {
			app.config.Logger.Error("unrecovered panic in ServeKruda", "panic", fmt.Sprintf("%v", rec))
			if !c.responded {
				c.Status(500)
				_ = c.JSON(Map{
					"code":    500,
					"message": "internal server error",
				})
			}
		}
		c.cleanup()
		app.ctxPool.Put(c)
	}()

	// 1. Execute OnRequest hooks
	for _, hook := range app.hooks.OnRequest {
		if err := hook(c); err != nil {
			app.handleError(c, err)
			return
		}
	}

	// 2. Find route
	handlers := app.router.find(c.method, c.path, c.params)
	if handlers == nil {
		// Check 405 Method Not Allowed
		if allowed := app.router.findAllowedMethods(c.path); allowed != "" {
			c.SetHeader("Allow", allowed)
			app.handleError(c, NewError(405, "method not allowed"))
			return
		}
		// 404 Not Found
		app.handleError(c, NotFound("not found"))
		return
	}

	// 3. Execute handler chain (middleware + handler)
	c.handlers = handlers
	c.routeIndex = 0
	if err := c.handlers[0](c); err != nil {
		app.handleError(c, err)
	}

	// 4. Execute OnResponse hooks (errors are ignored)
	for _, hook := range app.hooks.OnResponse {
		_ = hook(c)
	}
}

// --- Route registration methods (all return *App for chaining) ---
// H1 fix: removed unused ...HookConfig parameter from all route methods.
// Per-route hooks will be re-added when BeforeHandle/AfterHandle are implemented.

// Get registers a GET route.
func (app *App) Get(path string, handler HandlerFunc) *App {
	app.addRoute("GET", path, handler)
	return app
}

// Post registers a POST route.
func (app *App) Post(path string, handler HandlerFunc) *App {
	app.addRoute("POST", path, handler)
	return app
}

// Put registers a PUT route.
func (app *App) Put(path string, handler HandlerFunc) *App {
	app.addRoute("PUT", path, handler)
	return app
}

// Delete registers a DELETE route.
func (app *App) Delete(path string, handler HandlerFunc) *App {
	app.addRoute("DELETE", path, handler)
	return app
}

// Patch registers a PATCH route.
func (app *App) Patch(path string, handler HandlerFunc) *App {
	app.addRoute("PATCH", path, handler)
	return app
}

// Options registers an OPTIONS route.
func (app *App) Options(path string, handler HandlerFunc) *App {
	app.addRoute("OPTIONS", path, handler)
	return app
}

// Head registers a HEAD route.
func (app *App) Head(path string, handler HandlerFunc) *App {
	app.addRoute("HEAD", path, handler)
	return app
}

// All registers a route on all standard HTTP methods.
func (app *App) All(path string, handler HandlerFunc) *App {
	for _, method := range standardMethods {
		app.addRoute(method, path, handler)
	}
	return app
}

// addRoute builds the pre-built handler chain and registers the route.
func (app *App) addRoute(method, path string, handler HandlerFunc) {
	chain := buildChain(app.middleware, nil, handler)
	app.router.addRoute(method, path, chain)
}

// Use appends global middleware to the App.
// H8: middleware added via Use() only applies to routes registered AFTER this call.
// Routes registered before Use() will not have this middleware in their chain.
func (app *App) Use(middleware ...HandlerFunc) *App {
	app.middleware = append(app.middleware, middleware...)
	return app
}

// Listen compiles the router, starts the transport in a goroutine,
// and blocks waiting for SIGINT or SIGTERM to initiate graceful shutdown.
func (app *App) Listen(addr string) error {
	// Build and serve OpenAPI spec if configured (must be before Compile)
	if app.config.openAPIInfo.Title != "" {
		specJSON, err := app.buildOpenAPISpec()
		if err != nil {
			return fmt.Errorf("kruda: failed to build OpenAPI spec: %w", err)
		}
		app.Get(app.config.openAPIPath, func(c *Ctx) error {
			c.SetHeader("Content-Type", "application/json")
			c.SetHeader("Cache-Control", "public, max-age=3600")
			return c.sendBytes(specJSON)
		})
	}

	app.router.Compile()

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.transport.ListenAndServe(addr, app)
	}()

	// M3: log happens before server is fully ready; this is a known limitation.
	// The server may still be binding the port when this log fires.
	app.config.Logger.Info("listening", "addr", addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err // server failed to start
	case <-sigCh:
		// graceful shutdown
	}

	return app.shutdown()
}

// shutdown performs graceful shutdown: drains connections, runs OnShutdown hooks in LIFO order.
func (app *App) shutdown() error {
	timeout := app.config.ShutdownTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	app.config.Logger.Info("shutting down...", "timeout", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := app.transport.Shutdown(ctx)

	app.runShutdownHooks()

	app.config.Logger.Info("shutdown complete")
	return err
}

// Shutdown initiates graceful shutdown programmatically (useful for tests).
// M1 fix: now also runs OnShutdown hooks in LIFO order.
func (app *App) Shutdown(ctx context.Context) error {
	err := app.transport.Shutdown(ctx)
	app.runShutdownHooks()
	return err
}

// runShutdownHooks executes OnShutdown hooks in LIFO order (like defer).
// NEW-3 fix: extracted from shutdown() and Shutdown() to eliminate duplication.
// Each hook is wrapped in recover so a panicking hook doesn't prevent others from running.
func (app *App) runShutdownHooks() {
	for i := len(app.hooks.OnShutdown) - 1; i >= 0; i-- {
		func(fn func()) {
			defer func() {
				if r := recover(); r != nil {
					app.config.Logger.Error("panic in OnShutdown hook", "panic", fmt.Sprintf("%v", r))
				}
			}()
			fn()
		}(app.hooks.OnShutdown[i])
	}
}

// OnShutdown registers a cleanup function to be called during shutdown.
// Functions are executed in LIFO order (last registered, first called).
func (app *App) OnShutdown(fn func()) *App {
	app.hooks.OnShutdown = append(app.hooks.OnShutdown, fn)
	return app
}

// OnParse registers a hook that fires after input parsing but before validation.
// The hook receives the parsed input as `any` — type-assert to the specific type.
// Returning an error stops the pipeline (skips validation and handler).
func (app *App) OnParse(fn func(c *Ctx, input any) error) *App {
	app.hooks.OnParse = append(app.hooks.OnParse, fn)
	return app
}

// Validator returns the app's Validator instance, creating one if needed.
// Use this to register custom rules or override messages.
func (app *App) Validator() *Validator {
	if app.config.Validator == nil {
		app.config.Validator = NewValidator()
	}
	return app.config.Validator
}

// handleError converts an error to a KrudaError, executes OnError hooks,
// and sends the appropriate JSON error response.
//
// ValidationError handling: when a validation error occurs, handleError wraps it
// in a *KrudaError{Code: 422, Message: "Validation failed", Err: ve}. If a custom
// ErrorHandler is configured, it receives this *KrudaError. To access the underlying
// *ValidationError from the handler, use:
//
//	var ve *kruda.ValidationError
//	if errors.As(ke.Unwrap(), &ve) {
//	    // ve.Errors contains per-field validation failures
//	}
//
// F1 fix: checks c.Responded() before writing to avoid double-write when
// a middleware or handler has already sent a response.
func (app *App) handleError(c *Ctx, err error) {
	// Check for ValidationError first — it has its own JSON structure
	var ve *ValidationError
	if errors.As(err, &ve) {
		// OnError hooks still fire for validation errors
		ke := &KrudaError{Code: 422, Message: "Validation failed", Err: ve}
		for _, hook := range app.hooks.OnError {
			hook(c, ke)
		}

		// F1: don't write if response already sent
		if c.Responded() {
			return
		}

		// Use custom error handler if configured
		if app.config.ErrorHandler != nil {
			app.config.ErrorHandler(c, ke)
			return
		}

		// Default: use ValidationError's own JSON marshaling
		c.Status(422)
		data, _ := ve.MarshalJSON()
		c.SetHeader("Content-Type", "application/json; charset=utf-8")
		_ = c.sendBytes(data)
		return
	}

	ke := app.resolveError(err)

	// Execute OnError hooks (always fire, even if already responded,
	// so logging/metrics hooks still work)
	for _, hook := range app.hooks.OnError {
		hook(c, ke)
	}

	// F1: don't write if response already sent
	if c.Responded() {
		return
	}

	// Use custom error handler if configured
	if app.config.ErrorHandler != nil {
		app.config.ErrorHandler(c, ke)
		return
	}

	// Default: set status and send JSON error response
	c.Status(ke.Code)
	_ = c.JSON(ke)
}
