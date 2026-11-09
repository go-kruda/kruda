//go:build !windows

package transport

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

// FastHTTPTransport implements Transport using valyala/fasthttp.
type FastHTTPTransport struct {
	server *fasthttp.Server
	config FastHTTPConfig
}

// FastHTTPConfig holds configuration for the fasthttp transport.
type FastHTTPConfig struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxBodySize    int
	Concurrency    int
	TrustedProxies []string // exact proxy IP allowlist; empty trusts no peer
	TrustProxy     bool
}

// NewFastHTTP creates a new fasthttp transport.
func NewFastHTTP(cfg FastHTTPConfig) *FastHTTPTransport {
	return &FastHTTPTransport{config: cfg}
}

// newServer creates and configures a fasthttp.Server with the transport's config.
func (t *FastHTTPTransport) newServer(handler Handler) *fasthttp.Server {
	policy := NewProxyPolicy(t.config.TrustedProxies)
	var fhHandler fasthttp.RequestHandler

	// Fast path: if handler implements FastHTTPHandler, call ServeFast directly
	// bypassing adapter allocation (2 allocs/req saved).
	if fh, ok := handler.(FastHTTPHandler); ok {
		fhHandler = func(ctx *fasthttp.RequestCtx) {
			fh.ServeFastHTTP(ctx)
		}
	} else {
		fhHandler = func(ctx *fasthttp.RequestCtx) {
			req := &fasthttpRequest{ctx: ctx, trustProxy: t.config.TrustProxy, proxyPolicy: policy}
			resp := &fasthttpResponseWriter{ctx: ctx}
			handler.ServeKruda(resp, req)
		}
	}

	s := &fasthttp.Server{Handler: fhHandler}

	if t.config.ReadTimeout > 0 {
		s.ReadTimeout = t.config.ReadTimeout
	}
	if t.config.WriteTimeout > 0 {
		s.WriteTimeout = t.config.WriteTimeout
	}
	if t.config.IdleTimeout > 0 {
		s.IdleTimeout = t.config.IdleTimeout
	}
	if t.config.MaxBodySize > 0 {
		s.MaxRequestBodySize = t.config.MaxBodySize
	}
	if t.config.Concurrency > 0 {
		s.Concurrency = t.config.Concurrency
	}

	return s
}

// ListenAndServe starts the fasthttp server.
func (t *FastHTTPTransport) ListenAndServe(addr string, handler Handler) error {
	t.server = t.newServer(handler)
	return t.server.ListenAndServe(addr)
}

// Serve starts the fasthttp server on an existing listener.
func (t *FastHTTPTransport) Serve(ln net.Listener, handler Handler) error {
	t.server = t.newServer(handler)
	return t.server.Serve(ln)
}

// Shutdown gracefully shuts down the server.
func (t *FastHTTPTransport) Shutdown(ctx context.Context) error {
	if t.server == nil {
		return nil
	}
	return t.server.ShutdownWithContext(ctx)
}

type fasthttpRequest struct {
	ctx         *fasthttp.RequestCtx
	trustProxy  bool
	proxyPolicy ProxyPolicy
}

func (r *fasthttpRequest) Method() string {
	return string(r.ctx.Method())
}

func (r *fasthttpRequest) Path() string {
	path := string(r.ctx.Path())
	if path == "" {
		return "/"
	}
	return path
}

func (r *fasthttpRequest) Header(key string) string {
	return string(r.ctx.Request.Header.Peek(key))
}

func (r *fasthttpRequest) Body() ([]byte, error) {
	return r.ctx.PostBody(), nil
}

func (r *fasthttpRequest) QueryParam(key string) string {
	return string(r.ctx.QueryArgs().Peek(key))
}

func (r *fasthttpRequest) PeerAddr() string { return r.ctx.RemoteAddr().String() }

func (r *fasthttpRequest) ForwardedHeaders() (string, int, string, int) {
	xff := r.ctx.Request.Header.PeekAll("X-Forwarded-For")
	n := len(xff)
	var a string
	if n > 0 {
		a = string(xff[0])
	}
	xri := r.ctx.Request.Header.PeekAll("X-Real-IP")
	m := len(xri)
	var b string
	if m > 0 {
		b = string(xri[0])
	}
	return a, n, b, m
}

func (r *fasthttpRequest) RemoteAddr() string {
	if r.trustProxy {
		return r.proxyPolicy.ClientIP(r)
	}
	return r.ctx.RemoteAddr().String()
}

func (r *fasthttpRequest) Cookie(name string) string {
	return string(r.ctx.Request.Header.Cookie(name))
}

func (r *fasthttpRequest) RawRequest() any {
	return r.ctx
}

func (r *fasthttpRequest) MultipartForm(maxBytes int64) (*multipart.Form, error) {
	if maxBytes > 0 {
		cl := int64(r.ctx.Request.Header.ContentLength())
		if cl > maxBytes {
			return nil, fmt.Errorf("multipart: request body too large (%d > %d)", cl, maxBytes)
		}
	}
	return r.ctx.Request.MultipartForm()
}

func (r *fasthttpRequest) Context() context.Context {
	return r.ctx
}

func (r *fasthttpRequest) AllHeaders() map[string]string {
	m := make(map[string]string)
	for k, v := range r.ctx.Request.Header.All() {
		m[string(k)] = string(v)
	}
	return m
}

func (r *fasthttpRequest) RawQuery() string { return string(r.ctx.URI().QueryString()) }

func (r *fasthttpRequest) AllQuery() map[string]string {
	m := make(map[string]string)
	for k, v := range r.ctx.QueryArgs().All() {
		m[string(k)] = string(v)
	}
	return m
}

func (w *fasthttpResponseWriter) SetFileStream(file io.ReadCloser, size int64) {
	bodySize := int(size)
	if int64(bodySize) != size {
		bodySize = -1
	}
	w.ctx.SetBodyStream(file, bodySize)
}

type fasthttpResponseWriter struct {
	ctx *fasthttp.RequestCtx
}

func (w *fasthttpResponseWriter) WriteHeader(statusCode int) {
	w.ctx.SetStatusCode(statusCode)
}

func (w *fasthttpResponseWriter) Header() HeaderMap {
	return &fasthttpHeaderMap{ctx: w.ctx}
}

func (w *fasthttpResponseWriter) Write(data []byte) (int, error) {
	_, err := w.ctx.Write(data)
	return len(data), err
}

type fasthttpHeaderMap struct {
	ctx *fasthttp.RequestCtx
}

func (m *fasthttpHeaderMap) Set(key, value string) { m.ctx.Response.Header.Set(key, value) }
func (m *fasthttpHeaderMap) Add(key, value string) { m.ctx.Response.Header.Add(key, value) }
func (m *fasthttpHeaderMap) Get(key string) string { return string(m.ctx.Response.Header.Peek(key)) }
func (m *fasthttpHeaderMap) Del(key string)        { m.ctx.Response.Header.Del(key) }

// ProxyConfig returns forwarding settings and an owned copy of the allowlist.
func (t *FastHTTPTransport) ProxyConfig() (bool, []string) {
	return t.config.TrustProxy, append([]string(nil), t.config.TrustedProxies...)
}
