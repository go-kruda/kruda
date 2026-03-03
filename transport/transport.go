package transport

import (
	"context"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Transport defines the network layer interface.
// Implementations can be net/http, fasthttp, or custom transports.
type Transport interface {
	ListenAndServe(addr string, handler Handler) error
	Serve(ln net.Listener, handler Handler) error
	Shutdown(ctx context.Context) error
}

// Handler is the core request handler interface that transports call.
type Handler interface {
	ServeKruda(w ResponseWriter, r Request)
}

// FastHTTPHandler is an optional interface that Handler implementations can
// satisfy to enable zero-allocation fasthttp serving. FastHTTPTransport
// checks for this at startup and calls ServeFast directly, bypassing the
// generic ServeKruda path (which allocates adapter objects per request).
type FastHTTPHandler interface {
	ServeFastHTTP(ctx interface{}) // accepts *fasthttp.RequestCtx as interface{} to avoid import cycle
}

// HandlerFunc is an adapter to allow use of ordinary functions as Handler.
type HandlerFunc func(w ResponseWriter, r Request)

// ServeKruda calls f(w, r).
func (f HandlerFunc) ServeKruda(w ResponseWriter, r Request) {
	f(w, r)
}

// Request abstracts an incoming HTTP request.
type Request interface {
	Method() string
	Path() string
	Header(key string) string
	Body() ([]byte, error)
	QueryParam(key string) string
	RemoteAddr() string
	Cookie(name string) string
	RawRequest() any // underlying *http.Request or *fasthttp.RequestCtx
	Context() context.Context
	MultipartForm(maxBytes int64) (*multipart.Form, error)
}

// ResponseWriter abstracts the HTTP response writer.
type ResponseWriter interface {
	WriteHeader(statusCode int)
	Header() HeaderMap
	Write(data []byte) (int, error)
}

// StaticResponder is an optional interface for ResponseWriters that support
// pre-built static responses (e.g., Wing transport). When SetStaticResponse
// is called, the transport skips normal response serialization and writes
// the pre-built bytes directly.
type StaticResponder interface {
	SetStaticResponse(data []byte)
}

// StaticTextResponder extends StaticResponder with a string-based fast path
// that avoids the []byte(s) allocation.
type StaticTextResponder interface {
	SetStaticText(status int, contentType, text string)
}

// FileSender is an optional interface for ResponseWriters that support
// sendfile(2) zero-copy file transfer (e.g., Wing transport).
type FileSender interface {
	SetSendFile(fd int32, size int64)
}

// JSONResponder is an optional interface for ResponseWriters that support
// a zero-copy JSON fast path — bypasses header interface overhead.
// Implement SetJSON(status int, data []byte) to write status + Content-Type:json + body in one shot.
type JSONResponder interface {
	SetJSON(status int, data []byte)
}

// FeatherConfigurator is an optional interface for transports that support
// per-route tuning (e.g., Wing transport). SetRouteFeather is called during
// route registration to configure dispatch/buffer/response modes per route.
type FeatherConfigurator interface {
	SetRouteFeather(method, path string, feather any)
}

// HeaderMap abstracts response header manipulation.
type HeaderMap interface {
	Set(key, value string)
	Add(key, value string) // appends a value (for multi-value headers like Set-Cookie)
	Get(key string) string
	Del(key string)
}

// DirectHeaderAccess is an optional interface that HeaderMap implementations
// can implement to provide direct access to the underlying http.Header for optimization.
type DirectHeaderAccess interface {
	DirectHeader() http.Header
}

// AllHeadersProvider is implemented by transports that can enumerate all headers.
type AllHeadersProvider interface {
	AllHeaders() map[string]string
}

// AllQueryProvider is implemented by transports that can enumerate all query params.
type AllQueryProvider interface {
	AllQuery() map[string]string
}

// --- Static response cache ---

var staticCache sync.Map // key → *staticEntry

type staticEntry struct {
	resp       []byte
	dateOffset int
}

// Pre-computed status lines for static responses.
var staticStatusLines [600][]byte

func init() {
	codes := [][2]any{
		{200, "OK"}, {204, "No Content"}, {301, "Moved Permanently"},
		{304, "Not Modified"}, {400, "Bad Request"}, {404, "Not Found"},
		{500, "Internal Server Error"},
	}
	for _, p := range codes {
		c := p[0].(int)
		staticStatusLines[c] = []byte("HTTP/1.1 " + strconv.Itoa(c) + " " + p[1].(string) + "\r\n")
	}
	// Start date updater.
	go func() {
		for range time.Tick(time.Second) {
			dateBytes := []byte(time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
			staticCache.Range(func(_, v any) bool {
				e := v.(*staticEntry)
				copy(e.resp[e.dateOffset:], dateBytes)
				return true
			})
		}
	}()
}

// GetStaticResponse returns a cached pre-built HTTP response.
// The Date header is auto-patched every second.
func GetStaticResponse(status int, contentType string, body []byte) []byte {
	key := contentType + ":" + string(body)
	return getStaticByKey(status, contentType, key, body)
}

// GetStaticResponseString is like GetStaticResponse but avoids []byte(s) allocation.
func GetStaticResponseString(status int, contentType, body string) []byte {
	key := contentType + ":" + body
	return getStaticByKey(status, contentType, key, []byte(body))
}

func getStaticByKey(status int, contentType, key string, body []byte) []byte {
	if v, ok := staticCache.Load(key); ok {
		return v.(*staticEntry).resp
	}
	var b []byte
	if status > 0 && status < len(staticStatusLines) && staticStatusLines[status] != nil {
		b = append(b, staticStatusLines[status]...)
	} else {
		b = append(b, "HTTP/1.1 "...)
		b = strconv.AppendInt(b, int64(status), 10)
		b = append(b, " OK\r\n"...)
	}
	b = append(b, "Date: "...)
	dateOffset := len(b)
	b = append(b, time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")...)
	b = append(b, "\r\nServer: Kruda\r\n"...)
	if contentType != "" {
		b = append(b, "Content-Type: "...)
		b = append(b, contentType...)
		b = append(b, "\r\n"...)
	}
	b = append(b, "Content-Length: "...)
	b = strconv.AppendInt(b, int64(len(body)), 10)
	b = append(b, "\r\n\r\n"...)
	b = append(b, body...)
	entry := &staticEntry{resp: b, dateOffset: dateOffset}
	actual, _ := staticCache.LoadOrStore(key, entry)
	return actual.(*staticEntry).resp
}
