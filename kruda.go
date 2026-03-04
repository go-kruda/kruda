package kruda

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"strconv"
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

	// DI container (nil by default = zero overhead)
	container  *Container
	errorTypes []errorTypeMapping
	errorFuncs []errorFuncMapping

	// Precomputed security headers (nil if SecurityHeaders=false)
	secHeaders [][2]string

	// hasLifecycle is true when any request lifecycle hook is registered.
	// Set once at Compile() time. ServeFast() checks this single bool
	// to skip the entire lifecycle path — zero-cost when no hooks.
	hasLifecycle bool

	// transportType identifies the active transport: "nethttp", "fasthttp", or "wing".
	// Set once during New() based on the resolved transport selection.
	transportType string
}

// New creates a new App with default config and applies the provided options.
// If no Transport option is given, it defaults to Wing (epoll/kqueue) on
// Linux and macOS, and net/http on Windows.
func New(opts ...Option) *App {
	app := &App{
		config:   defaultConfig(),
		router:   newRouter(),
		errorMap: defaultErrorMap(),
	}

	for _, opt := range opts {
		opt(app)
	}

	if !app.config.devModeSet && os.Getenv("KRUDA_ENV") == "development" {
		app.config.DevMode = true
	}

	if app.config.DevMode {
		app.config.Security.XFrameOptions = "SAMEORIGIN"
		app.registerPprofRoutes()
	}

	app.transport, app.transportType = selectTransport(app.config, app.config.Logger)

	app.ctxPool = sync.Pool{
		New: func() any {
			return newCtx(app)
		},
	}

	return app
}

// Compile freezes the router tree and applies AOT optimizations.
// Called automatically by Listen(). Useful for benchmarks and tests
// that call ServeKruda directly without Listen().
func (app *App) Compile() {
	app.router.Compile()

	// Precompute security headers with canonical keys
	if app.config.SecurityHeaders {
		sec := app.config.Security
		var headers [][2]string

		if sec.XSSProtection != "" {
			headers = append(headers, [2]string{textproto.CanonicalMIMEHeaderKey("X-XSS-Protection"), sec.XSSProtection})
		}
		if sec.ContentTypeNosniff != "" {
			headers = append(headers, [2]string{textproto.CanonicalMIMEHeaderKey("X-Content-Type-Options"), sec.ContentTypeNosniff})
		}
		if sec.XFrameOptions != "" {
			headers = append(headers, [2]string{textproto.CanonicalMIMEHeaderKey("X-Frame-Options"), sec.XFrameOptions})
		}
		if sec.ReferrerPolicy != "" {
			headers = append(headers, [2]string{textproto.CanonicalMIMEHeaderKey("Referrer-Policy"), sec.ReferrerPolicy})
		}
		if sec.ContentSecurityPolicy != "" {
			headers = append(headers, [2]string{textproto.CanonicalMIMEHeaderKey("Content-Security-Policy"), sec.ContentSecurityPolicy})
		}
		if sec.HSTSMaxAge > 0 {
			headers = append(headers, [2]string{textproto.CanonicalMIMEHeaderKey("Strict-Transport-Security"), "max-age=" + strconv.Itoa(sec.HSTSMaxAge)})
		}

		app.secHeaders = headers
	}

	// Set hasLifecycle flag — ServeFast checks this single bool to decide
	// whether to run the full lifecycle pipeline or the fast path.
	app.hasLifecycle = len(app.hooks.OnRequest) > 0 ||
		len(app.hooks.OnResponse) > 0 ||
		len(app.hooks.BeforeHandle) > 0 ||
		len(app.hooks.AfterHandle) > 0 ||
		len(app.hooks.OnError) > 0
}

// containsDotPercent checks if path contains . or % using a fast byte scan.
func containsDotPercent(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '%' {
			return true
		}
	}
	return false
}

// ServeHTTP implements http.Handler for fast net/http path.
// Uses lightweight adapters and avoids defer overhead.
func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := app.ctxPool.Get().(*Ctx)

	c.embeddedReq.r = r
	c.embeddedReq.maxBody = app.config.BodyLimit
	c.embeddedReq.bodyRead = false
	c.embeddedReq.body = nil
	c.embeddedReq.queryDone = false
	c.embeddedResp.w = w
	c.embeddedResp.statusCode = 200
	c.embeddedResp.written = false
	c.embeddedResp.headerMap = nil

	if c.multipartForm != nil {
		_ = c.multipartForm.RemoveAll()
		c.multipartForm = nil
	}

	// Minimal inline reset — ONLY hot fields
	c.method = r.Method
	c.path = r.URL.Path
	c.status = 200
	c.responded = false
	c.bodyParsed = false
	c.bodyBytes = nil
	c.bodyErr = nil
	c.routeIndex = 0
	c.handlers = nil
	c.writer = &c.embeddedResp
	c.request = &c.embeddedReq
	c.contentType = ""
	c.contentLength = -1
	c.cacheControl = ""
	c.location = ""
	c.body = nil
	if len(c.respHeaders) > 0 {
		clear(c.respHeaders)
	}
	if len(c.cookies) > 0 {
		c.cookies = c.cookies[:0]
	}
	if len(c.headers) > 0 {
		clear(c.headers)
	}
	if len(c.locals) > 0 {
		clear(c.locals)
	}

	if len(c.path) == 0 {
		c.path = "/"
	}

	// Path traversal prevention (opt-in via WithPathTraversal or WithSecurity)
	if app.config.PathTraversal && len(c.path) > 1 && containsDotPercent(c.path) {
		if cleaned, err := cleanPath(c.path); err != nil {
			app.handleError(c, NewError(400, "bad request: "+err.Error()))
			c.shrinkMaps()
			app.ctxPool.Put(c)
			return
		} else {
			c.path = cleaned
		}
	}

	if c.params.count > 0 {
		c.params.reset()
	}

	handlers := app.router.find(c.method, c.path, &c.params)
	if handlers == nil {
		if allowed := app.router.findAllowedMethods(c.path); allowed != "" {
			c.SetHeader("Allow", allowed)
			app.handleError(c, NewError(405, "method not allowed"))
		} else {
			app.handleError(c, NotFound("not found"))
		}
		c.shrinkMaps()
		app.ctxPool.Put(c)
		return
	}

	c.handlers = handlers
	if err := c.handlers[0](c); err != nil {
		app.handleError(c, err)
	}

	if c.body != nil && !c.responded {
		c.responded = true
		c.contentLength = len(c.body)
		c.writeHeaders()
		c.writer.WriteHeader(c.status)
		c.writer.Write(c.body)
		c.body = nil
	}

	c.shrinkMaps()
	app.ctxPool.Put(c)
}

// ServeKruda implements transport.Handler. Includes panic recovery to prevent
// server crashes from unhandled panics not caught by Recovery middleware.
func (app *App) ServeKruda(w transport.ResponseWriter, r transport.Request) {
	c := app.ctxPool.Get().(*Ctx)
	c.reset(w, r)
	defer func() {
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
	if app.config.PathTraversal {
		cleaned, err := cleanPath(c.path)
		if err != nil {
			w.WriteHeader(400)
			_, _ = w.Write([]byte("Bad Request"))
			return
		}
		c.path = cleaned
	}

	// OnRequest hooks — fire before route matching.
	for _, hook := range app.hooks.OnRequest {
		if err := hook(c); err != nil {
			app.handleError(c, err)
			return
		}
	}

	handlers := app.router.find(c.method, c.path, &c.params)
	if handlers == nil {
		if allowed := app.router.findAllowedMethods(c.path); allowed != "" {
			c.SetHeader("Allow", allowed)
			app.handleError(c, NewError(405, "method not allowed"))
		} else {
			app.handleError(c, NotFound("not found"))
		}
		goto response
	}

	c.handlers = handlers
	c.routeIndex = 0

	// BeforeHandle hooks — fire after middleware chain is set, before handler.
	for _, hook := range app.hooks.BeforeHandle {
		if err := hook(c); err != nil {
			app.handleError(c, err)
			goto afterHandle
		}
	}

	if err := c.handlers[0](c); err != nil {
		app.handleError(c, err)
	}

afterHandle:
	// AfterHandle hooks — fire after handler, before response flush.
	for _, hook := range app.hooks.AfterHandle {
		if err := hook(c); err != nil {
			app.handleError(c, err)
		}
	}

	// Flush lazy body before OnResponse hooks so hooks can inspect the response.
	if c.body != nil && !c.responded {
		_ = c.send()
	}

response:
	// OnResponse hooks — fire after body flush.
	for _, hook := range app.hooks.OnResponse {
		_ = hook(c)
	}
}

// Get registers a GET route.
func (app *App) Get(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("GET", path, handler, opts...)
	return app
}

// Post registers a POST route.
func (app *App) Post(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("POST", path, handler, opts...)
	return app
}

// Put registers a PUT route.
func (app *App) Put(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("PUT", path, handler, opts...)
	return app
}

// Delete registers a DELETE route.
func (app *App) Delete(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("DELETE", path, handler, opts...)
	return app
}

// Patch registers a PATCH route.
func (app *App) Patch(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("PATCH", path, handler, opts...)
	return app
}

// Options registers an OPTIONS route.
func (app *App) Options(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("OPTIONS", path, handler, opts...)
	return app
}

// Head registers a HEAD route.
func (app *App) Head(path string, handler HandlerFunc, opts ...RouteOption) *App {
	app.addRoute("HEAD", path, handler, opts...)
	return app
}

// All registers a route on all standard HTTP methods.
func (app *App) All(path string, handler HandlerFunc, opts ...RouteOption) *App {
	for _, method := range standardMethods {
		app.addRoute(method, path, handler, opts...)
	}
	return app
}

// addRoute builds the pre-built handler chain and registers the route.
func (app *App) addRoute(method, path string, handler HandlerFunc, opts ...RouteOption) {
	chain := buildChain(app.middleware, nil, handler)
	app.router.addRoute(method, path, chain)

	// Apply Wing feather if transport supports it.
	if len(opts) > 0 {
		var rc routeConfig
		for _, o := range opts {
			o(&rc)
		}
		if rc.wingFeather != nil {
			if fc, ok := app.transport.(transport.FeatherConfigurator); ok {
				fc.SetRouteFeather(method, path, rc.wingFeather)
			}
		}
	}
}

// Use appends global middleware to the App.
// Middleware added via Use() only applies to routes registered AFTER this call.
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

	// Use optimized listener (TCP_DEFER_ACCEPT + TCP_FASTOPEN on Linux,
	// plain net.Listen on other platforms), then hand it to the transport.
	ln, err := optimizedListener(addr)
	if err != nil {
		return fmt.Errorf("kruda: listen: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.transport.Serve(ln, app)
	}()

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

	if app.container != nil {
		if err := app.container.Shutdown(ctx); err != nil {
			app.config.Logger.Error("container shutdown error", "error", err)
		}
	}

	err := app.transport.Shutdown(ctx)
	app.runShutdownHooks()
	app.config.Logger.Info("shutdown complete")
	return err
}

// Shutdown initiates graceful shutdown programmatically (useful for tests).
// Also runs OnShutdown hooks in LIFO order.
func (app *App) Shutdown(ctx context.Context) error {
	if app.container != nil {
		if err := app.container.Shutdown(ctx); err != nil {
			app.config.Logger.Error("container shutdown error", "error", err)
		}
	}

	err := app.transport.Shutdown(ctx)
	app.runShutdownHooks()
	return err
}

// runShutdownHooks executes OnShutdown hooks in LIFO order (like defer).
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

// OnRequest registers a hook that fires when a request arrives, before route matching.
// Use for logging, request ID injection, rate limiting, or early request validation.
// Returning an error stops the pipeline — the error handler runs and no routing occurs.
func (app *App) OnRequest(fn HookFunc) *App {
	app.hooks.OnRequest = append(app.hooks.OnRequest, fn)
	return app
}

// OnResponse registers a hook that fires after the handler completes (including AfterHandle).
// Use for response logging, metrics collection, or cleanup. Errors are logged but do not
// affect the response (it has already been written).
func (app *App) OnResponse(fn HookFunc) *App {
	app.hooks.OnResponse = append(app.hooks.OnResponse, fn)
	return app
}

// BeforeHandle registers a hook that fires after middleware but before the final handler.
// Use for auth checks, permission verification, or request decoration.
// Returning an error stops the pipeline — the handler is NOT called.
func (app *App) BeforeHandle(fn HookFunc) *App {
	app.hooks.BeforeHandle = append(app.hooks.BeforeHandle, fn)
	return app
}

// AfterHandle registers a hook that fires after the handler returns, before OnResponse.
// Use for response transformation, caching, or post-processing.
// Returning an error triggers the error handler.
func (app *App) AfterHandle(fn HookFunc) *App {
	app.hooks.AfterHandle = append(app.hooks.AfterHandle, fn)
	return app
}

// OnError registers a hook that fires when an error occurs during request processing.
// Receives the context and the error. Always fires — even if a response was already sent —
// so logging and metrics hooks work reliably.
func (app *App) OnError(fn ErrorHookFunc) *App {
	app.hooks.OnError = append(app.hooks.OnError, fn)
	return app
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

// handleError converts an error to a KrudaError, fires OnError hooks,
// and sends the appropriate JSON error response.
//
// To access the underlying *ValidationError from a custom ErrorHandler:
//
//	var ve *kruda.ValidationError
//	if errors.As(ke.Unwrap(), &ve) { ... }
func (app *App) handleError(c *Ctx, err error) {
	// Check for ValidationError first — it has its own JSON structure
	var ve *ValidationError
	if errors.As(err, &ve) {
		// OnError hooks still fire for validation errors
		ke := &KrudaError{Code: 422, Message: "Validation failed", Err: ve}
		for _, hook := range app.hooks.OnError {
			hook(c, ke)
		}

		// Don't write if response already sent
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

	// R7.7: Always log the full unsanitized error regardless of DevMode
	slog.Error("request error",
		"method", c.Method(),
		"path", c.Path(),
		"status", ke.Code,
		"error", err.Error(),
	)

	// Execute OnError hooks (always fire, even if already responded,
	// so logging/metrics hooks still work)
	for _, hook := range app.hooks.OnError {
		hook(c, ke)
	}

	// Don't write if response already sent
	if c.Responded() {
		return
	}

	// Try dev error page first (only in DevMode)
	if app.config.DevMode {
		if devErr := renderDevErrorPage(c, err, ke.Code); devErr != nil {
			return // dev page rendered successfully
		}
	}

	// R7.1-R7.6: Sanitize error response in production mode
	if !app.config.DevMode {
		var krudaErr *KrudaError
		isKrudaError := errors.As(err, &krudaErr)

		if !isKrudaError && !ke.mapped {
			// R7.6: Raw Go error (not KrudaError, not mapped) → replace message with HTTP status text
			ke.Message = http.StatusText(ke.Code)
			if ke.Message == "" {
				ke.Message = "Internal Server Error"
			}
			ke.Detail = ""
		} else if ke.Code >= 500 {
			// R7.2: KrudaError or mapped error with 5xx → preserve user-facing Message, strip Detail
			ke.Detail = ""
		}
		// R7.5: KrudaError/mapped error with 4xx → preserve Message and Detail as-is (user-facing)
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

// HookFunc is a lifecycle hook function.
type HookFunc func(c *Ctx) error

// ErrorHookFunc is an error lifecycle hook function.
type ErrorHookFunc func(c *Ctx, err error)

// Hooks holds all lifecycle hook slices.
//
// Full lifecycle order (see spec §10.2):
//
//	OnRequest → OnParse → Middleware → BeforeHandle → Handler → AfterHandle → OnResponse → OnError
//
// BeforeHandle fires after middleware but before the final handler.
// AfterHandle fires after the handler returns (before OnResponse).
// All hooks are zero-cost when not registered — ServeFast checks a single bool flag.
type Hooks struct {
	OnRequest    []HookFunc
	OnResponse   []HookFunc
	BeforeHandle []HookFunc
	AfterHandle  []HookFunc
	OnError      []ErrorHookFunc
	OnShutdown   []func()
	OnParse      []func(c *Ctx, input any) error
}

// MiddlewareFunc is a type alias for HandlerFunc for semantic clarity.
// Using a type alias (=) means MiddlewareFunc and HandlerFunc are interchangeable.
type MiddlewareFunc = HandlerFunc

// buildChain creates a pre-built handler chain from global, group, and route-level handlers.
// Called once at registration time — the slice is reused for every request (zero alloc on hot path).
func buildChain(global, group []HandlerFunc, handler HandlerFunc) []HandlerFunc {
	chain := make([]HandlerFunc, 0, len(global)+len(group)+1)
	chain = append(chain, global...)
	chain = append(chain, group...)
	chain = append(chain, handler)
	return chain
}
