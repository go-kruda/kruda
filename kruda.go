package kruda

import (
	"context"
	"fmt"
	"log/slog"
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

	startupMu         sync.Mutex
	openAPIRegistered bool
	containerStarted  bool
}

// New creates a new App with default config and applies the provided options.
// If no Transport option is given, it defaults to Wing (epoll+eventfd) on
// Linux, fasthttp on macOS, and net/http on Windows.
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
	if err := app.compile(); err != nil {
		panic(err)
	}
}

func (app *App) compile() error {
	if err := app.registerOpenAPI(); err != nil {
		return err
	}

	app.router.Compile()
	app.prepareCompiledState()
	return nil
}

func (app *App) registerOpenAPI() error {
	app.startupMu.Lock()
	defer app.startupMu.Unlock()

	if app.openAPIRegistered || app.config.openAPIInfo.Title == "" {
		return nil
	}

	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		return fmt.Errorf("kruda: failed to build OpenAPI spec: %w", err)
	}
	app.Get(app.config.openAPIPath, func(c *Ctx) error {
		c.SetHeader("Content-Type", "application/json")
		c.SetHeader("Cache-Control", "public, max-age=3600")
		return c.sendBytes(specJSON)
	})
	app.openAPIRegistered = true
	return nil
}

func (app *App) prepareCompiledState() {
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

func (app *App) startContainer(ctx context.Context) error {
	if app.container == nil {
		return nil
	}

	app.startupMu.Lock()
	defer app.startupMu.Unlock()

	if app.containerStarted {
		return nil
	}
	if err := app.container.Start(ctx); err != nil {
		return err
	}
	app.containerStarted = true
	return nil
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

	// Apply Wing preset if transport supports it.
	if len(opts) > 0 {
		var rc routeConfig
		for _, o := range opts {
			o.applyRoute(&rc)
		}
		if rc.preset != nil {
			if fc, ok := app.transport.(transport.PresetConfigurator); ok {
				f := *rc.preset
				f.handlers = chain
				fc.SetRoutePreset(method, path, &f)
				eff := f
				eff.defaults()
				slog.Debug("kruda: route preset",
					"route", method+" "+path,
					"dispatch", eff.Dispatch.String())
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
	if err := app.compile(); err != nil {
		return err
	}

	// Use optimized listener (TCP_DEFER_ACCEPT + TCP_FASTOPEN on Linux,
	// plain net.Listen on other platforms), then hand it to the transport.
	ln, err := optimizedListener(addr)
	if err != nil {
		return fmt.Errorf("kruda: listen: %w", err)
	}

	if err := app.startContainer(context.Background()); err != nil {
		_ = ln.Close()
		return err
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
