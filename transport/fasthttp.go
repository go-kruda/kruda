//go:build !windows

package transport

import (
	"context"
	"mime/multipart"
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
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	MaxBodySize  int
	Concurrency  int
	TrustProxy   bool
}

// NewFastHTTP creates a new fasthttp transport.
func NewFastHTTP(cfg FastHTTPConfig) *FastHTTPTransport {
	return &FastHTTPTransport{config: cfg}
}

// ListenAndServe starts the fasthttp server.
func (t *FastHTTPTransport) ListenAndServe(addr string, handler Handler) error {
	t.server = &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			req := &fasthttpRequest{ctx: ctx, trustProxy: t.config.TrustProxy}
			resp := &fasthttpResponseWriter{ctx: ctx}
			handler.ServeKruda(resp, req)
		},
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
	if t.config.MaxBodySize > 0 {
		t.server.MaxRequestBodySize = t.config.MaxBodySize
	}
	if t.config.Concurrency > 0 {
		t.server.Concurrency = t.config.Concurrency
	}

	return t.server.ListenAndServe(addr)
}

// Shutdown gracefully shuts down the server.
func (t *FastHTTPTransport) Shutdown(ctx context.Context) error {
	if t.server == nil {
		return nil
	}
	return t.server.ShutdownWithContext(ctx)
}

// --- Request implementation ---

type fasthttpRequest struct {
	ctx        *fasthttp.RequestCtx
	trustProxy bool
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

func (r *fasthttpRequest) RemoteAddr() string {
	if r.trustProxy {
		if ip := string(r.ctx.Request.Header.Peek("X-Forwarded-For")); ip != "" {
			for i := 0; i < len(ip); i++ {
				if ip[i] == ',' {
					return trimSpace(ip[:i])
				}
			}
			return trimSpace(ip)
		}
		if ip := string(r.ctx.Request.Header.Peek("X-Real-Ip")); ip != "" {
			return trimSpace(ip)
		}
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
	mr, err := r.ctx.Request.MultipartForm()
	if err != nil {
		return nil, err
	}
	return mr, nil
}

func (r *fasthttpRequest) Context() context.Context {
	return r.ctx // *fasthttp.RequestCtx implements context.Context
}

func (r *fasthttpRequest) AllHeaders() map[string]string {
	m := make(map[string]string)
	r.ctx.Request.Header.VisitAll(func(k, v []byte) {
		m[string(k)] = string(v)
	})
	return m
}

func (r *fasthttpRequest) AllQuery() map[string]string {
	m := make(map[string]string)
	r.ctx.QueryArgs().VisitAll(func(k, v []byte) {
		m[string(k)] = string(v)
	})
	return m
}

// --- ResponseWriter implementation ---

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
	w.ctx.Write(data)
	return len(data), nil
}

// --- HeaderMap implementation ---

type fasthttpHeaderMap struct {
	ctx *fasthttp.RequestCtx
}

func (m *fasthttpHeaderMap) Set(key, value string) {
	m.ctx.Response.Header.Set(key, value)
}

func (m *fasthttpHeaderMap) Add(key, value string) {
	m.ctx.Response.Header.Add(key, value)
}

func (m *fasthttpHeaderMap) Get(key string) string {
	return string(m.ctx.Response.Header.Peek(key))
}

func (m *fasthttpHeaderMap) Del(key string) {
	m.ctx.Response.Header.Del(key)
}