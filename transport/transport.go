package transport

import (
	"context"
	"mime/multipart"
	"net/http"
)

// Transport defines the network layer interface.
// Implementations can be net/http, Netpoll, or custom transports.
type Transport interface {
	// ListenAndServe starts the server on the given address.
	ListenAndServe(addr string, handler Handler) error

	// Shutdown gracefully shuts down the server.
	Shutdown(ctx context.Context) error
}

// Handler is the core request handler interface that transports call.
type Handler interface {
	ServeKruda(w ResponseWriter, r Request)
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
	RawRequest() any // access underlying *http.Request if needed
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

// MultipartProvider is implemented by transports that support multipart forms.
type MultipartProvider interface {
	MultipartForm(maxBytes int64) (*multipart.Form, error)
}

// ContextProvider is implemented by transports that carry a request context.
type ContextProvider interface {
	Context() context.Context
}

// AllHeadersProvider is implemented by transports that can enumerate all headers.
type AllHeadersProvider interface {
	AllHeaders() map[string]string
}

// AllQueryProvider is implemented by transports that can enumerate all query params.
type AllQueryProvider interface {
	AllQuery() map[string]string
}
