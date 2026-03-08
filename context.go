package kruda

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// responseBufPool pools []byte buffers for small responses (≤4KB).
// Using *[]byte so we can update the slice header (append may grow it).
var responseBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 4096)
		return &buf
	},
}

// jsonBufPool pools bytes.Buffer for JSON marshal output.
// The streaming encoder writes directly into this buffer, avoiding the
// fresh []byte allocation that Marshal() creates each call.
// Typical JSON API response is 50–500 bytes; the buffer grows as needed
// and is reused across requests.
var jsonBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

// jsonContentType is a pre-allocated byte slice for the JSON content type header.
// Used by both the fasthttp fast path (SetContentTypeBytes) and generic paths.
var jsonContentType = []byte("application/json; charset=utf-8")

// headerIntern caches canonical header keys to reduce allocations.
var headerIntern sync.Map

// internHeader returns the canonical form of a header key, using cache.
func internHeader(key string) string {
	if v, ok := headerIntern.Load(key); ok {
		return v.(string)
	}
	canonical := http.CanonicalHeaderKey(key)
	headerIntern.Store(key, canonical)
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

// Method returns the HTTP method (GET, POST, etc.).
func (c *Ctx) Method() string { return c.method }

// Path returns the request path.
func (c *Ctx) Path() string { return c.path }

// Route returns the matched route pattern (e.g. "/users/:id").
// Returns the raw path if no pattern was matched (static routes).
func (c *Ctx) Route() string {
	if c.params.pattern != "" {
		return c.params.pattern
	}
	return c.path
}

// Param returns a path parameter value by name.
func (c *Ctx) Param(name string) string {
	return c.params.get(name)
}

// ParamInt returns a path parameter parsed as int.
func (c *Ctx) ParamInt(name string) (int, error) {
	return strconv.Atoi(c.params.get(name))
}

// Query returns a query parameter value by name, with optional default.
// An empty query value (?flag= or ?flag) returns the default.
func (c *Ctx) Query(name string, def ...string) string {
	if c.request != nil {
		if v := c.request.QueryParam(name); v != "" {
			return v
		}
	} else if v := c.tryQueryFastHTTP(name); v != "" {
		return v
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// QueryInt returns a query parameter parsed as int.
func (c *Ctx) QueryInt(name string, def ...int) int {
	s := c.Query(name)
	if s == "" {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	return v
}

// Header returns a request header value (lazy parsed, cached).
// Keys are normalized to canonical form so lookups are case-insensitive.
func (c *Ctx) Header(name string) string {
	key := http.CanonicalHeaderKey(name)
	if v, ok := c.headers[key]; ok {
		return v
	}
	if c.request != nil {
		v := c.request.Header(name)
		if v != "" {
			c.dirty |= dirtyHeaders
			c.headers[key] = v
		}
		return v
	}
	return ""
}

// Cookie returns the value of the named cookie via the transport interface.
func (c *Ctx) Cookie(name string) string {
	if c.request != nil {
		return c.request.Cookie(name)
	}
	return ""
}

// IP returns the client's IP address.
func (c *Ctx) IP() string {
	if c.request != nil {
		return c.request.RemoteAddr()
	}
	return ""
}

// BodyBytes returns the raw request body as a safe copy.
func (c *Ctx) BodyBytes() ([]byte, error) {
	if !c.bodyParsed {
		if c.request != nil {
			data, err := c.request.Body()
			c.bodyBytes = data
			c.bodyErr = err
		} else if data, ok := c.tryBodyBytesFastHTTP(); ok {
			// fasthttp path — PostBody() never returns an error
			c.bodyBytes = data
		}
		c.dirty |= dirtyBodyBytes
		c.bodyParsed = true
	}
	return c.bodyBytes, c.bodyErr
}

// BodyString returns the request body as a string.
// Discards body read errors for convenience — use BodyBytes() if you need error handling.
func (c *Ctx) BodyString() string {
	b, _ := c.BodyBytes()
	return string(b)
}

// Bind parses the request body into the given struct.
func (c *Ctx) Bind(v any) error {
	body, err := c.BodyBytes()
	if err != nil {
		if isBodyTooLarge(err) {
			return NewError(413, "request entity too large", err)
		}
		return BadRequest("failed to read request body")
	}
	if len(body) == 0 {
		return BadRequest("empty request body")
	}
	return c.app.config.JSONDecoder(body, v)
}

// Status sets the HTTP response status code. Chainable.
func (c *Ctx) Status(code int) *Ctx {
	c.status = code
	return c
}

// StatusCode returns the current response status code.
func (c *Ctx) StatusCode() int {
	return c.status
}

// Responded returns whether a response has already been written.
func (c *Ctx) Responded() bool {
	return c.responded
}

// JSON sends a JSON response using the configured JSON encoder.
// On the fasthttp path, uses SetBodyRaw for zero-copy response writing:
// sonic.Marshal allocates a []byte, SetBodyRaw references it (no memcpy),
// and fasthttp writes it to the TCP socket before GC reclaims it.
// This eliminates jsonBufPool contention under high concurrency.
// On net/http or non-default encoder paths, falls back to pooled buffer or direct marshal.
func (c *Ctx) JSON(v any) error {
	if c.responded {
		return ErrAlreadyResponded
	}

	// Ultra-fast path: fasthttp + default encoder → zero-copy via SetBodyRaw.
	if c.tryFastHTTPJSONDirect(v) {
		return nil
	}

	// Wing fast path: Marshal → SetJSON directly, skip pooled buffer.
	if jr, ok := c.writer.(transport.JSONResponder); ok {
		if len(c.cookies) == 0 && len(c.respHeaders) == 0 {
			data, err := c.app.config.JSONEncoder(v)
			if err != nil {
				return err
			}
			c.responded = true
			jr.SetJSON(c.status, data)
			return nil
		}
	}

	// Fast path: stream into pooled buffer (net/http transport)
	if enc := c.app.config.JSONStreamEncoder; enc != nil {
		return c.jsonPooled(v, enc)
	}

	// Fallback: custom JSONEncoder (no pooling)
	data, err := c.app.config.JSONEncoder(v)
	if err != nil {
		return err
	}
	return c.jsonSend(data)
}

// jsonPooled marshals v into a pooled bytes.Buffer and writes the response.
// The buffer is returned to the pool after the write completes.
// The fasthttp path is inlined here to avoid the jsonSend function call overhead.
func (c *Ctx) jsonPooled(v any, enc func(buf *bytes.Buffer, v any) error) error {
	buf := jsonBufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := enc(buf, v); err != nil {
		jsonBufPool.Put(buf)
		return err
	}

	data := buf.Bytes()

	// Fast path: tryFastHTTPJSON writes directly to fasthttp, bypassing transport interface.
	if c.tryFastHTTPJSON(data) {
		jsonBufPool.Put(buf)
		return nil
	}

	// Non-fasthttp path — delegate to shared jsonSend
	err := c.jsonSend(data)
	jsonBufPool.Put(buf)
	return err
}

// jsonSend writes a JSON response body. data must remain valid until this returns.
func (c *Ctx) jsonSend(data []byte) error {
	// Fast path for fasthttp — bypass transport interface and pool-copy overhead
	if c.tryFastHTTPJSON(data) {
		return nil
	}
	// Fast path for Wing transport — bypass header interface entirely
	if jr, ok := c.writer.(transport.JSONResponder); ok {
		c.responded = true
		jr.SetJSON(c.status, data)
		return nil
	}
	// Fast path for net/http embedded adapter — bypass transport interface
	if w := &c.embeddedResp; w.w != nil {
		c.responded = true
		h := w.w.Header()
		if w.contentTypeSlice == nil {
			w.contentTypeSlice = make([]string, 1)
		}
		w.contentTypeSlice[0] = "application/json; charset=utf-8"
		h["Content-Type"] = w.contentTypeSlice
		cl := len(data)
		if cl < len(contentLengthStrings) {
			h["Content-Length"] = []string{contentLengthStrings[cl]}
		} else {
			h["Content-Length"] = []string{strconv.Itoa(cl)}
		}
		w.w.WriteHeader(c.status)
		w.written = true
		_, _ = w.w.Write(data)
		return nil
	}
	c.contentType = "application/json; charset=utf-8"
	return c.sendBytes(data)
}

// Text sends a plain text response.
func (c *Ctx) Text(s string) error {
	if c.responded {
		return ErrAlreadyResponded
	}
	// Fast path for fasthttp - bypass transport interface
	if c.tryFastHTTPText(s) {
		return nil
	}
	// Fast path for transports with pre-built static response support (e.g., Wing)
	if sr, ok := c.writer.(transport.StaticTextResponder); ok {
		c.responded = true
		sr.SetStaticText(c.status, "text/plain; charset=utf-8", s)
		return nil
	}
	// Fast path for net/http embedded adapter - bypass transport interface
	if w := &c.embeddedResp; w.w != nil {
		c.responded = true
		h := w.w.Header()
		if w.contentTypeSlice == nil {
			w.contentTypeSlice = make([]string, 1)
		}
		w.contentTypeSlice[0] = "text/plain; charset=utf-8"
		h["Content-Type"] = w.contentTypeSlice
		w.w.WriteHeader(c.status)
		w.written = true
		_, _ = io.WriteString(w.w, s)
		return nil
	}
	c.contentType = "text/plain; charset=utf-8"
	return c.sendBytes([]byte(s))
}

// HTML sends an HTML response (raw string, no template).
// For template rendering, use c.Render() with a ViewEngine.
func (c *Ctx) HTML(html string) error {
	c.contentType = "text/html; charset=utf-8"
	return c.sendBytes([]byte(html))
}

// Render renders a named template with data using the configured ViewEngine.
//
//	app := kruda.New(kruda.WithViews(kruda.NewViewEngine("views/*.html")))
//	app.Get("/", func(c *kruda.Ctx) error {
//	    return c.Render("index", Map{"title": "Home"})
//	})
func (c *Ctx) Render(name string, data any, code ...int) error {
	if c.app.config.Views == nil {
		return NewError(500, "no view engine configured")
	}
	if len(code) > 0 {
		c.status = code[0]
	}
	buf := viewBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer viewBufPool.Put(buf)
	if err := c.app.config.Views.Render(buf, name, data); err != nil {
		return err
	}
	c.contentType = "text/html; charset=utf-8"
	return c.sendBytes(buf.Bytes())
}

// httpResponseWriter is an interface for transport.ResponseWriter implementations
// that wrap an underlying http.ResponseWriter (e.g. netHTTPResponseWriter).
type httpResponseWriter interface {
	Unwrap() http.ResponseWriter
}

// File sends a file as the response.
func (c *Ctx) File(path string) error {
	raw := c.request.RawRequest()
	if r, ok := raw.(*http.Request); ok {
		// Try to unwrap the underlying http.ResponseWriter from the transport adapter.
		if unwrapper, ok := c.writer.(httpResponseWriter); ok {
			c.responded = true
			http.ServeFile(unwrapper.Unwrap(), r, path)
			return nil
		}
	}
	return InternalError("file serving requires net/http transport, use NetHTTP() option")
}

// Stream sends a streaming response from an io.Reader.
func (c *Ctx) Stream(reader io.Reader) error {
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	c.responded = true
	_, err := io.Copy(writerAdapter{c.writer}, reader)
	return err
}

// NoContent sends a 204 No Content response.
func (c *Ctx) NoContent() error {
	c.status = 204
	return c.send()
}

// SetBody sets the response body from a []byte buffer without copying (lazy-send).
//
// The framework flushes the body during post-handler processing — no data is
// written to the transport until the handler returns. This means the caller
// MUST keep the buffer valid until the handler returns; do NOT return the
// buffer to a sync.Pool before the handler completes.
//
// If the handler also calls SendBytes, JSON, or Text, the eager-send wins
// (responded flag is set) and the SetBody data is silently discarded during
// the post-handler flush. Calling SetBody multiple times overwrites the
// previous value (last write wins).
//
// Returns *Ctx for method chaining:
//
//	c.SetContentType("application/json").SetBody(buf)
func (c *Ctx) SetBody(data []byte) *Ctx {
	c.dirty |= dirtyBody
	c.body = data
	return c
}

// SetContentType sets the Content-Type response header.
//
// Returns *Ctx for method chaining:
//
//	c.SetContentType("text/html; charset=utf-8").SetBody(html)
func (c *Ctx) SetContentType(ct string) *Ctx {
	c.contentType = ct
	return c
}

// SendBytesWithType is an optimized variant of SendBytes that sets Content-Type
// and writes the body in a single operation. On the fasthttp path this bypasses
// writeHeadersFastHTTP entirely — no branch checks for cacheControl, location,
// respHeaders, or cookies. Use this for handlers that only need Content-Type
// (e.g. json, plaintext, cached-queries).
//
// Precondition: caller must have already set the Date header via SetHeaderBytes.
func (c *Ctx) SendBytesWithType(contentType string, data []byte) error {
	if c.responded {
		return ErrAlreadyResponded
	}
	c.responded = true
	c.contentLength = len(data)

	// Ultra-fast path: fasthttp — bypass writeHeadersFastHTTP entirely
	if c.trySendBytesWithTypeFastHTTP(contentType, data) {
		return nil
	}

	// net/http and generic transport paths
	c.contentType = contentType
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	_, err := c.writer.Write(data)
	return err
}

// SendBytesWithTypeBytes is the zero-alloc variant of SendBytesWithType.
// Takes content-type as []byte to avoid string→[]byte conversion on the fasthttp hot path.
// Use pre-computed []byte content-type constants for maximum performance.
func (c *Ctx) SendBytesWithTypeBytes(contentType []byte, data []byte) error {
	if c.responded {
		return ErrAlreadyResponded
	}
	c.responded = true

	// Ultra-fast path: fasthttp — zero-alloc content-type write
	if c.trySendBytesWithTypeBytesFastHTTP(contentType, data) {
		return nil
	}

	// net/http and generic transport paths — convert back to string
	c.contentLength = len(data)
	c.contentType = string(contentType)
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	_, err := c.writer.Write(data)
	return err
}

// SendStaticWithTypeBytes is the zero-copy variant of SendBytesWithTypeBytes for
// static, pre-allocated response bodies. On the fasthttp path it uses SetBodyRaw
// instead of SetBody, avoiding a memcopy entirely.
//
// SAFETY: The data slice MUST be immutable for the lifetime of the program
// (e.g. a package-level var or const-like []byte). Do NOT use with pooled
// buffers or any data that may be modified after this call returns.
func (c *Ctx) SendStaticWithTypeBytes(contentType []byte, data []byte) error {
	if c.responded {
		return ErrAlreadyResponded
	}
	c.responded = true

	// Ultra-fast path: fasthttp — zero-copy body (no memcopy)
	if c.trySendStaticWithTypeBytesFastHTTP(contentType, data) {
		return nil
	}

	// net/http and generic transport paths — same as SendBytesWithTypeBytes
	c.contentLength = len(data)
	c.contentType = string(contentType)
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	_, err := c.writer.Write(data)
	return err
}

// SendBytes writes the response body and triggers an immediate flush to the
// transport (eager-send). Unlike SetBody, the data is written before SendBytes
// returns, so the caller may safely return the buffer to a sync.Pool
// immediately after the call.
//
// SendBytes is terminal: it returns an error and does not support method
// chaining. If the response has already been sent (via a prior SendBytes,
// JSON, Text, etc.), it returns ErrAlreadyResponded.
//
// On the fasthttp serve path the data is written via the embedded fasthttp
// adapter for zero-copy performance. On net/http and generic transport paths
// the data is copied by the underlying http.ResponseWriter.
func (c *Ctx) SendBytes(data []byte) error {
	if c.responded {
		return ErrAlreadyResponded
	}
	c.responded = true
	c.contentLength = len(data)

	// Fast path: fasthttp — use embedded adapter directly (c.writer is nil)
	if c.trySendBytesFastHTTP(data) {
		return nil
	}

	// net/http and generic transport paths — use existing writeHeaders + writer
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	_, err := c.writer.Write(data)
	return err
}

// Redirect sends a redirect response. Default status is 302.
func (c *Ctx) Redirect(url string, code ...int) error {
	c.body = nil // Clear any SetBody data — redirects MUST NOT have a body
	status := 302
	if len(code) > 0 {
		status = code[0]
	}
	c.status = status
	c.location = url // fixed slot — no map write
	return c.send()
}

// sanitizeHeaderValue strips CR and LF characters from a header value to prevent
// HTTP header injection (CRLF injection). Most values pass through unchanged via
// the fast path check.
func sanitizeHeaderValue(value string) string {
	if !strings.ContainsAny(value, "\r\n") {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\r' && value[i] != '\n' {
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

// isValidHeaderKey checks if key contains only valid token characters per RFC 7230.
// token = 1*tchar
// tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//
//	"^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
func isValidHeaderKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i := 0; i < len(key); i++ {
		if !isTokenChar(key[i]) {
			return false
		}
	}
	return true
}

// isTokenChar returns true if c is a valid HTTP token character per RFC 7230.
func isTokenChar(c byte) bool {
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

// SetHeader sets a response header, replacing any existing values. Chainable.
// Common headers (Content-Type, Cache-Control, Location) use fixed slots
// to avoid map allocation on the hot path.
// Validates key per RFC 7230 and strips CRLF from value to prevent header injection.
func (c *Ctx) SetHeader(key, value string) *Ctx {
	// Fast path: write directly to fasthttp response headers.
	// Bypasses map storage, validation, and sanitization for maximum throughput.
	if c.trySetHeaderFastHTTP(key, value) {
		return c
	}
	if !isValidHeaderKey(key) {
		c.app.config.Logger.Warn("kruda: invalid header key, skipping", "key", key)
		return c
	}
	value = sanitizeHeaderValue(value)
	switch key {
	case "Content-Type":
		c.contentType = value
	case "Cache-Control":
		c.cacheControl = value
	case "Location":
		c.location = value
	default:
		c.dirty |= dirtyRespHdrs
		c.respHeaders[internHeader(key)] = []string{value}
	}
	return c
}

// SetHeaderBytes sets a response header with a []byte value.
// On the fasthttp path this is zero-alloc (avoids []byte→string conversion).
// Falls back to SetHeader(key, string(value)) on net/http.
func (c *Ctx) SetHeaderBytes(key string, value []byte) *Ctx {
	if c.trySetHeaderBytesFastHTTP(key, value) {
		return c
	}
	return c.SetHeader(key, string(value))
}

// AddHeader appends a value to a response header without replacing existing values.
// Supports multi-value headers like Vary, Cache-Control. Chainable.
// Fixed-slot headers: Content-Type and Location always replace (single-valued per RFC).
// Cache-Control appends comma-separated (multi-valued per RFC 7234 section 5.2).
// Validates key per RFC 7230 and strips CRLF from value to prevent header injection.
func (c *Ctx) AddHeader(key, value string) *Ctx {
	if !isValidHeaderKey(key) {
		c.app.config.Logger.Warn("kruda: invalid header key, skipping", "key", key)
		return c
	}
	value = sanitizeHeaderValue(value)
	switch key {
	case "Content-Type":
		c.contentType = value
	case "Cache-Control":
		if c.cacheControl != "" {
			c.cacheControl += ", " + value
		} else {
			c.cacheControl = value
		}
	case "Location":
		c.location = value
	default:
		key = internHeader(key)
		c.dirty |= dirtyRespHdrs
		c.respHeaders[key] = append(c.respHeaders[key], value)
	}
	return c
}

// SetCookie sets a cookie on the response. Supports multiple cookies.
// Cookie values are sanitized to prevent header injection.
// Supports SameSite attribute and MaxAge<=0 for deletion.
func (c *Ctx) SetCookie(cookie *Cookie) *Ctx {
	cookie.Value = sanitizeHeaderValue(cookie.Value)
	cookie.Path = sanitizeHeaderValue(cookie.Path)
	cookie.Domain = sanitizeHeaderValue(cookie.Domain)
	c.dirty |= dirtyCookies
	c.cookies = append(c.cookies, cookie)
	return c
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

	return fn(stream)
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

// ErrAlreadyResponded is returned when a handler attempts to write a response
// after one has already been sent. Check with errors.Is(err, ErrAlreadyResponded).
var ErrAlreadyResponded = fmt.Errorf("kruda: response already sent")

func (c *Ctx) send() error {
	if c.responded {
		return ErrAlreadyResponded
	}
	c.responded = true

	// Per HTTP spec, 204 No Content MUST NOT have a body.
	if c.status == 204 {
		c.body = nil
	}
	if c.body != nil {
		c.contentLength = len(c.body)
	}

	// Fast path: fasthttp — use embedded adapter directly (c.writer is nil)
	if c.trySendFastHTTP() {
		return nil
	}

	// net/http and generic transport paths
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	if c.body != nil {
		_, err := c.writer.Write(c.body)
		c.body = nil
		return err
	}
	return nil
}

// responseBufPoolThreshold is the max response size that uses a pooled buffer.
const responseBufPoolThreshold = 4096

func (c *Ctx) sendBytes(data []byte) error {
	if c.responded {
		return ErrAlreadyResponded
	}
	c.responded = true
	c.contentLength = len(data)
	c.writeHeaders()
	c.writer.WriteHeader(c.status)

	// Small responses: copy into a pooled buffer then write once.
	// This avoids an allocation for the write path on the hot path.
	if len(data) <= responseBufPoolThreshold {
		bufp := responseBufPool.Get().(*[]byte)
		buf := append((*bufp)[:0], data...)
		_, err := c.writer.Write(buf)
		*bufp = buf
		responseBufPool.Put(bufp)
		return err
	}

	// Large responses: direct write, no pool.
	_, err := c.writer.Write(data)
	return err
}

// contentLengthStrings is a pre-computed lookup table for common Content-Length
// values as strings, avoiding strconv.Itoa allocation on the net/http path.
// Covers 0–2048 which handles the majority of JSON API responses.
var contentLengthStrings [2049]string

func init() {
	for i := range contentLengthStrings {
		contentLengthStrings[i] = strconv.Itoa(i)
	}
}

func contentLengthString(length int) string {
	if length >= 0 && length < len(contentLengthStrings) {
		return contentLengthStrings[length]
	}
	return strconv.Itoa(length)
}

func (c *Ctx) writeHeaders() {
	// Fast path: net/http embedded adapter — boolean check instead of type assert
	if c.embeddedResp.w != nil {
		// Simple case: only content-type + content-length, no other headers
		if c.cacheControl == "" && c.location == "" &&
			len(c.respHeaders) == 0 && len(c.cookies) == 0 &&
			len(c.app.secHeaders) == 0 {
			if c.contentType != "" {
				c.embeddedResp.SetContentType(c.contentType)
			}
			if c.contentLength >= 0 {
				c.embeddedResp.SetContentLength(contentLengthString(c.contentLength))
			}
			return
		}
		// Complex case: use direct header access
		c.writeHeadersNetHTTP()
		return
	}

	// Fallback: custom transport — use interface methods
	c.writeHeadersGeneric()
}

// writeHeadersNetHTTP writes all response headers using the embedded net/http
// adapter's direct header access. Called when the response has custom headers,
// cookies, or security headers beyond simple content-type/content-length.
func (c *Ctx) writeHeadersNetHTTP() {
	// Use embedded adapter's optimized methods for common headers
	if c.contentType != "" {
		c.embeddedResp.SetContentType(c.contentType)
	}
	if c.contentLength >= 0 {
		c.embeddedResp.SetContentLength(contentLengthString(c.contentLength))
	}

	// Direct header access for other headers
	h := c.embeddedResp.DirectHeader()

	if c.cacheControl != "" {
		h.Set("Cache-Control", c.cacheControl)
	}
	if c.location != "" {
		h.Set("Location", c.location)
	}

	if len(c.respHeaders) > 0 {
		for k, vals := range c.respHeaders {
			h[k] = vals
		}
	}

	if len(c.cookies) > 0 {
		for _, cookie := range c.cookies {
			h.Add("Set-Cookie", formatCookie(cookie))
		}
	}

	if len(c.app.secHeaders) > 0 {
		for _, kv := range c.app.secHeaders {
			if h[kv[0]] == nil {
				h[kv[0]] = []string{kv[1]}
			}
		}
	}
}

// writeHeadersGeneric writes all response headers using the transport interface
// methods. Used as fallback for custom transports that are not the embedded
// net/http adapter.
func (c *Ctx) writeHeadersGeneric() {
	h := c.writer.Header()

	if c.contentType != "" {
		h.Set("Content-Type", c.contentType)
	}
	if c.contentLength >= 0 {
		h.Set("Content-Length", contentLengthString(c.contentLength))
	}
	if c.cacheControl != "" {
		h.Set("Cache-Control", c.cacheControl)
	}
	if c.location != "" {
		h.Set("Location", c.location)
	}

	if len(c.respHeaders) > 0 {
		for k, vals := range c.respHeaders {
			if len(vals) == 1 {
				h.Set(k, vals[0])
			} else {
				for i, v := range vals {
					if i == 0 {
						h.Set(k, v)
					} else {
						h.Add(k, v)
					}
				}
			}
		}
	}

	if len(c.cookies) > 0 {
		for _, cookie := range c.cookies {
			h.Add("Set-Cookie", formatCookie(cookie))
		}
	}

	if len(c.app.secHeaders) > 0 {
		if directAccess, ok := h.(transport.DirectHeaderAccess); ok {
			if httpHeader := directAccess.DirectHeader(); httpHeader != nil {
				for _, kv := range c.app.secHeaders {
					if httpHeader[kv[0]] == nil {
						httpHeader[kv[0]] = []string{kv[1]}
					}
				}
			} else {
				for _, kv := range c.app.secHeaders {
					if h.Get(kv[0]) == "" {
						h.Set(kv[0], kv[1])
					}
				}
			}
		} else {
			for _, kv := range c.app.secHeaders {
				if h.Get(kv[0]) == "" {
					h.Set(kv[0], kv[1])
				}
			}
		}
	}
}

// formatCookie serializes a Cookie to a Set-Cookie header value.
// Sanitizes name and value to prevent header injection. Supports SameSite and MaxAge<0 for deletion.
func formatCookie(cookie *Cookie) string {
	name := sanitizeCookieToken(cookie.Name)
	value := sanitizeCookieValue(cookie.Value)

	var b strings.Builder
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(value)

	if cookie.Path != "" {
		b.WriteString("; Path=")
		b.WriteString(sanitizeCookieValue(cookie.Path))
	}
	if cookie.Domain != "" {
		b.WriteString("; Domain=")
		b.WriteString(sanitizeCookieValue(cookie.Domain))
	}
	// MaxAge < 0 means delete cookie (set Max-Age=0)
	if cookie.MaxAge > 0 {
		b.WriteString("; Max-Age=")
		b.WriteString(strconv.Itoa(cookie.MaxAge))
	} else if cookie.MaxAge < 0 {
		b.WriteString("; Max-Age=0")
	}
	if cookie.Secure {
		b.WriteString("; Secure")
	}
	if cookie.HTTPOnly {
		b.WriteString("; HttpOnly")
	}
	switch cookie.SameSite {
	case http.SameSiteLaxMode:
		b.WriteString("; SameSite=Lax")
	case http.SameSiteStrictMode:
		b.WriteString("; SameSite=Strict")
	case http.SameSiteNoneMode:
		b.WriteString("; SameSite=None")
	}
	return b.String()
}

// sanitizeCookieToken removes characters not allowed in cookie names (RFC 6265 token).
func sanitizeCookieToken(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c > 0x20 && c < 0x7f && !isCookieSeparator(c) {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// sanitizeCookieValue removes characters not allowed in cookie values.
// Strips semicolons, newlines, and other control characters to prevent header injection.
func sanitizeCookieValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x21 || (c >= 0x23 && c <= 0x2b) || (c >= 0x2d && c <= 0x3a) || (c >= 0x3c && c <= 0x5b) || (c >= 0x5d && c <= 0x7e) {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isCookieSeparator(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}', ' ', '\t':
		return true
	}
	return false
}

// writerAdapter adapts transport.ResponseWriter to io.Writer for io.Copy.
type writerAdapter struct {
	w transport.ResponseWriter
}

func (a writerAdapter) Write(p []byte) (int, error) {
	return a.w.Write(p)
}

// isBodyTooLarge checks if an error indicates the request body exceeded the size limit.
// Works with transport.ErrBodyTooLarge from both net/http and fasthttp transports.
func isBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}
	var btle *transport.BodyTooLargeError
	return errors.As(err, &btle)
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
