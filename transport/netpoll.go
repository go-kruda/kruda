//go:build !windows

package transport

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/go-kruda/kruda/internal/bytesconv"
)

// NetpollTransport implements Transport using CloudWeGo's netpoll for
// epoll/kqueue-based I/O multiplexing. It provides a high-performance
// HTTP/1.1 server with zero-copy reads and pooled connection buffers.
type NetpollTransport struct {
	mu        sync.Mutex
	listener  net.Listener
	handler   Handler
	config    NetpollConfig
	connPool  sync.Pool
	closed    atomic.Bool
	wg        sync.WaitGroup
	eventLoop netpoll.EventLoop
}

// NetpollConfig holds configuration for the netpoll transport.
type NetpollConfig struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxBodySize    int
	MaxHeaderBytes int
	TrustProxy     bool
}

// connBuffers holds per-connection reusable buffers to reduce allocations.
// Buffers are pooled via sync.Pool and attached to each connection.
type connBuffers struct {
	readBuf  []byte // 4KB read scratch buffer
	writeBuf []byte // 4KB write scratch buffer
}


// NewNetpoll creates a new netpoll-based transport.
func NewNetpoll(cfg NetpollConfig) (*NetpollTransport, error) {
	t := &NetpollTransport{
		config: cfg,
		connPool: sync.Pool{
			New: func() any {
				return &connBuffers{
					readBuf:  make([]byte, 4096),
					writeBuf: make([]byte, 0, 4096),
				}
			},
		},
	}
	return t, nil
}

// ListenAndServe starts the netpoll event loop on the given address.
// It creates a netpoll listener, configures the event loop with onRequest/onConnect
// callbacks, and blocks until the server is shut down.
func (t *NetpollTransport) ListenAndServe(addr string, handler Handler) error {
	t.handler = handler

	ln, err := netpoll.CreateListener("tcp", addr)
	if err != nil {
		return fmt.Errorf("netpoll: create listener: %w", err)
	}
	t.listener = ln

	var opts []netpoll.Option
	opts = append(opts, netpoll.WithOnConnect(t.onConnect))
	if t.config.ReadTimeout > 0 {
		opts = append(opts, netpoll.WithReadTimeout(t.config.ReadTimeout))
	}
	if t.config.WriteTimeout > 0 {
		opts = append(opts, netpoll.WithWriteTimeout(t.config.WriteTimeout))
	}
	if t.config.IdleTimeout > 0 {
		opts = append(opts, netpoll.WithIdleTimeout(t.config.IdleTimeout))
	}

	evl, err := netpoll.NewEventLoop(t.onRequest, opts...)
	if err != nil {
		ln.Close()
		return fmt.Errorf("netpoll: create event loop: %w", err)
	}
	t.mu.Lock()
	t.eventLoop = evl
	t.mu.Unlock()

	return evl.Serve(ln)
}

// onConnect is a no-op — buffers are pooled per-request to avoid a race
// where keep-alive connections share a returned-to-pool buffer.
func (t *NetpollTransport) onConnect(ctx context.Context, conn netpoll.Connection) context.Context {
	return ctx
}

// onRequest handles a single HTTP/1.1 request on the connection.
// Netpoll's event loop calls this for each readable event; keep-alive
// is achieved by returning nil (netpoll re-arms the read event).
func (t *NetpollTransport) onRequest(ctx context.Context, conn netpoll.Connection) error {
	// Reject new requests if shutting down.
	if t.closed.Load() {
		return conn.Close()
	}

	t.wg.Add(1)
	defer t.wg.Done()

	bufs := t.connPool.Get().(*connBuffers)
	defer func() {
		// Reset write buffer and return to pool.
		bufs.writeBuf = bufs.writeBuf[:0]
		t.connPool.Put(bufs)
	}()

	reader := conn.Reader()

	// Parse the request line and headers from the connection.
	method, path, query, headers, err := t.parseRequest(reader)
	if err != nil {
		// On parse error, send 400 and close.
		write400(conn.Writer())
		return conn.Close()
	}

	// Build the request object.
	req := &netpollRequest{
		method:     method,
		path:       path,
		rawQuery:   query,
		headers:    headers,
		reader:     reader,
		remoteAddr: conn.RemoteAddr(),
		maxBody:    t.config.MaxBodySize,
		trustProxy: t.config.TrustProxy,
	}

	// Build the response writer.
	w := &netpollResponseWriter{
		writer:     conn.Writer(),
		statusCode: 200,
		headers:    netpollHeaderMap{headers: make([][2]string, 0, 8)},
		buf:        bufs.writeBuf,
	}

	// Dispatch to the application handler.
	t.handler.ServeKruda(w, req)

	// Drain unread body for keep-alive framing. Failure → close connection.
	if err := req.drainBody(); err != nil {
		reader.Release()
		return conn.Close()
	}

	// Flush the response to the connection.
	if err := w.flush(); err != nil {
		reader.Release()
		return conn.Close()
	}

	// Check for Connection: close — if so, close the connection.
	// Must check BEFORE Release() since headers are zero-copy strings
	// referencing the reader's internal buffer.
	closeConn := false
	for _, h := range headers {
		if strings.EqualFold(h[0], "connection") && strings.EqualFold(h[1], "close") {
			closeConn = true
			break
		}
	}

	// Release reader buffers after we're done reading headers.
	reader.Release()

	if closeConn {
		return conn.Close()
	}

	return nil
}

// parseRequest reads the HTTP request line and headers from the netpoll reader.
// Returns method, path, query string, and headers as a [][2]string slice.
//
// Zero-copy lifetime: the returned strings reference the reader's internal buffer
// via bytesconv.UnsafeString. They are valid until reader.Release() is called.
// The caller must finish using them before releasing the reader.
func (t *NetpollTransport) parseRequest(reader netpoll.Reader) (method, path, query string, headers [][2]string, err error) {
	// Read the request line: "GET /path?q=1 HTTP/1.1\r\n"
	line, err := reader.Until('\n')
	if err != nil {
		return "", "", "", nil, err
	}

	method, path, query, ok := parseRequestLine(bytesconv.UnsafeString(line))
	if !ok {
		return "", "", "", nil, fmt.Errorf("netpoll: malformed request line")
	}

	// Parse headers until we hit an empty line (\r\n).
	headers = make([][2]string, 0, 16)
	maxHeaders := 128 // sane default limit
	if t.config.MaxHeaderBytes > 0 {
		// Rough estimate: average header ~64 bytes → max count
		maxHeaders = t.config.MaxHeaderBytes / 64
		if maxHeaders < 16 {
			maxHeaders = 16
		}
	}
	totalHeaderBytes := 0
	for {
		line, err = reader.Until('\n')
		if err != nil {
			return "", "", "", nil, err
		}

		// Trim trailing \r\n or \n.
		s := bytesconv.UnsafeString(line)
		s = trimCRLF(s)

		// Empty line signals end of headers.
		if len(s) == 0 {
			break
		}

		// Enforce header limits.
		totalHeaderBytes += len(s)
		if t.config.MaxHeaderBytes > 0 && totalHeaderBytes > t.config.MaxHeaderBytes {
			return "", "", "", nil, fmt.Errorf("netpoll: headers too large")
		}
		if len(headers) >= maxHeaders {
			return "", "", "", nil, fmt.Errorf("netpoll: too many headers")
		}

		// Find the colon separator.
		colon := strings.IndexByte(s, ':')
		if colon < 0 {
			continue // skip malformed header lines
		}

		key := s[:colon]
		val := s[colon+1:]
		// Trim leading space from value (OWS per RFC 7230).
		if len(val) > 0 && val[0] == ' ' {
			val = val[1:]
		}

		headers = append(headers, [2]string{key, val})
	}

	return method, path, query, headers, nil
}

// parseRequestLine splits "GET /path?q=1 HTTP/1.1\r\n" into method, path, query.
func parseRequestLine(line string) (method, path, query string, ok bool) {
	line = trimCRLF(line)

	// Find first space → end of method.
	sp1 := strings.IndexByte(line, ' ')
	if sp1 < 0 {
		return "", "", "", false
	}
	method = line[:sp1]

	// Find second space → end of URI.
	rest := line[sp1+1:]
	sp2 := strings.IndexByte(rest, ' ')
	if sp2 < 0 {
		return "", "", "", false
	}
	uri := rest[:sp2]

	// Split URI into path and query.
	if qmark := strings.IndexByte(uri, '?'); qmark >= 0 {
		path = uri[:qmark]
		query = uri[qmark+1:]
	} else {
		path = uri
	}

	if path == "" {
		path = "/"
	}

	return method, path, query, true
}

// trimCRLF removes trailing \r\n or \n from a string.
func trimCRLF(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	return s
}

// --- netpollRequest: implements transport.Request ---

// netpollRequest implements the transport.Request interface for netpoll connections.
// Body and query params are lazily parsed on first access.
//
// Zero-copy note: method, path, query, and header values reference the netpoll
// reader's internal buffer. They must not be used after reader.Release().
type netpollRequest struct {
	method     string
	path       string
	rawQuery   string
	headers    [][2]string
	reader     netpoll.Reader
	remoteAddr net.Addr
	maxBody    int
	trustProxy bool

	// Lazy body.
	body     []byte
	bodyErr  error
	bodyRead bool

	// Lazy query params.
	queryParams map[string]string
	queryDone   bool
}

// Method returns the HTTP method (GET, POST, etc.).
// Returns a safe copy — the original is a zero-copy string tied to the reader buffer.
func (r *netpollRequest) Method() string { return strings.Clone(r.method) }

// Path returns the request path (without query string).
// Returns a safe copy — the original is a zero-copy string tied to the reader buffer.
func (r *netpollRequest) Path() string { return strings.Clone(r.path) }

// Header returns the value of the named header (case-insensitive).
// Returns a safe copy — header values are zero-copy strings tied to the reader buffer.
func (r *netpollRequest) Header(key string) string {
	for _, h := range r.headers {
		if strings.EqualFold(h[0], key) {
			return strings.Clone(h[1])
		}
	}
	return ""
}

// headerVal returns the value of the named header (case-insensitive).
// Used internally for Content-Length and Transfer-Encoding lookups.
// Returns zero-copy string — caller must not retain after Release().
func (r *netpollRequest) headerVal(key string) string {
	for _, h := range r.headers {
		if strings.EqualFold(h[0], key) {
			return h[1]
		}
	}
	return ""
}

// Body lazily reads the request body based on Content-Length.
// Enforces MaxBodySize — returns ErrBodyTooLarge if exceeded.
// The returned slice is a copy and safe to use after reader.Release().
func (r *netpollRequest) Body() ([]byte, error) {
	if r.bodyRead {
		return r.body, r.bodyErr
	}
	r.bodyRead = true

	clStr := r.headerVal("Content-Length")
	if clStr == "" {
		// Check for chunked Transfer-Encoding — not yet supported.
		if te := r.headerVal("Transfer-Encoding"); strings.Contains(te, "chunked") {
			r.bodyErr = fmt.Errorf("netpoll: chunked Transfer-Encoding not supported")
			return nil, r.bodyErr
		}
		return nil, nil
	}

	cl, err := strconv.Atoi(clStr)
	if err != nil || cl < 0 {
		return nil, nil
	}
	if cl == 0 {
		return nil, nil
	}

	// Enforce max body size before reading.
	if r.maxBody > 0 && cl > r.maxBody {
		r.bodyErr = ErrBodyTooLarge
		return nil, ErrBodyTooLarge
	}

	// ReadBinary returns a copy — safe to use after Release().
	data, err := r.reader.ReadBinary(cl)
	if err != nil {
		r.bodyErr = err
		return nil, err
	}

	r.body = data
	return data, nil
}

// QueryParam returns the value of a query parameter. Parses the query string
// lazily on first call using a custom parser (first value wins).
// Returns a safe copy — query values originate from the zero-copy reader buffer.
func (r *netpollRequest) QueryParam(key string) string {
	if !r.queryDone {
		r.queryParams = parseQuery(r.rawQuery)
		r.queryDone = true
	}
	return strings.Clone(r.queryParams[key])
}

// RemoteAddr returns the client IP address. When TrustProxy is enabled,
// it checks X-Forwarded-For and X-Real-Ip headers first.
// The port is stripped for a consistent bare-IP format.
func (r *netpollRequest) RemoteAddr() string {
	if r.trustProxy {
		if xff := r.Header("X-Forwarded-For"); xff != "" {
			// Take the first IP in the comma-separated chain.
			for i := 0; i < len(xff); i++ {
				if xff[i] == ',' {
					return trimSpace(xff[:i])
				}
			}
			return trimSpace(xff)
		}
		if xri := r.Header("X-Real-Ip"); xri != "" {
			return trimSpace(xri)
		}
	}
	if r.remoteAddr == nil {
		return ""
	}
	return stripPort(r.remoteAddr.String())
}

// Cookie returns the value of the named cookie from the Cookie header.
func (r *netpollRequest) Cookie(name string) string {
	cookieHeader := r.Header("Cookie")
	if cookieHeader == "" {
		return ""
	}
	// Parse "key1=val1; key2=val2" format.
	for cookieHeader != "" {
		var pair string
		if idx := strings.IndexByte(cookieHeader, ';'); idx >= 0 {
			pair = cookieHeader[:idx]
			cookieHeader = cookieHeader[idx+1:]
			// Skip leading space after semicolon.
			if len(cookieHeader) > 0 && cookieHeader[0] == ' ' {
				cookieHeader = cookieHeader[1:]
			}
		} else {
			pair = cookieHeader
			cookieHeader = ""
		}
		if eq := strings.IndexByte(pair, '='); eq >= 0 {
			if pair[:eq] == name {
				return pair[eq+1:]
			}
		}
	}
	return ""
}

// RawRequest returns nil — there is no underlying *http.Request for netpoll connections.
func (r *netpollRequest) RawRequest() any { return nil }

// drainBody skips unread body bytes to keep HTTP/1.1 framing correct.
func (r *netpollRequest) drainBody() error {
	if r.bodyRead {
		return nil
	}
	r.bodyRead = true

	if te := r.headerVal("Transfer-Encoding"); strings.Contains(te, "chunked") {
		return fmt.Errorf("netpoll: cannot drain chunked body")
	}

	clStr := r.headerVal("Content-Length")
	if clStr == "" {
		return nil
	}
	cl, err := strconv.Atoi(clStr)
	if err != nil || cl < 0 {
		return fmt.Errorf("netpoll: invalid Content-Length %q", clStr)
	}
	if cl == 0 {
		return nil
	}
	return r.reader.Skip(cl)
}

// --- netpollResponseWriter: implements transport.ResponseWriter ---

// netpollResponseWriter buffers the HTTP/1.1 response and flushes it to the
// netpoll connection writer. Headers and body are assembled into a single
// buffer to minimize write syscalls.
type netpollResponseWriter struct {
	writer     netpoll.Writer
	statusCode int
	headers    netpollHeaderMap
	buf        []byte // assembled response buffer
	written    bool   // whether WriteHeader has been called
	bodyBuf    []byte // body data accumulated via Write()
}

// WriteHeader sets the HTTP status code. Only the first call takes effect.
func (w *netpollResponseWriter) WriteHeader(statusCode int) {
	if w.written {
		return
	}
	w.statusCode = statusCode
	w.written = true
}

// Header returns the response header map for setting headers before Write.
func (w *netpollResponseWriter) Header() HeaderMap {
	return &w.headers
}

// Write appends data to the response body buffer.
func (w *netpollResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	w.bodyBuf = append(w.bodyBuf, data...)
	return len(data), nil
}

// flush assembles the full HTTP/1.1 response and writes it to the connection.
func (w *netpollResponseWriter) flush() error {
	buf := w.buf[:0]

	// Status line: "HTTP/1.1 200 OK\r\n"
	buf = append(buf, "HTTP/1.1 "...)
	buf = strconv.AppendInt(buf, int64(w.statusCode), 10)
	buf = append(buf, ' ')
	buf = append(buf, statusText(w.statusCode)...)
	buf = append(buf, '\r', '\n')

	// Always write Content-Length for keep-alive framing (1xx/204/304 excluded per RFC 7230).
	if w.headers.Get("Content-Length") == "" &&
		w.statusCode >= 200 && w.statusCode != 204 && w.statusCode != 304 {
		buf = append(buf, "Content-Length: "...)
		buf = strconv.AppendInt(buf, int64(len(w.bodyBuf)), 10)
		buf = append(buf, '\r', '\n')
	}

	// Response headers.
	for _, h := range w.headers.headers {
		buf = append(buf, h[0]...)
		buf = append(buf, ':', ' ')
		buf = append(buf, h[1]...)
		buf = append(buf, '\r', '\n')
	}

	// End of headers.
	buf = append(buf, '\r', '\n')

	// Body.
	if len(w.bodyBuf) > 0 {
		buf = append(buf, w.bodyBuf...)
	}

	// Save buf back for potential reuse.
	w.buf = buf

	// Write the assembled response to the netpoll writer.
	if _, err := w.writer.WriteBinary(buf); err != nil {
		return err
	}
	return w.writer.Flush()
}

// --- netpollHeaderMap: implements transport.HeaderMap ---

// netpollHeaderMap implements HeaderMap using a [][2]string slice.
// This is faster than a map for the typical case of <20 response headers.
// All operations are case-insensitive.
type netpollHeaderMap struct {
	headers [][2]string
}

// Set sets the header to a single value, replacing any existing values.
func (m *netpollHeaderMap) Set(key, value string) {
	for i := range m.headers {
		if strings.EqualFold(m.headers[i][0], key) {
			m.headers[i][1] = value
			return
		}
	}
	m.headers = append(m.headers, [2]string{key, value})
}

// Add appends a header value (for multi-value headers like Set-Cookie).
func (m *netpollHeaderMap) Add(key, value string) {
	m.headers = append(m.headers, [2]string{key, value})
}

// Get returns the first value for the given header key (case-insensitive).
func (m *netpollHeaderMap) Get(key string) string {
	for _, h := range m.headers {
		if strings.EqualFold(h[0], key) {
			return h[1]
		}
	}
	return ""
}

// Del removes all values for the given header key (case-insensitive).
func (m *netpollHeaderMap) Del(key string) {
	n := 0
	for _, h := range m.headers {
		if !strings.EqualFold(h[0], key) {
			m.headers[n] = h
			n++
		}
	}
	m.headers = m.headers[:n]
}

// --- Shutdown ---

// Shutdown gracefully shuts down the netpoll transport.
// It sets the closed flag, shuts down the event loop, and waits for
// in-flight requests to complete (respecting the context deadline).
func (t *NetpollTransport) Shutdown(ctx context.Context) error {
	t.closed.Store(true)

	// Shut down the event loop (stops accepting new connections).
	t.mu.Lock()
	evl := t.eventLoop
	t.mu.Unlock()
	if evl != nil {
		if err := evl.Shutdown(ctx); err != nil {
			return err
		}
	}

	// Wait for in-flight requests to finish, with context deadline.
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// --- Helpers ---

// write400 sends a minimal "400 Bad Request" response with Connection: close
// and flushes it to the connection writer.
func write400(w netpoll.Writer) {
	const resp = "HTTP/1.1 400 Bad Request\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	w.WriteString(resp)
	w.Flush()
}

// parseQuery parses a query string into a map. First value wins for duplicate keys.
// This is a custom implementation to avoid importing net/url.
func parseQuery(query string) map[string]string {
	m := make(map[string]string)
	if query == "" {
		return m
	}

	for query != "" {
		var pair string
		if idx := strings.IndexByte(query, '&'); idx >= 0 {
			pair = query[:idx]
			query = query[idx+1:]
		} else {
			pair = query
			query = ""
		}

		if pair == "" {
			continue
		}

		var key, val string
		if eq := strings.IndexByte(pair, '='); eq >= 0 {
			key = pair[:eq]
			val = pair[eq+1:]
		} else {
			key = pair
		}

		// First value wins. Decode both key and value for net/http consistency.
		dk := queryUnescape(key)
		if _, exists := m[dk]; !exists {
			m[dk] = queryUnescape(val)
		}
	}
	return m
}

// queryUnescape decodes percent-encoded query values.
// Returns the original string if decoding fails (best-effort).
func queryUnescape(s string) string {
	if !strings.ContainsRune(s, '%') && !strings.ContainsRune(s, '+') {
		return s
	}
	var buf []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '%':
			if i+2 < len(s) {
				hi, ok1 := unhex(s[i+1])
				lo, ok2 := unhex(s[i+2])
				if ok1 && ok2 {
					buf = append(buf, hi<<4|lo)
					i += 2
					continue
				}
			}
			buf = append(buf, s[i])
		case '+':
			buf = append(buf, ' ')
		default:
			buf = append(buf, s[i])
		}
	}
	return string(buf)
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// statusText returns the reason phrase for common HTTP status codes.
// Falls back to "Unknown" for unrecognized codes.
func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 307:
		return "Temporary Redirect"
	case 308:
		return "Permanent Redirect"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 408:
		return "Request Timeout"
	case 409:
		return "Conflict"
	case 413:
		return "Content Too Large"
	case 415:
		return "Unsupported Media Type"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	default:
		return "Unknown"
	}
}
