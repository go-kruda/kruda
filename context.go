package kruda

import (
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
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
	Expires  time.Time // optional; if set, emits Expires header for legacy client compat
	MaxAge   int
	Secure   bool
	HTTPOnly bool
	SameSite http.SameSite
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

// Ctx is the request context, pooled via sync.Pool for zero allocation.
// All string values are proper copies — safe to use anywhere.
//
// Field ordering is optimized for CPU cache line alignment.
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

// Map is a convenience type alias for map[string]any.
// Use it for quick JSON responses without defining a struct.
//
//	c.JSON(kruda.Map{"message": "hello", "ok": true})
type Map = map[string]any
