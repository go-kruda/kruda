// Package quic provides an HTTP/3 transport for Kruda using the QUIC protocol.
// This is a separate Go module to avoid pulling the quic-go dependency
// for users who only need net/http or Netpoll transports.
package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-kruda/kruda/transport"
	"github.com/quic-go/quic-go/http3"
)

// QUICTransport implements transport.Transport for HTTP/3 via QUIC.
type QUICTransport struct {
	mu        sync.Mutex
	server    *http3.Server
	tlsConfig *tls.Config
	config    Config
	handler   transport.Handler
}

// Config holds HTTP/3 transport configuration.
type Config struct {
	TLSCertFile string // required: path to TLS certificate file
	TLSKeyFile  string // required: path to TLS private key file
	MaxBodySize int    // max request body size in bytes (0 = unlimited)
}

// New creates a new HTTP/3 QUIC transport.
// Returns an error if the TLS certificate or key cannot be loaded.
func New(cfg Config) (*QUICTransport, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("quic: load TLS cert: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"},
	}
	return &QUICTransport{tlsConfig: tlsCfg, config: cfg}, nil
}

// ListenAndServe starts the HTTP/3 server on the given UDP address.
func (t *QUICTransport) ListenAndServe(addr string, handler transport.Handler) error {
	t.mu.Lock()
	t.handler = handler
	t.server = &http3.Server{
		Addr:      addr,
		TLSConfig: t.tlsConfig,
		Handler:   &quicAdapter{handler: handler, maxBody: t.config.MaxBodySize},
	}
	srv := t.server
	t.mu.Unlock()

	return srv.ListenAndServe()
}

// Shutdown gracefully closes all QUIC connections.
// Respects the context deadline — waits for in-flight requests to complete.
func (t *QUICTransport) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	srv := t.server
	t.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// quicAdapter bridges net/http.Handler to transport.Handler.
// quic-go's http3.Server uses the standard net/http.Handler interface,
// so we create local wrappers that implement transport.Request and
// transport.ResponseWriter by delegating to *http.Request and http.ResponseWriter.
type quicAdapter struct {
	handler transport.Handler
	maxBody int
}

func (a *quicAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := &quicRequest{r: r, maxBody: a.maxBody}
	resp := &quicResponseWriter{w: w, statusCode: 200}
	a.handler.ServeKruda(resp, req)
}

// --- Request wrapper ---

// quicRequest implements transport.Request by delegating to *http.Request.
type quicRequest struct {
	r         *http.Request
	maxBody   int
	body      []byte
	bodyErr   error
	bodyRead  bool
	queryVals url.Values
	queryDone bool
}

func (r *quicRequest) Method() string { return r.r.Method }

func (r *quicRequest) Path() string {
	path := r.r.URL.Path
	if path == "" {
		return "/"
	}
	return path
}

func (r *quicRequest) Header(key string) string {
	return r.r.Header.Get(key)
}

func (r *quicRequest) Body() ([]byte, error) {
	if r.bodyRead {
		return r.body, r.bodyErr
	}
	r.bodyRead = true
	if r.r.Body == nil {
		return nil, nil
	}
	defer r.r.Body.Close()

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
		r.bodyErr = fmt.Errorf("request body too large")
		return nil, r.bodyErr
	}
	r.body = data
	return data, nil
}

func (r *quicRequest) QueryParam(key string) string {
	if !r.queryDone {
		r.queryVals = r.r.URL.Query()
		r.queryDone = true
	}
	return r.queryVals.Get(key)
}

func (r *quicRequest) RemoteAddr() string {
	addr := r.r.RemoteAddr
	// Strip port for consistent bare-IP format (matches nethttp/netpoll behavior).
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

func (r *quicRequest) Cookie(name string) string {
	cookie, err := r.r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (r *quicRequest) RawRequest() any { return r.r }

// --- ResponseWriter wrapper ---

// quicResponseWriter implements transport.ResponseWriter by delegating to http.ResponseWriter.
type quicResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
	written    bool
}

func (w *quicResponseWriter) WriteHeader(statusCode int) {
	if w.written {
		return
	}
	w.statusCode = statusCode
	w.w.WriteHeader(statusCode)
	w.written = true
}

func (w *quicResponseWriter) Header() transport.HeaderMap {
	return &quicHeaderMap{h: w.w.Header()}
}

func (w *quicResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.w.WriteHeader(w.statusCode)
		w.written = true
	}
	return w.w.Write(data)
}

// --- HeaderMap wrapper ---

// quicHeaderMap implements transport.HeaderMap by delegating to http.Header.
type quicHeaderMap struct {
	h http.Header
}

func (m *quicHeaderMap) Set(key, value string) { m.h.Set(key, value) }
func (m *quicHeaderMap) Get(key string) string { return m.h.Get(key) }
func (m *quicHeaderMap) Del(key string)        { m.h.Del(key) }
func (m *quicHeaderMap) Add(key, value string) { m.h.Add(key, value) }
