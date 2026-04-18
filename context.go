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

// Map is a convenience type alias for map[string]any.
// Use it for quick JSON responses without defining a struct.
//
//	c.JSON(kruda.Map{"message": "hello", "ok": true})
type Map = map[string]any

