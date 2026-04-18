package kruda

import (
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// headerIntern caches canonical header keys to reduce allocations.
// Capped at maxHeaderInternEntries to prevent memory DoS from adversarial
// requests with randomized header keys.
var headerIntern sync.Map

// maxHeaderInternEntries limits the number of cached canonical header keys.
// Typical apps use <30 unique header keys; 256 is generous headroom.
const maxHeaderInternEntries = 256

// headerInternCount tracks the number of entries in headerIntern.
var headerInternCount atomic.Int64

// internHeader returns the canonical form of a header key, using cache.
// Once the cache reaches maxHeaderInternEntries, new keys are computed
// on-the-fly without caching to prevent unbounded memory growth.
func internHeader(key string) string {
	if v, ok := headerIntern.Load(key); ok {
		return v.(string)
	}
	canonical := http.CanonicalHeaderKey(key)
	if headerInternCount.Load() < maxHeaderInternEntries {
		if _, loaded := headerIntern.LoadOrStore(key, canonical); !loaded {
			headerInternCount.Add(1)
		}
	}
	return canonical
}

// HandlerFunc is the function signature for route handlers and middleware.
type HandlerFunc func(c *Ctx) error

// Cookie represents an HTTP cookie.
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	Expires  time.Time // optional; if set, emits Expires header for legacy client compat
	MaxAge   int
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
}

// Ctx is the request context, pooled via sync.Pool for zero allocation.
// All string values are proper copies — safe to use anywhere.
//
// Field ordering is optimized for CPU cache line alignment.
// maxRouteParams is the maximum number of path parameters per route.
// 8 covers virtually all real-world routes (e.g. /api/v1/:org/:repo/:id).
const maxRouteParams = 8

// RouteParam is a key-value pair for a single path parameter.
type RouteParam struct {
	Key   string
	Value string
}

// routeParams is a fixed-size array of path parameters, avoiding map overhead.
// Linear scan on ≤8 items is faster than map hash+lookup due to cache locality.
type routeParams struct {
	items   [maxRouteParams]RouteParam
	count   int
	pattern string // matched route pattern (e.g. "/users/:id"), set by find()
}

// set adds or updates a param. Returns the routeParams for chaining.
func (p *routeParams) set(key, value string) {
	// Update existing key (for router backtrack overwrite)
	for i := 0; i < p.count; i++ {
		if p.items[i].Key == key {
			p.items[i].Value = value
			return
		}
	}
	if p.count < maxRouteParams {
		p.items[p.count] = RouteParam{Key: key, Value: value}
		p.count++
	}
}

// get returns the value for a key, or "" if not found.
func (p *routeParams) get(key string) string {
	for i := 0; i < p.count; i++ {
		if p.items[i].Key == key {
			return p.items[i].Value
		}
	}
	return ""
}

// del removes a param by key (used during router backtracking).
func (p *routeParams) del(key string) {
	for i := 0; i < p.count; i++ {
		if p.items[i].Key == key {
			// Shift remaining items left
			p.count--
			p.items[i] = p.items[p.count]
			p.items[p.count] = RouteParam{} // zero out for GC
			return
		}
	}
}

// reset clears all params without allocation.
// Only resets count — the next find() will overwrite used slots.
// String headers in items are tiny (backed by fasthttp arena or interned)
// and will be overwritten before the next read, so no GC leak risk.
func (p *routeParams) reset() {
	p.count = 0
	p.pattern = ""
}

// dirtyFlags tracks which cold fields were modified during a request.
// Only dirty fields are cleaned up when the context returns to the pool.
// This eliminates branch checks on every request for fields that most handlers never touch.
type dirtyFlags uint8

const (
	dirtyHeaders   dirtyFlags = 1 << iota // c.headers was written
	dirtyRespHdrs                         // c.respHeaders was written
	dirtyLocals                           // c.locals was written
	dirtyCookies                          // c.cookies was appended
	dirtyBody                             // c.body was set (lazy response path)
	dirtyBodyBytes                        // c.bodyBytes was read (body parsing)
	dirtyCtx                              // c.ctx was set
	dirtyMultipart                        // c.multipartForm was used
)

// Hot fields accessed every request are packed into the first cache lines
// to minimize cache misses on the hot path.
type Ctx struct {
	// === Cache line 1 (0–63 bytes): Hot fields — accessed every request ===
	app        *App       // 8 bytes  (pointer)
	method     string     // 16 bytes (string header)
	path       string     // 16 bytes (string header)
	status     int        // 8 bytes
	routeIndex int        // 8 bytes  (current position in handler chain)
	responded  bool       // 1 byte
	dirty      dirtyFlags // 1 byte — tracks which cold fields need cleanup
	// Total: 58 bytes + 6 padding = 64 bytes

	// === Cache line 2 (64–127 bytes): Handler chain + transport ===
	handlers []HandlerFunc            // 24 bytes (slice header)
	writer   transport.ResponseWriter // 16 bytes (interface)
	request  transport.Request        // 16 bytes (interface)
	params   routeParams              // inline fixed-size array — zero-alloc param storage
	// Note: routeParams is ~264 bytes but accessed every param request, cache-hot

	// === Cache line 3 (128–191 bytes): Fixed-slot response headers ===
	// Avoids map write + slice allocation for the most common headers.
	contentType   string // 16 bytes — Content-Type (set by JSON(), Text(), HTML(), SetHeader())
	contentLength int    // 8 bytes  — Content-Length (set by sendBytes(), int avoids strconv until write)
	cacheControl  string // 16 bytes — Cache-Control (set by SetHeader())
	location      string // 16 bytes — Location (set by Redirect(), SetHeader())
	bodyParsed    bool   // 1 byte + 7 padding
	// Total: 64 bytes

	// === Cache line 4+ / Cold region (192+ bytes): accessed only when needed ===
	body          []byte
	headers       map[string]string
	respHeaders   map[string][]string // multi-value header support
	cookies       []*Cookie           // separate slice for multi-cookie support
	locals        map[string]any
	bodyBytes     []byte
	bodyErr       error // preserved body read error
	ctx           context.Context
	startTime     time.Time
	logger        *slog.Logger    // lazy-init, cached per request
	multipartForm *multipart.Form // cleanup reference (RemoveAll temp files)

	// === Embedded adapters (end of struct, largest, least directly accessed) ===
	embeddedReq       fastNetHTTPRequest
	embeddedResp      fastNetHTTPResponseWriter
	ctxFastHTTPFields // platform-specific, empty on Windows
}

// newCtx creates a new context with pre-allocated maps.
func newCtx(app *App) *Ctx {
	return &Ctx{
		app:           app,
		headers:       make(map[string]string, 8),
		respHeaders:   make(map[string][]string, 8),
		locals:        make(map[string]any, 4),
		cookies:       make([]*Cookie, 0, 4),
		status:        200,
		contentLength: -1,
		// params is zero-value routeParams — no allocation needed
	}
}

// reset prepares the context for reuse from the pool.
func (c *Ctx) reset(w transport.ResponseWriter, r transport.Request) {
	c.method = r.Method()
	c.path = r.Path()
	c.status = 200
	c.responded = false
	c.bodyParsed = false
	c.routeIndex = 0
	c.handlers = nil
	c.startTime = time.Time{}
	c.writer = w
	c.request = r
	// routePattern is reset via params.reset() above

	// Reset fixed-slot headers (zero-cost, no allocation)
	c.contentType = ""
	c.contentLength = -1
	c.cacheControl = ""
	c.location = ""

	// Only set context if the request provides one
	c.ctx = nil

	// Reset params (inline array, zero-alloc)
	if c.params.count > 0 {
		c.params.reset()
	}

	if len(c.headers) > 0 {
		clear(c.headers)
	}
	if len(c.respHeaders) > 0 {
		clear(c.respHeaders)
	}
	if len(c.locals) > 0 {
		clear(c.locals)
	}
	if len(c.cookies) > 0 {
		c.cookies = c.cookies[:0]
	}

	c.body = nil
	c.bodyBytes = nil
	c.bodyErr = nil
	c.logger = nil
	c.multipartForm = nil
}

// Pool shrink thresholds — maps exceeding these entry counts are reallocated
// to initial size on cleanup to prevent unbounded pool memory growth.
const (
	maxHeadersCapacity     = 32 // initial: 8
	maxRespHeadersCapacity = 32 // initial: 8
	maxLocalsCapacity      = 16 // initial: 4
)

// shrinkMaps replaces oversized maps with fresh small ones.
// Called before pool.Put to prevent unbounded pool memory growth.
// Note: params no longer needs shrinking (fixed-size inline array).
func (c *Ctx) shrinkMaps() {
	if len(c.headers) > maxHeadersCapacity {
		c.headers = make(map[string]string, 8)
	}
	if len(c.respHeaders) > maxRespHeadersCapacity {
		c.respHeaders = make(map[string][]string, 8)
	}
	if len(c.locals) > maxLocalsCapacity {
		c.locals = make(map[string]any, 4)
	}
}

// cleanup prepares the context for returning to the pool.
func (c *Ctx) cleanup() {
	// Remove multipart temp files now that the handler is done.
	if c.multipartForm != nil {
		_ = c.multipartForm.RemoveAll()
		c.multipartForm = nil
	}

	// Shrink oversized maps via shared method.
	c.shrinkMaps()

	c.body = nil
	c.writer = nil
	c.request = nil
	c.bodyBytes = nil
	c.handlers = nil
	c.ctx = nil
	c.logger = nil
}

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

// SSE starts a Server-Sent Events stream.
// Sets appropriate headers and creates an SSEStream for the callback.
// Returns when the callback returns or the client disconnects.
func (c *Ctx) SSE(fn func(*SSEStream) error) error {
	// Check flusher support before writing headers
	flusher, ok := c.writer.(http.Flusher)
	if !ok {
		return InternalError("SSE requires a transport that supports flushing")
	}

	// Set SSE headers
	c.SetHeader("Content-Type", "text/event-stream")
	c.SetHeader("Cache-Control", "no-cache")
	c.SetHeader("Connection", "keep-alive")

	// Write headers immediately
	c.writeHeaders()
	c.writer.WriteHeader(200)
	c.responded = true

	stream := &SSEStream{
		writer:  writerAdapter{c.writer},
		flusher: flusher,
		encode:  c.app.config.JSONEncoder,
		ctx:     c.Context(),
	}

	err := fn(stream)

	// Flush after callback returns to ensure net/http sends the terminating chunk.
	flusher.Flush()

	return err
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

// Map is a convenience type alias for map[string]any.
// Use it for quick JSON responses without defining a struct.
//
//	c.JSON(kruda.Map{"message": "hello", "ok": true})
type Map = map[string]any

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
