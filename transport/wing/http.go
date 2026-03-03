package wing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/go-kruda/kruda/transport"
)

// Compile-time interface assertions.
var (
	_ transport.Request        = (*wingRequest)(nil)
	_ transport.ResponseWriter = (*wingResponse)(nil)
	_ transport.HeaderMap      = (*wingHeaders)(nil)
)

// ----------------------------- HTTP parser -----------------------------

// parseHTTPRequest parses a raw HTTP/1.1 request from buf.
// All strings are safe copies (no reference to buf after return).
// Returns (nil, 0, false) if the request is incomplete or exceeds limits.
// On success, consumed is the number of bytes used by the parsed request,
// allowing callers to preserve any pipelined data that follows.
func parseHTTPRequest(data []byte, limits parserLimits) (*wingRequest, int, bool) {
	// Find end of headers.
	headerEnd := bytes.Index(data, crlfcrlf)
	if headerEnd < 0 {
		return nil, 0, false
	}
	bodyStart := headerEnd + 4

	// Parse request line: "METHOD /path HTTP/1.x\r\n"
	lineEnd := bytes.IndexByte(data, '\n')
	if lineEnd < 0 {
		return nil, 0, false
	}
	line := data[:lineEnd]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	sp1 := bytes.IndexByte(line, ' ')
	if sp1 <= 0 {
		return nil, 0, false
	}
	sp2 := bytes.IndexByte(line[sp1+1:], ' ')
	if sp2 < 0 {
		return nil, 0, false
	}
	sp2 += sp1 + 1
	if sp2 <= sp1+1 {
		return nil, 0, false
	}

	// Validate request line format — version must start with "HTTP/".
	version := line[sp2+1:]
	if len(version) < 5 || !bytes.Equal(version[:5], bHTTPVersionPrefix) {
		return nil, 0, false
	}

	// Safe copies via string().
	method := internMethod(line[:sp1])
	rawPath := line[sp1+1 : sp2]

	// Request-target must start with '/' or be exactly '*'.
	if len(rawPath) == 0 || (rawPath[0] != '/' && !bytes.Equal(rawPath, bStar)) {
		return nil, 0, false
	}

	var path, query string
	if qi := bytes.IndexByte(rawPath, '?'); qi >= 0 {
		path = string(rawPath[:qi])
		query = string(rawPath[qi+1:])
	} else if len(rawPath) == 1 && rawPath[0] == '/' {
		path = "/"
	} else {
		path = string(rawPath)
	}

	// Parse headers (only fields we need).
	contentLength := 0
	contentType := ""
	cookie := ""
	keepAlive := true // HTTP/1.1 default
	headerCount := 0
	contentLengthSeen := false
	hasTE := false            
	hasCL := false
	var extraHdrs [8]struct{ k, v string }
	extraN := 0

	pos := lineEnd + 1
	for pos < headerEnd {
		nlIdx := bytes.IndexByte(data[pos:headerEnd], '\n')
		var hline []byte
		if nlIdx < 0 {
			hline = data[pos:headerEnd]
			pos = headerEnd
		} else {
			// Reject bare LF (without preceding CR) as line terminator.
			absIdx := pos + nlIdx
			if absIdx == 0 || data[absIdx-1] != '\r' {
				return nil, 0, false
			}
			hline = data[pos : pos+nlIdx]
			pos += nlIdx + 1
		}
		if len(hline) > 0 && hline[len(hline)-1] == '\r' {
			hline = hline[:len(hline)-1]
		}

		colon := bytes.IndexByte(hline, ':')
		if colon < 0 {
			continue
		}

		// R1: header count limit (only when configured > 0).
		headerCount++
		if limits.maxHeaderCount > 0 && headerCount > limits.maxHeaderCount {
			return nil, 0, false
		}

		// R1: header size limit — full line (key + ":" + value).
		if limits.maxHeaderSize > 0 && len(hline) > limits.maxHeaderSize {
			return nil, 0, false
		}

		key := bytes.TrimSpace(hline[:colon])
		val := bytes.TrimSpace(hline[colon+1:])

		// Validate header name characters (RFC 7230 token set).
		if !isValidTokenName(key) {
			return nil, 0, false
		}

		// Reject bare CR or LF in header values (CRLF injection).
		if containsCRLF(val) {
			return nil, 0, false
		}

		switch {
		case len(key) == 14 && asciiEqualFold(key, bContentLength):
			// Reject duplicate Content-Length headers.
			if contentLengthSeen {
				return nil, 0, false
			}
			contentLengthSeen = true
			hasCL = true

			// Reject non-numeric Content-Length values.
			if !isAllDigits(val) {
				return nil, 0, false
			}
			contentLength = btoi(val)
		case len(key) == 12 && asciiEqualFold(key, bContentType):
			contentType = string(val)
		case len(key) == 10 && asciiEqualFold(key, bConnection):
			if asciiEqualFold(val, bClose) {
				keepAlive = false
			}
		case len(key) == 17 && asciiEqualFold(key, bTransferEncoding):
			hasTE = true
		case len(key) == 6 && asciiEqualFold(key, bCookie):
			cookie = string(val)
		default:
			if extraN < len(extraHdrs) {
				lk := make([]byte, len(key))
				for i, c := range key {
					if c >= 'A' && c <= 'Z' {
						lk[i] = byte(c + 32)
					} else {
						lk[i] = byte(c)
					}
				}
				extraHdrs[extraN] = struct{ k, v string }{string(lk), string(val)}
				extraN++
			}
		}
	}

	// Reject requests with both Transfer-Encoding and Content-Length (RFC 7230 §3.3.3).
	if hasTE && hasCL {
		return nil, 0, false
	}

	// Verify body completeness.
	if contentLength > maxContentLength {
		return nil, 0, false // reject oversized requests
	}
	if contentLength > 0 {
		if bodyStart+contentLength > len(data) {
			return nil, 0, false // incomplete body
		}
		consumed := bodyStart + contentLength
		body := make([]byte, contentLength) // safe copy
		copy(body, data[bodyStart:bodyStart+contentLength])
		r := acquireRequest()
		r.method = method
		r.path = path
		r.query = query
		r.body = body
		r.contentType = contentType
		r.cookie = cookie
		r.extraHdrs = extraHdrs
		r.extraN = extraN
		r.keepAlive = keepAlive
		return r, consumed, true
	}

	r := acquireRequest()
	r.method = method
	r.path = path
	r.query = query
	r.contentType = contentType
	r.cookie = cookie
	r.extraHdrs = extraHdrs
	r.extraN = extraN
	r.keepAlive = keepAlive
	return r, bodyStart, true
}

var (
	crlfcrlf           = []byte("\r\n\r\n")
	bContentLength     = []byte("content-length")
	bContentType       = []byte("content-type")
	bConnection        = []byte("connection")
	bClose             = []byte("close")
	bTransferEncoding  = []byte("transfer-encoding")
	bCookie            = []byte("cookie")
	bHTTPVersionPrefix = []byte("HTTP/")
	bStar              = []byte("*")
)

// noLimits is a zero-value parserLimits (all unlimited).
var noLimits = parserLimits{}

// tokenTable is a lookup table for RFC 7230 token characters.
// token = 1*tchar
// tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//
//	"^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
var tokenTable [128]bool

func init() {
	// DIGIT
	for c := '0'; c <= '9'; c++ {
		tokenTable[c] = true
	}
	// ALPHA
	for c := 'A'; c <= 'Z'; c++ {
		tokenTable[c] = true
	}
	for c := 'a'; c <= 'z'; c++ {
		tokenTable[c] = true
	}
	// Special tchar
	for _, c := range "!#$%&'*+-.^_`|~" {
		tokenTable[c] = true
	}
}

func asciiEqualFold(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

const maxContentLength = 10 << 20 // 10 MB — reject absurd values

func btoi(b []byte) int {
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		if n > maxContentLength {
			return maxContentLength + 1 // caller will reject
		}
	}
	return n
}

// isValidTokenName checks that every byte in name is a valid RFC 7230 token character.
// Uses the pre-computed tokenTable for O(1) per character lookup.
func isValidTokenName(name []byte) bool {
	if len(name) == 0 {
		return false
	}
	for _, c := range name {
		if c >= 128 || !tokenTable[c] {
			return false
		}
	}
	return true
}

// containsCRLF returns true if b contains any bare CR or LF character.
func containsCRLF(b []byte) bool {
	for _, c := range b {
		if c == '\r' || c == '\n' {
			return true
		}
	}
	return false
}

// isAllDigits returns true if b is non-empty and every byte is an ASCII digit.
func isAllDigits(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ----------------------------- request adapter -----------------------------

var reqPool = sync.Pool{
	New: func() any { return &wingRequest{} },
}

func acquireRequest() *wingRequest {
	return reqPool.Get().(*wingRequest)
}

func releaseRequest(r *wingRequest) {
	*r = wingRequest{}
	reqPool.Put(r)
}

// parseCookieValue finds the value of a named cookie in a Cookie header string.
// e.g. "session=abc; user=tiger" → parseCookieValue(..., "session") = "abc"
func parseCookieValue(cookie, name string) string {
	for len(cookie) > 0 {
		var pair string
		if i := strings.IndexByte(cookie, ';'); i >= 0 {
			pair = strings.TrimSpace(cookie[:i])
			cookie = strings.TrimSpace(cookie[i+1:])
		} else {
			pair = strings.TrimSpace(cookie)
			cookie = ""
		}
		if eq := strings.IndexByte(pair, '='); eq >= 0 && pair[:eq] == name {
			return pair[eq+1:]
		}
	}
	return ""
}

// wingRequest implements transport.Request with safe-copied strings.
type wingRequest struct {
	method      string
	path        string
	query       string
	body        []byte
	contentType string
	cookie      string
	remoteAddr  string
	keepAlive   bool
	fd          int32 // connection fd — for RawRequest().Fd()
	extraHdrs   [8]struct{ k, v string }
	extraN      int
	ctx         context.Context
}

func (r *wingRequest) Method() string         { return r.method }
func (r *wingRequest) Path() string           { return r.path }
func (r *wingRequest) Body() ([]byte, error)  { return r.body, nil }
func (r *wingRequest) RemoteAddr() string     { return r.remoteAddr }
func (r *wingRequest) RawRequest() any        { return r }
func (r *wingRequest) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}
func (r *wingRequest) Cookie(name string) string                      { return parseCookieValue(r.cookie, name) }
func (r *wingRequest) MultipartForm(maxBytes int64) (*multipart.Form, error) {
	ct := r.contentType
	if ct == "" {
		return nil, fmt.Errorf("missing Content-Type")
	}
	// Extract boundary from Content-Type: multipart/form-data; boundary=xxx
	_, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, err
	}
	boundary, ok := params["boundary"]
	if !ok {
		return nil, fmt.Errorf("no boundary in Content-Type")
	}
	if maxBytes <= 0 {
		maxBytes = 32 << 20 // 32 MB default
	}
	mr := multipart.NewReader(io.LimitReader(bytes.NewReader(r.body), maxBytes), boundary)
	return mr.ReadForm(maxBytes)
}

func (r *wingRequest) Header(key string) string {
	if key == "Content-Type" || key == "content-type" {
		return r.contentType
	}
	if key == "Cookie" || key == "cookie" {
		return r.cookie
	}
	lk := strings.ToLower(key)
	for i := range r.extraN {
		if r.extraHdrs[i].k == lk {
			return r.extraHdrs[i].v
		}
	}
	return ""
}

func (r *wingRequest) QueryParam(name string) string {
	q := r.query
	for len(q) > 0 {
		var kv string
		if i := strings.IndexByte(q, '&'); i >= 0 {
			kv = q[:i]
			q = q[i+1:]
		} else {
			kv = q
			q = ""
		}
		if eq := strings.IndexByte(kv, '='); eq >= 0 && kv[:eq] == name {
			return kv[eq+1:]
		}
	}
	return ""
}

// ----------------------------- response adapter -----------------------------

var respPool = sync.Pool{
	New: func() any {
		return &wingResponse{
			buf:  make([]byte, 0, 2048),
			body: make([]byte, 0, 512),
		}
	},
}

func acquireResponse() *wingResponse {
	r := respPool.Get().(*wingResponse)
	r.status = 200
	r.headers.count = 0
	r.body = r.body[:0]
	r.buf = r.buf[:0]
	r.staticResp = nil
	return r
}

func releaseResponse(r *wingResponse) {
	// Cap pool buffers to avoid holding huge allocations.
	if cap(r.buf) > 65536 {
		r.buf = make([]byte, 0, 2048)
	}
	if cap(r.body) > 65536 {
		r.body = make([]byte, 0, 512)
	}
	respPool.Put(r)
}

// wingResponse implements transport.ResponseWriter.
type wingResponse struct {
	status     int
	headers    wingHeaders
	body       []byte
	buf        []byte // scratch buffer for serialization
	staticResp []byte // pre-built full response (if set, buildZeroCopy returns this)
}

func (r *wingResponse) WriteHeader(code int)        { r.status = code }
func (r *wingResponse) Header() transport.HeaderMap { return &r.headers }
func (r *wingResponse) Write(data []byte) (int, error) {
	r.body = append(r.body, data...)
	return len(data), nil
}
func (r *wingResponse) SetStaticResponse(data []byte) { r.staticResp = data }
func (r *wingResponse) SetStaticText(status int, contentType, text string) {
	r.staticResp = transport.GetStaticResponseString(status, contentType, text)
}

// Pre-computed status lines to avoid strconv per response.
var statusLines [600][]byte

func init() {
	codes := [][2]any{
		{200, "OK"}, {201, "Created"}, {204, "No Content"},
		{301, "Moved Permanently"}, {302, "Found"}, {304, "Not Modified"},
		{400, "Bad Request"}, {401, "Unauthorized"}, {403, "Forbidden"},
		{404, "Not Found"}, {405, "Method Not Allowed"}, {409, "Conflict"},
		{413, "Content Too Large"}, {422, "Unprocessable Entity"},
		{429, "Too Many Requests"}, {500, "Internal Server Error"},
		{502, "Bad Gateway"}, {503, "Service Unavailable"},
	}
	for _, pair := range codes {
		code := pair[0].(int)
		text := pair[1].(string)
		statusLines[code] = []byte("HTTP/1.1 " + strconv.Itoa(code) + " " + text + "\r\n")
	}
}

// buildZeroCopy serialises the HTTP response into r.buf and returns
// the buf slice directly (no copy). The caller must hold a reference to
// the returned data (via conn.sendBuf) until the send completes.
// SAFETY: The wingResponse is returned to pool AFTER conn.sendBuf is set,
// but the underlying array is NOT reused until releaseResponse is called
// on the NEXT request cycle, by which time the send has completed.
// cachedDateHdr holds "Date: <RFC1123>\r\n" updated every second.
var cachedDateHdr atomic.Pointer[[]byte]

func init() {
	updateDateHdr()
	go func() {
		for range time.Tick(time.Second) {
			updateDateHdr()
		}
	}()
}

func internMethod(b []byte) string {
	switch len(b) {
	case 3:
		if b[0] == 'G' && b[1] == 'E' && b[2] == 'T' {
			return "GET"
		}
		if b[0] == 'P' && b[1] == 'U' && b[2] == 'T' {
			return "PUT"
		}
	case 4:
		if b[0] == 'P' && b[1] == 'O' && b[2] == 'S' && b[3] == 'T' {
			return "POST"
		}
		if b[0] == 'H' && b[1] == 'E' && b[2] == 'A' && b[3] == 'D' {
			return "HEAD"
		}
	case 5:
		if b[0] == 'P' && b[1] == 'A' && b[2] == 'T' && b[3] == 'C' && b[4] == 'H' {
			return "PATCH"
		}
	case 6:
		if b[0] == 'D' && b[1] == 'E' && b[2] == 'L' && b[3] == 'E' && b[4] == 'T' && b[5] == 'E' {
			return "DELETE"
		}
	}
	return *(*string)(unsafe.Pointer(&b))
}

func updateDateHdr() {
	b := []byte("Date: " + time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT") + "\r\nServer: Kruda\r\n")
	cachedDateHdr.Store(&b)
}

// dateHdr returns the cached Date+Server header chunk.
func dateHdr() []byte {
	return *cachedDateHdr.Load()
}

func (r *wingResponse) buildZeroCopy() []byte {
	if r.staticResp != nil {
		return r.staticResp
	}
	b := r.buf[:0]

	// Status line — use pre-computed when available.
	if r.status > 0 && r.status < len(statusLines) && statusLines[r.status] != nil {
		b = append(b, statusLines[r.status]...)
	} else {
		b = append(b, "HTTP/1.1 "...)
		b = strconv.AppendInt(b, int64(r.status), 10)
		b = append(b, " Unknown\r\n"...)
	}

	// Fixed headers: Date + Server (combined in dateHdr)
	b = append(b, dateHdr()...)

	// Headers — check for Content-Length in the same loop.
	hasCL := false
	for i := 0; i < r.headers.count; i++ {
		b = append(b, r.headers.keys[i]...)
		b = append(b, ": "...)
		b = append(b, r.headers.vals[i]...)
		b = append(b, "\r\n"...)
		if !hasCL && r.headers.keys[i] == "Content-Length" {
			hasCL = true
		}
	}
	if !hasCL {
		b = append(b, "Content-Length: "...)
		b = strconv.AppendInt(b, int64(len(r.body)), 10)
		b = append(b, "\r\n"...)
	}
	b = append(b, "\r\n"...)
	b = append(b, r.body...)

	// Detach buf from response — caller owns this memory now.
	r.buf = nil
	return b
}

// build serialises the HTTP response with a safe copy (for async dispatch).
func (r *wingResponse) build() []byte {
	b := r.buildZeroCopy()
	out := make([]byte, len(b))
	copy(out, b)
	// Restore buf to pool (we made a copy).
	r.buf = b
	return out
}

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
	case 409:
		return "Conflict"
	case 413:
		return "Content Too Large"
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
	default:
		return "Unknown"
	}
}

// ----------------------------- header adapter -----------------------------

// wingHeaders implements transport.HeaderMap with zero-alloc fixed array.
type wingHeaders struct {
	keys  [8]string
	vals  [8]string
	count int
}

func (h *wingHeaders) Set(key, value string) {
	for i := 0; i < h.count; i++ {
		if h.keys[i] == key {
			h.vals[i] = value
			return
		}
	}
	if h.count < len(h.keys) {
		h.keys[h.count] = key
		h.vals[h.count] = value
		h.count++
	}
}

func (h *wingHeaders) Add(key, value string) { h.Set(key, value) }

func (h *wingHeaders) Get(key string) string {
	for i := 0; i < h.count; i++ {
		if h.keys[i] == key {
			return h.vals[i]
		}
	}
	return ""
}

func (h *wingHeaders) Del(key string) {
	for i := 0; i < h.count; i++ {
		if h.keys[i] == key {
			h.count--
			h.keys[i] = h.keys[h.count]
			h.vals[i] = h.vals[h.count]
			return
		}
	}
}
