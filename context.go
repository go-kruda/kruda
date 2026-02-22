package kruda

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kruda/kruda/transport"
)

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
type Ctx struct {
	app *App

	// Request data (populated on init, safe copies)
	method     string
	path       string
	params     map[string]string // pre-allocated, reset per request
	query      map[string]string // lazy parsed
	headers    map[string]string // lazy parsed, cached
	bodyBytes  []byte
	bodyParsed bool
	bodyErr    error // H3: preserve body read errors

	// Response
	status      int
	respHeaders map[string][]string // N2: multi-value header support
	cookies     []*Cookie           // C4: separate cookie slice for multi-cookie support
	responded   bool

	// Internal
	routeIndex int           // current position in handler chain
	handlers   []HandlerFunc // middleware + handler chain
	locals     map[string]any

	// Writer (transport-specific)
	writer transport.ResponseWriter

	// Raw request (transport-specific)
	request transport.Request

	// Timing
	startTime time.Time

	// Context for stdlib compatibility
	ctx context.Context

	// Logger (lazy-init, cached per request)
	logger *slog.Logger
}

// newCtx creates a new context with pre-allocated maps.
func newCtx(app *App) *Ctx {
	return &Ctx{
		app:         app,
		params:      make(map[string]string, 4),
		query:       make(map[string]string, 8),
		headers:     make(map[string]string, 8),
		respHeaders: make(map[string][]string, 8),
		locals:      make(map[string]any, 4),
		cookies:     make([]*Cookie, 0, 4),
		status:      200,
	}
}

// reset prepares the context for reuse from the pool.
func (c *Ctx) reset(w transport.ResponseWriter, r transport.Request) {
	c.method = r.Method()
	c.path = r.Path()
	c.status = 200
	c.responded = false
	c.bodyParsed = false
	c.bodyBytes = nil
	c.bodyErr = nil
	c.routeIndex = 0
	c.handlers = nil
	c.startTime = time.Now()
	c.writer = w
	c.request = r
	c.ctx = nil
	c.logger = nil
	c.cookies = c.cookies[:0]

	// Reset maps without reallocating
	clear(c.params)
	clear(c.query)
	clear(c.headers)
	clear(c.respHeaders)
	clear(c.locals)
}

// cleanup prepares the context for returning to the pool.
func (c *Ctx) cleanup() {
	c.writer = nil
	c.request = nil
	c.bodyBytes = nil
	c.handlers = nil
	c.ctx = nil
	c.logger = nil
}

// --- Request methods (all return safe, owned strings) ---

// Method returns the HTTP method (GET, POST, etc.).
func (c *Ctx) Method() string { return c.method }

// Path returns the request path.
func (c *Ctx) Path() string { return c.path }

// Param returns a path parameter value by name.
func (c *Ctx) Param(name string) string {
	return c.params[name]
}

// ParamInt returns a path parameter parsed as int.
func (c *Ctx) ParamInt(name string) (int, error) {
	return strconv.Atoi(c.params[name])
}

// Query returns a query parameter value by name, with optional default.
// F4 note: an empty query value (?flag= or ?flag) returns the default.
// This is intentional for Phase 1 — Phase 2 binding will distinguish
// between "present but empty" and "absent" via struct tags.
func (c *Ctx) Query(name string, def ...string) string {
	if v, ok := c.query[name]; ok && v != "" {
		return v
	}
	// Try from raw request (lazy parse + cache)
	if c.request != nil {
		if v := c.request.QueryParam(name); v != "" {
			c.query[name] = v
			return v
		}
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
func (c *Ctx) Header(name string) string {
	if v, ok := c.headers[name]; ok {
		return v
	}
	if c.request != nil {
		v := c.request.Header(name)
		if v != "" {
			c.headers[name] = v
		}
		return v
	}
	return ""
}

// Cookie returns the value of the named cookie via the transport interface.
// H4 fix: no longer casts to *http.Request directly.
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
// H3 fix: preserves and returns body read errors.
func (c *Ctx) BodyBytes() ([]byte, error) {
	if !c.bodyParsed && c.request != nil {
		data, err := c.request.Body()
		c.bodyBytes = data
		c.bodyErr = err
		c.bodyParsed = true
	}
	return c.bodyBytes, c.bodyErr
}

// BodyString returns the request body as a string.
// N6: intentionally discards body read errors for convenience.
// Use BodyBytes() directly if you need error handling.
func (c *Ctx) BodyString() string {
	b, _ := c.BodyBytes()
	return string(b)
}

// Bind parses the request body into the given struct.
func (c *Ctx) Bind(v any) error {
	body, err := c.BodyBytes()
	if err != nil {
		return BadRequest("failed to read request body")
	}
	if len(body) == 0 {
		return BadRequest("empty request body")
	}
	return c.app.config.JSONDecoder(body, v)
}

// --- Response methods ---

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
func (c *Ctx) JSON(v any) error {
	data, err := c.app.config.JSONEncoder(v)
	if err != nil {
		return err
	}
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	return c.sendBytes(data)
}

// Text sends a plain text response.
func (c *Ctx) Text(s string) error {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	return c.sendBytes([]byte(s))
}

// HTML sends an HTML response (raw string, no template).
// For template rendering, use a template engine middleware.
func (c *Ctx) HTML(html string) error {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	return c.sendBytes([]byte(html))
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
	return InternalError("file serving requires net/http transport")
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

// Redirect sends a redirect response. Default status is 302.
func (c *Ctx) Redirect(url string, code ...int) error {
	status := 302
	if len(code) > 0 {
		status = code[0]
	}
	c.status = status
	c.SetHeader("Location", url)
	return c.send()
}

// SetHeader sets a response header, replacing any existing values. Chainable.
func (c *Ctx) SetHeader(key, value string) *Ctx {
	c.respHeaders[key] = []string{value}
	return c
}

// AddHeader appends a value to a response header without replacing existing values.
// N2 fix: supports multi-value headers like Vary, Cache-Control. Chainable.
func (c *Ctx) AddHeader(key, value string) *Ctx {
	c.respHeaders[key] = append(c.respHeaders[key], value)
	return c
}

// SetCookie sets a cookie on the response. Supports multiple cookies (C4 fix).
// Cookie values are sanitized to prevent header injection (H7 fix).
// Supports SameSite attribute (M14 fix) and MaxAge<=0 for deletion (M15 fix).
func (c *Ctx) SetCookie(cookie *Cookie) *Ctx {
	c.cookies = append(c.cookies, cookie)
	return c
}

// --- Locals (request-scoped key-value store) ---

// Set stores a value in the request-scoped locals.
func (c *Ctx) Set(key string, value any) {
	c.locals[key] = value
}

// Get retrieves a value from the request-scoped locals.
func (c *Ctx) Get(key string) any {
	return c.locals[key]
}

// --- Flow control ---

// Next calls the next handler in the middleware chain.
func (c *Ctx) Next() error {
	c.routeIndex++
	if c.routeIndex < len(c.handlers) {
		return c.handlers[c.routeIndex](c)
	}
	return nil
}

// Latency returns the time elapsed since the request started.
func (c *Ctx) Latency() time.Duration {
	return time.Since(c.startTime)
}

// --- Context for stdlib compatibility ---

// Context returns the context.Context for this request.
func (c *Ctx) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

// SetContext sets the context.Context for this request.
func (c *Ctx) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// --- Request-scoped logger ---

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

// --- Internal send helpers ---

func (c *Ctx) send() error {
	if c.responded {
		return nil
	}
	c.responded = true
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	return nil
}

func (c *Ctx) sendBytes(data []byte) error {
	if c.responded {
		return nil
	}
	c.responded = true
	// M12: set Content-Length
	c.respHeaders["Content-Length"] = []string{strconv.Itoa(len(data))}
	c.writeHeaders()
	c.writer.WriteHeader(c.status)
	_, err := c.writer.Write(data)
	return err
}

func (c *Ctx) writeHeaders() {
	h := c.writer.Header()
	// N2: write multi-value headers correctly
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

	// C4: write cookies using Add to support multiple Set-Cookie headers
	for _, cookie := range c.cookies {
		h.Add("Set-Cookie", formatCookie(cookie))
	}

	// Security headers
	sec := c.app.config.Security
	if sec.XSSProtection != "" {
		h.Set("X-XSS-Protection", sec.XSSProtection)
	}
	if sec.ContentTypeNosniff != "" {
		h.Set("X-Content-Type-Options", sec.ContentTypeNosniff)
	}
	if sec.XFrameOptions != "" {
		h.Set("X-Frame-Options", sec.XFrameOptions)
	}
	if sec.ReferrerPolicy != "" {
		h.Set("Referrer-Policy", sec.ReferrerPolicy)
	}
	if sec.ContentSecurityPolicy != "" {
		h.Set("Content-Security-Policy", sec.ContentSecurityPolicy)
	}
}

// formatCookie serializes a Cookie to a Set-Cookie header value.
// H7: sanitizes name and value to prevent header injection.
// M14: includes SameSite attribute.
// M15: supports MaxAge < 0 for cookie deletion.
func formatCookie(cookie *Cookie) string {
	// H7: sanitize cookie name and value
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
	// M15: MaxAge < 0 means delete cookie (set Max-Age=0)
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
	// M14: SameSite attribute
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
