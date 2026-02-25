package transport

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// NetHTTPTransport implements Transport using Go's standard net/http.
type NetHTTPTransport struct {
	mu     sync.Mutex
	server *http.Server
	config NetHTTPConfig
}

// NetHTTPConfig holds configuration for the net/http transport.
type NetHTTPConfig struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxBodySize    int
	MaxHeaderBytes int    // maps to http.Server.MaxHeaderBytes
	TrustProxy     bool   // C6: only trust X-Forwarded-For/X-Real-IP when true
	TLSCertFile    string // path to TLS certificate file
	TLSKeyFile     string // path to TLS private key file
}

// NewNetHTTP creates a new net/http transport.
func NewNetHTTP(cfg NetHTTPConfig) *NetHTTPTransport {
	return &NetHTTPTransport{config: cfg}
}

// ListenAndServe starts the HTTP server.
func (t *NetHTTPTransport) ListenAndServe(addr string, handler Handler) error {
	t.mu.Lock()
	t.server = &http.Server{
		Addr:    addr,
		Handler: &netHTTPAdapter{handler: handler, maxBody: t.config.MaxBodySize, trustProxy: t.config.TrustProxy},
	}

	if t.config.ReadTimeout > 0 {
		t.server.ReadTimeout = t.config.ReadTimeout
	}
	if t.config.WriteTimeout > 0 {
		t.server.WriteTimeout = t.config.WriteTimeout
	}
	if t.config.IdleTimeout > 0 {
		t.server.IdleTimeout = t.config.IdleTimeout
	}
	if t.config.MaxHeaderBytes > 0 {
		t.server.MaxHeaderBytes = t.config.MaxHeaderBytes
	}
	srv := t.server
	t.mu.Unlock()

	// TLS: use ListenAndServeTLS for HTTPS + HTTP/2 auto-negotiation
	if t.config.TLSCertFile != "" && t.config.TLSKeyFile != "" {
		return srv.ListenAndServeTLS(t.config.TLSCertFile, t.config.TLSKeyFile)
	}
	return srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (t *NetHTTPTransport) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	srv := t.server
	t.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// netHTTPAdapter adapts net/http to Kruda's transport interface.
type netHTTPAdapter struct {
	handler    Handler
	maxBody    int
	trustProxy bool
}

func (a *netHTTPAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := &netHTTPRequest{r: r, maxBody: a.maxBody, trustProxy: a.trustProxy}
	resp := &netHTTPResponseWriter{w: w, statusCode: 200}
	a.handler.ServeKruda(resp, req)
}

// --- Request implementation ---

type netHTTPRequest struct {
	r          *http.Request
	maxBody    int
	trustProxy bool
	body       []byte
	bodyErr    error
	bodyRead   bool
	queryVals  url.Values // cached parsed query string
	queryDone  bool       // whether queryVals has been parsed
}

func (r *netHTTPRequest) Method() string { return r.r.Method }

func (r *netHTTPRequest) Path() string {
	path := r.r.URL.Path
	if path == "" {
		return "/"
	}
	return path
}

func (r *netHTTPRequest) Header(key string) string {
	return r.r.Header.Get(key)
}

func (r *netHTTPRequest) Body() ([]byte, error) {
	if r.bodyRead {
		return r.body, r.bodyErr
	}
	r.bodyRead = true

	if r.r.Body == nil {
		return nil, nil
	}
	defer r.r.Body.Close()

	// Limit body size
	reader := io.Reader(r.r.Body)
	if r.maxBody > 0 {
		reader = io.LimitReader(r.r.Body, int64(r.maxBody)+1)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		r.bodyErr = err
		return nil, err
	}

	if r.maxBody > 0 && len(data) > r.maxBody {
		r.bodyErr = ErrBodyTooLarge
		return nil, ErrBodyTooLarge
	}

	r.body = data
	return data, nil
}

// QueryParam returns a query parameter value.
// Parses the query string once and caches the result for subsequent calls.
func (r *netHTTPRequest) QueryParam(key string) string {
	if !r.queryDone {
		r.queryVals = r.r.URL.Query()
		r.queryDone = true
	}
	return r.queryVals.Get(key)
}

// RemoteAddr returns the client IP. Only trusts proxy headers (X-Forwarded-For,
// X-Real-IP) when TrustProxy is enabled. (C6 fix)
// N8 fix: strips port from r.r.RemoteAddr for consistent bare-IP format.
func (r *netHTTPRequest) RemoteAddr() string {
	if r.trustProxy {
		if ip := r.r.Header.Get("X-Forwarded-For"); ip != "" {
			// Take the first IP in the chain
			for i := 0; i < len(ip); i++ {
				if ip[i] == ',' {
					return trimSpace(ip[:i])
				}
			}
			// NEW-2 fix: trim single-value case too (may have leading/trailing spaces)
			return trimSpace(ip)
		}
		if ip := r.r.Header.Get("X-Real-Ip"); ip != "" {
			return trimSpace(ip)
		}
	}
	return stripPort(r.r.RemoteAddr)
}

// Cookie returns the value of the named cookie via the transport interface. (H4 fix)
func (r *netHTTPRequest) Cookie(name string) string {
	cookie, err := r.r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (r *netHTTPRequest) RawRequest() any { return r.r }

func (r *netHTTPRequest) MultipartForm(maxBytes int64) (*multipart.Form, error) {
	if err := r.r.ParseMultipartForm(maxBytes); err != nil {
		return nil, err
	}
	return r.r.MultipartForm, nil
}

func (r *netHTTPRequest) Context() context.Context {
	return r.r.Context()
}

func (r *netHTTPRequest) AllHeaders() map[string]string {
	m := make(map[string]string, len(r.r.Header))
	for k, v := range r.r.Header {
		m[k] = strings.Join(v, ", ")
	}
	return m
}

func (r *netHTTPRequest) AllQuery() map[string]string {
	q := r.r.URL.Query()
	m := make(map[string]string, len(q))
	for k, v := range q {
		m[k] = strings.Join(v, ", ")
	}
	return m
}

// trimSpace trims leading and trailing spaces from a string without importing strings.
func trimSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

// stripPort removes the port suffix from a host:port address.
// Handles IPv6 bracket notation (e.g. "[::1]:8080" → "::1").
// Returns the input unchanged if no port is found.
func stripPort(addr string) string {
	if len(addr) == 0 {
		return addr
	}
	// IPv6 with brackets: [::1]:port
	if addr[0] == '[' {
		for i := 1; i < len(addr); i++ {
			if addr[i] == ']' {
				return addr[1:i]
			}
		}
		return addr
	}
	// IPv4 or hostname: host:port — find last colon
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// --- ResponseWriter implementation ---

type netHTTPResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
	written    bool
}

func (w *netHTTPResponseWriter) WriteHeader(statusCode int) {
	if w.written {
		return
	}
	w.statusCode = statusCode
	w.w.WriteHeader(statusCode)
	w.written = true
}

func (w *netHTTPResponseWriter) Header() HeaderMap {
	return &netHTTPHeaderMap{h: w.w.Header()}
}

func (w *netHTTPResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.w.WriteHeader(w.statusCode)
		w.written = true
	}
	return w.w.Write(data)
}

// Unwrap returns the underlying http.ResponseWriter for use with http.ServeFile etc.
func (w *netHTTPResponseWriter) Unwrap() http.ResponseWriter {
	return w.w
}

// Flush delegates to the underlying http.ResponseWriter if it supports flushing.
// Required for SSE (Server-Sent Events) streaming.
func (w *netHTTPResponseWriter) Flush() {
	if f, ok := w.w.(http.Flusher); ok {
		f.Flush()
	}
}

// --- HeaderMap implementation ---

type netHTTPHeaderMap struct {
	h http.Header
}

func (m *netHTTPHeaderMap) Set(key, value string) { m.h.Set(key, value) }
func (m *netHTTPHeaderMap) Get(key string) string { return m.h.Get(key) }
func (m *netHTTPHeaderMap) Del(key string)        { m.h.Del(key) }
func (m *netHTTPHeaderMap) Add(key, value string) { m.h.Add(key, value) }

// DirectHeader implements DirectHeaderAccess interface for optimization
func (m *netHTTPHeaderMap) DirectHeader() http.Header { return m.h }

// --- Errors ---

var ErrBodyTooLarge = &BodyTooLargeError{}

type BodyTooLargeError struct{}

func (e *BodyTooLargeError) Error() string { return "request body too large" }

// NewNetHTTPRequest wraps an *http.Request into a transport.Request.
func NewNetHTTPRequest(r *http.Request) Request {
	return &netHTTPRequest{r: r}
}

// NewNetHTTPRequestWithLimit wraps an *http.Request into a transport.Request
// with a maximum body size limit. When the body exceeds maxBody bytes,
// Body() returns ErrBodyTooLarge.
func NewNetHTTPRequestWithLimit(r *http.Request, maxBody int) Request {
	return &netHTTPRequest{r: r, maxBody: maxBody}
}

// NewNetHTTPResponseWriter wraps an http.ResponseWriter into a transport.ResponseWriter.
func NewNetHTTPResponseWriter(w http.ResponseWriter) ResponseWriter {
	return &netHTTPResponseWriter{w: w, statusCode: 200}
}
