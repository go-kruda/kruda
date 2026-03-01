package transport

import (
	"context"
	"mime/multipart"
	"net"
	"net/http"
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
