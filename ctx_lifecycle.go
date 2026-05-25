package kruda

import (
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
	c.dirty = 0
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

	// Keep request context lazy. Most hot-path handlers never call Context(),
	// and Ctx.Context() can read the transport request context on demand.
	c.ctx = nil

	// Reset params (inline array, zero-alloc)
	if c.params.count > 0 {
		c.params.reset()
	}

	c.logger = nil
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
	if d := c.dirty; d != 0 {
		if d&(dirtyHeaders|dirtyRespHdrs|dirtyLocals) != 0 {
			c.shrinkMaps()
		}
		if d&dirtyHeaders != 0 {
			clear(c.headers)
		}
		if d&dirtyRespHdrs != 0 {
			clear(c.respHeaders)
		}
		if d&dirtyLocals != 0 {
			clear(c.locals)
		}
		if d&dirtyCookies != 0 {
			c.cookies = c.cookies[:0]
		}
		if d&dirtyBody != 0 {
			c.body = nil
		}
		if d&dirtyBodyBytes != 0 {
			c.bodyBytes = nil
			c.bodyErr = nil
		}
		if d&dirtyMultipart != 0 && c.multipartForm != nil {
			_ = c.multipartForm.RemoveAll()
			c.multipartForm = nil
		}
	}

	c.writer = nil
	c.request = nil
	c.handlers = nil
	c.ctx = nil
	c.logger = nil
	c.dirty = 0
}
