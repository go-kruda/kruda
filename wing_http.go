//go:build linux || darwin

package kruda

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
	return parseHTTPRequestInternal(data, limits, false)
}

func parseHTTPRequestFast(data []byte, limits parserLimits) (*wingRequest, int, bool) {
	return parseHTTPRequestInternal(data, limits, true)
}

// wingExtraHeader is one request header outside the knownHeader fast-path
// set, stored lowercase-keyed for case-insensitive lookup in Header().
type wingExtraHeader struct{ k, v string }

func parseHTTPRequestInternal(data []byte, limits parserLimits, unsafePath bool) (*wingRequest, int, bool) {
	// RFC 7230 §3.5: skip leading CRLF before request-line.
	skip := 0
	for skip+1 < len(data) && data[skip] == '\r' && data[skip+1] == '\n' {
		skip += 2
	}
	if skip > 0 {
		data = data[skip:]
	}

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

	// Validate request line format — version must be exactly "HTTP/X.Y"
	// (mirrors net/http's ParseHTTPVersion: 8 bytes, single digits, dot at
	// index 6). A bare "HTTP/" or a truncated/oversized version string is
	// malformed and must be rejected, not accepted (anti-smuggling: a front
	// proxy validating strictly would see a different verdict than Wing).
	version := line[sp2+1:]
	if !isValidHTTPVersion(version) {
		return nil, 0, false
	}

	// Method must be a valid RFC 7230 token (net/http's isToken rule); a
	// method containing a delimiter/CTL byte (e.g. a quote) is rejected so
	// Wing never accepts a request line a strict reader would reject
	// (anti-smuggling).
	rawMethod := line[:sp1]
	if !isValidTokenName(rawMethod) {
		return nil, 0, false
	}

	// Safe copies via string().
	method := wingInternMethod(rawMethod)
	rawPath := line[sp1+1 : sp2]

	// Request-target must start with '/' or be exactly '*'.
	if len(rawPath) == 0 || (rawPath[0] != '/' && !bytes.Equal(rawPath, bStar)) {
		return nil, 0, false
	}

	var path, query string
	if qi := bytes.IndexByte(rawPath, '?'); qi >= 0 {
		path = copyOrUnsafeString(rawPath[:qi], unsafePath)
		query = string(rawPath[qi+1:])
	} else if len(rawPath) == 1 && rawPath[0] == '/' {
		path = "/"
	} else {
		path = copyOrUnsafeString(rawPath, unsafePath)
	}

	// Parse headers (only fields we need).
	contentLength := 0
	contentType := ""
	cookie := ""
	host := ""
	accept := ""
	forwardedFor := ""
	realIP := ""
	forwardedForSeen := false // first header wins even if its value is empty (net/http Get)
	realIPSeen := false
	hostUnsafe := false
	acceptUnsafe := false
	keepAlive := true // HTTP/1.1 default
	headerCount := 0
	contentLengthSeen := false
	hasTE := false
	hasCL := false
	var extraHdrs [8]wingExtraHeader
	extraN := 0
	var extraOverflow []wingExtraHeader // spills only when a request carries >8 non-fast headers

	pos := lineEnd + 1
	for pos < headerEnd {
		nlIdx := bytes.IndexByte(data[pos:headerEnd], '\n')
		var hline []byte
		if nlIdx < 0 {
			// Last header line: its own "\r\n" was already consumed as the
			// first half of the crlfcrlf terminator, so this window must
			// hold ONLY the line content — no '\r' at all. A '\r' here is a
			// stray/doubled CR that net/http's line reader keeps as part of
			// the line and rejects on; silently stripping it here would
			// make Wing looser than a strict reader (anti-smuggling).
			hline = data[pos:headerEnd]
			if bytes.IndexByte(hline, '\r') >= 0 {
				return nil, 0, false
			}
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

		// RFC 9112 §5.2: obs-fold (a continuation line starting with SP/HTAB)
		// is unconditionally illegal in a request, on EVERY header line, not
		// just the first — there is no such thing as a legal continuation
		// here. A front proxy that unfolds this onto the previous line's
		// value would see a corrupted combined value (e.g. a mangled
		// Content-Length) and reject; parsing it here as a separate header
		// would silently diverge from that stricter reader (anti-smuggling).
		if len(hline) > 0 && (hline[0] == ' ' || hline[0] == '\t') {
			return nil, 0, false
		}

		colon := bytes.IndexByte(hline, ':')
		if colon < 0 {
			// RFC 9112 §5: every field line must contain a colon; a line
			// without one is either obs-fold (forbidden in requests) or
			// garbage. Rejecting keeps Wing's view identical to any
			// front proxy's (anti-smuggling).
			return nil, 0, false
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

		key := hline[:colon]
		val := trimHTTPSpaces(hline[colon+1:])

		// RFC 9112 §5.1: no whitespace is allowed between the field-name and
		// the colon. net/http's textproto reader rejects this outright rather
		// than tolerating it, precisely because implementations disagree on
		// whether to strip it — accepting it here would silently normalize
		// "Content-Length\t: 5" to a real Content-Length header while a
		// stricter reader (or front proxy) rejects the request outright
		// (anti-smuggling). Check the RAW key's last byte before any
		// trimming, since trimming would erase the evidence.
		if len(key) > 0 && (key[len(key)-1] == ' ' || key[len(key)-1] == '\t') {
			return nil, 0, false
		}

		// Reject CTL bytes in header values (CRLF injection + anti-smuggling:
		// net/http rejects any CTL byte other than HTAB in a header value).
		if hasInvalidHeaderValueByte(val) {
			return nil, 0, false
		}

		switch knownHeader(key) {
		case headerContentLength:
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
			continue
		case headerContentType:
			contentType = string(val)
			continue
		case headerConnection:
			if asciiEqualFold(val, bClose) {
				keepAlive = false
			}
			continue
		case headerTransferEncoding:
			hasTE = true
			continue
		case headerCookie:
			cookie = string(val)
			continue
		case headerHost:
			host = copyOrUnsafeString(val, unsafePath)
			hostUnsafe = unsafePath && len(val) > 0
			continue
		case headerAccept:
			accept = copyOrUnsafeString(val, unsafePath)
			acceptUnsafe = unsafePath && len(val) > 0
			continue
		case headerForwardedFor:
			if !forwardedForSeen {
				forwardedForSeen = true
				forwardedFor = string(val)
			}
			continue
		case headerRealIP:
			if !realIPSeen {
				realIPSeen = true
				realIP = string(val)
			}
			continue
		}

		key = trimHTTPSpaces(key)

		// Validate header name characters (RFC 7230 token set).
		if !isValidTokenName(key) {
			return nil, 0, false
		}

		switch knownHeader(key) {
		case headerContentLength:
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
		case headerContentType:
			contentType = string(val)
		case headerConnection:
			if asciiEqualFold(val, bClose) {
				keepAlive = false
			}
		case headerTransferEncoding:
			hasTE = true
		case headerCookie:
			cookie = string(val)
		case headerHost:
			host = copyOrUnsafeString(val, unsafePath)
			hostUnsafe = unsafePath && len(val) > 0
		case headerAccept:
			accept = copyOrUnsafeString(val, unsafePath)
			acceptUnsafe = unsafePath && len(val) > 0
		case headerForwardedFor:
			if !forwardedForSeen {
				forwardedForSeen = true
				forwardedFor = string(val)
			}
		case headerRealIP:
			if !realIPSeen {
				realIPSeen = true
				realIP = string(val)
			}
		default:
			lk := make([]byte, len(key))
			for i, c := range key {
				if c >= 'A' && c <= 'Z' {
					lk[i] = byte(c + 32)
				} else {
					lk[i] = byte(c)
				}
			}
			h := wingExtraHeader{string(lk), string(val)}
			if extraN < len(extraHdrs) {
				extraHdrs[extraN] = h
				extraN++
			} else {
				if extraOverflow == nil {
					wingHeaderSpills.Add(1)
				}
				extraOverflow = append(extraOverflow, h)
			}
		}
	}

	// Reject requests with both Transfer-Encoding and Content-Length (RFC 7230 §3.3.3).
	if hasTE && hasCL {
		return nil, 0, false
	}

	// Reject Transfer-Encoding: chunked (unsupported; HTTP/1.1 allows chunked only in responses).
	if hasTE {
		return nil, 0, false
	}

	// Verify body completeness.
	if contentLength > maxContentLength {
		return nil, 0, false // reject oversized requests
	}
	if contentLength > 0 {
		// Over BodyLimit — reject here so the slow-path classifier emits a 413.
		// Catches complete in-buffer bodies that fit the read buffer but exceed
		// the limit (the slow path alone only sees bodies larger than the buffer).
		if limits.bodyLimit > 0 && contentLength > limits.bodyLimit {
			return nil, 0, false
		}
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
		r.host = host
		r.accept = accept
		r.forwardedFor = forwardedFor
		r.realIP = realIP
		r.hostUnsafe = hostUnsafe
		r.acceptUnsafe = acceptUnsafe
		// Copy only the populated inline slots; a full value-array copy would
		// move the whole array every request even when there are no extra
		// headers. Slots past extraN are never read (Header scans [0:extraN]),
		// so leaving their stale (bounded) contents is harmless.
		copy(r.extraHdrs[:extraN], extraHdrs[:extraN])
		r.extraN = extraN
		r.extraOverflow = extraOverflow
		r.keepAlive = keepAlive
		r.pathUnsafe = unsafePath && path != "/"
		return r, skip + consumed, true
	}

	r := acquireRequest()
	r.method = method
	r.path = path
	r.query = query
	r.contentType = contentType
	r.cookie = cookie
	r.host = host
	r.accept = accept
	r.forwardedFor = forwardedFor
	r.realIP = realIP
	r.hostUnsafe = hostUnsafe
	r.acceptUnsafe = acceptUnsafe
	// Copy only the populated inline slots (see the with-body path above for
	// why a full value-array copy is avoided on the hot path).
	copy(r.extraHdrs[:extraN], extraHdrs[:extraN])
	r.extraN = extraN
	r.extraOverflow = extraOverflow
	r.keepAlive = keepAlive
	r.pathUnsafe = unsafePath && path != "/"
	return r, skip + bodyStart, true
}

func copyOrUnsafeString(b []byte, unsafeOK bool) string {
	if !unsafeOK {
		return string(b)
	}
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

func finalizeRequestPath(r *wingRequest, f Preset) {
	if !r.pathUnsafe {
		return
	}
	if f.path != "" && r.path == f.path {
		r.path = f.path
	} else {
		r.path = strings.Clone(r.path)
	}
	r.pathUnsafe = false
}

func finalizeRequestCommonHeaders(r *wingRequest) {
	if r.hostUnsafe {
		r.host = strings.Clone(r.host)
		r.hostUnsafe = false
	}
	if r.acceptUnsafe {
		r.accept = strings.Clone(r.accept)
		r.acceptUnsafe = false
	}
}

var (
	crlfcrlf          = []byte("\r\n\r\n")
	bContentLength    = []byte("content-length")
	bContentType      = []byte("content-type")
	bConnection       = []byte("connection")
	bClose            = []byte("close")
	bTransferEncoding = []byte("transfer-encoding")
	bExpect           = []byte("expect")
	bCookie           = []byte("cookie")
	bHost             = []byte("host")
	bAccept           = []byte("accept")
	bForwardedFor     = []byte("x-forwarded-for")
	bRealIP           = []byte("x-real-ip")
	bStar             = []byte("*")
	wing100Continue   = []byte("HTTP/1.1 100 Continue\r\n\r\n")
)

const (
	headerUnknown uint8 = iota
	headerContentLength
	headerContentType
	headerConnection
	headerTransferEncoding
	headerCookie
	headerHost
	headerAccept
	headerForwardedFor
	headerRealIP
)

func knownHeader(key []byte) uint8 {
	switch len(key) {
	case 4:
		if asciiEqualFold(key, bHost) {
			return headerHost
		}
	case 6:
		if asciiEqualFold(key, bCookie) {
			return headerCookie
		}
		if asciiEqualFold(key, bAccept) {
			return headerAccept
		}
	case 9:
		if asciiEqualFold(key, bRealIP) {
			return headerRealIP
		}
	case 15:
		if asciiEqualFold(key, bForwardedFor) {
			return headerForwardedFor
		}
	case 10:
		if asciiEqualFold(key, bConnection) {
			return headerConnection
		}
	case 12:
		if asciiEqualFold(key, bContentType) {
			return headerContentType
		}
	case 14:
		if asciiEqualFold(key, bContentLength) {
			return headerContentLength
		}
	case 17:
		if asciiEqualFold(key, bTransferEncoding) {
			return headerTransferEncoding
		}
	}
	return headerUnknown
}

// noLimits is a zero-value parserLimits (all unlimited).
var noLimits = parserLimits{}

type parseStatus uint8

const (
	parseNeedHeaderMore parseStatus = iota // headers not complete, buffer not full
	parseHeaderTooLarge                    // headers exceed limit / buffer
	parseMalformed                         // protocol error
	parseChunked                           // Transfer-Encoding: chunked body (unsupported)
	parseBodyTooLarge                      // Content-Length > bodyLimit
	parseNeedBody                          // valid headers, body incomplete; need N body bytes total
)

// classifyIncomplete inspects a buffer for which parseHTTPRequestFast returned
// ok==false and decides why. It validates the request line + headers but does
// not allocate a request. need is the total Content-Length when status==parseNeedBody.
// expectContinue is true if the request has "Expect: 100-continue" header.
func classifyIncomplete(data []byte, limits parserLimits) (status parseStatus, need int, expectContinue bool) {
	// strip leading CRLF (mirror parseHTTPRequestInternal)
	for len(data) >= 2 && data[0] == '\r' && data[1] == '\n' {
		data = data[2:]
	}
	headerEnd := bytes.Index(data, crlfcrlf)
	if headerEnd < 0 {
		// Headers still arriving: wait for more. Don't reject on accumulated
		// bytes here — that made the verdict depend on TCP segmentation (a
		// small-line/large-total header block was served when it arrived in one
		// read but 431'd when split). Oversized headers are caught per-line once
		// the block is complete, or by the buffer-full path (ReadBufSize is the
		// total backstop).
		return parseNeedHeaderMore, 0, false
	}
	// Request line must be well-formed.
	lineEnd := bytes.IndexByte(data[:headerEnd], '\n')
	if lineEnd <= 0 {
		return parseMalformed, 0, false
	}
	hasTE := false
	hasCL := false
	cl := 0
	pos := lineEnd + 1
	for pos < headerEnd {
		nl := bytes.IndexByte(data[pos:headerEnd], '\n')
		var hline []byte
		if nl < 0 {
			// Mirror parseHTTPRequestInternal: the last header line's own
			// "\r\n" was already consumed by the crlfcrlf terminator match,
			// so a '\r' surviving in this window is a stray/doubled CR —
			// reject rather than silently accept (anti-smuggling).
			hline = data[pos:headerEnd]
			if bytes.IndexByte(hline, '\r') >= 0 {
				return parseMalformed, 0, false
			}
			pos = headerEnd
		} else {
			hline = data[pos : pos+nl]
			pos += nl + 1
		}
		if n := len(hline); n > 0 && hline[n-1] == '\r' {
			hline = hline[:n-1]
		}
		// Mirror parseHTTPRequestInternal: obs-fold is illegal on EVERY
		// header line of a request, not just the first (RFC 9112 §5.2).
		if len(hline) > 0 && (hline[0] == ' ' || hline[0] == '\t') {
			return parseMalformed, 0, false
		}
		// A single over-limit header line is 431, not a silent close — mirror
		// the parser's per-line check so the client learns why it was rejected.
		if limits.maxHeaderSize > 0 && len(hline) > limits.maxHeaderSize {
			return parseHeaderTooLarge, 0, false
		}
		colon := bytes.IndexByte(hline, ':')
		if colon <= 0 {
			return parseMalformed, 0, false
		}
		// RFC 9112 §5.1: mirror parseHTTPRequestInternal's rejection of
		// whitespace between the field-name and the colon (anti-smuggling —
		// see the comment there for the full rationale). Check the raw
		// key's last byte before trimming.
		if raw := hline[:colon]; len(raw) > 0 && (raw[len(raw)-1] == ' ' || raw[len(raw)-1] == '\t') {
			return parseMalformed, 0, false
		}
		name := bytes.ToLower(bytes.TrimSpace(hline[:colon]))
		val := bytes.TrimSpace(hline[colon+1:])
		// Mirror parseHTTPRequestInternal's header-value byte validation.
		if hasInvalidHeaderValueByte(val) {
			return parseMalformed, 0, false
		}
		switch knownHeader(name) {
		case headerContentLength:
			if hasCL {
				return parseMalformed, 0, false // duplicate CL
			}
			hasCL = true
			n, err := strconv.Atoi(string(val))
			if err != nil || n < 0 {
				return parseMalformed, 0, false
			}
			cl = n
		case headerTransferEncoding:
			hasTE = true
		default:
			if asciiEqualFold(name, bExpect) {
				if asciiEqualFold(val, []byte("100-continue")) {
					expectContinue = true
				}
			}
		}
	}
	if hasTE && hasCL {
		return parseMalformed, 0, false
	}
	if hasTE {
		return parseChunked, 0, false
	}
	if !hasCL || cl == 0 {
		return parseMalformed, 0, false
	}
	if limits.bodyLimit > 0 && cl > limits.bodyLimit {
		return parseBodyTooLarge, 0, expectContinue
	}
	return parseNeedBody, cl, expectContinue
}

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
		if n > maxContentLength/10 {
			return maxContentLength + 1
		}
		n = n*10 + int(c-'0')
		if n > maxContentLength {
			return maxContentLength + 1
		}
	}
	return n
}

func trimHTTPSpaces(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	for len(b) > 0 {
		last := b[len(b)-1]
		if last != ' ' && last != '\t' {
			break
		}
		b = b[:len(b)-1]
	}
	return b
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

// hasInvalidHeaderValueByte reports whether b contains a byte that
// net/http's textproto reader rejects in a header value: any CTL byte
// (0x00-0x1F, 0x7F) other than HTAB (0x09). SP, VCHAR, and obs-text
// (0x80-0xFF) are all permitted. Matching this exactly (rather than only
// checking for CR/LF) closes the anti-smuggling gap where Wing accepted a
// value byte (e.g. DEL, a raw control char) that a strict reader rejects.
func hasInvalidHeaderValueByte(b []byte) bool {
	for _, c := range b {
		if c == '\t' {
			continue
		}
		if c < 0x20 || c == 0x7f {
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

// isValidHTTPVersion reports whether v is exactly "HTTP/X.Y" where X and Y
// are single ASCII digits — mirrors net/http's ParseHTTPVersion (RFC 7230
// §2.6). A bare "HTTP/", a missing minor version, or trailing garbage after
// the version must be rejected so Wing never accepts a request line a
// strict front proxy would reject (anti-smuggling).
func isValidHTTPVersion(v []byte) bool {
	if len(v) != 8 {
		return false
	}
	if v[0] != 'H' || v[1] != 'T' || v[2] != 'T' || v[3] != 'P' || v[4] != '/' || v[6] != '.' {
		return false
	}
	return v[5] >= '0' && v[5] <= '9' && v[7] >= '0' && v[7] <= '9'
}

// ----------------------------- request adapter -----------------------------

var reqPool = sync.Pool{
	New: func() any { return &wingRequest{} },
}

func acquireRequest() *wingRequest {
	return reqPool.Get().(*wingRequest)
}

func releaseRequest(r *wingRequest) {
	// Soft reset — keep allocated slices, clear values only
	r.method = ""
	r.path = ""
	r.query = ""
	r.body = r.body[:0]
	r.contentType = ""
	r.cookie = ""
	r.host = ""
	r.accept = ""
	r.forwardedFor = ""
	r.realIP = ""
	r.hostUnsafe = false
	r.acceptUnsafe = false
	r.remoteAddr = ""
	r.remoteAddrRef = nil
	r.trustProxy = false
	r.keepAlive = false
	r.pathUnsafe = false
	r.fd = 0
	r.extraN = 0
	r.extraOverflow = nil
	r.ctx = nil
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
	method        string
	path          string
	query         string
	body          []byte
	contentType   string
	cookie        string
	host          string
	accept        string
	forwardedFor  string
	realIP        string
	remoteAddr    string
	remoteAddrRef *string
	trustProxy    bool
	keepAlive     bool
	pathUnsafe    bool
	hostUnsafe    bool
	acceptUnsafe  bool
	fd            int32 // connection fd — for RawRequest().Fd()
	extraHdrs     [8]wingExtraHeader
	extraN        int
	extraOverflow []wingExtraHeader
	ctx           context.Context
}

func (r *wingRequest) Method() string        { return r.method }
func (r *wingRequest) Path() string          { return r.path }
func (r *wingRequest) Body() ([]byte, error) { return r.body, nil }
func (r *wingRequest) RemoteAddr() string {
	if r.trustProxy {
		if xff := r.forwardedFor; xff != "" {
			// Take the first (leftmost) IP — the original client.
			if i := strings.IndexByte(xff, ','); i >= 0 {
				xff = xff[:i]
			}
			return strings.TrimSpace(xff)
		}
		if r.realIP != "" {
			return strings.TrimSpace(r.realIP)
		}
	}
	if r.remoteAddr != "" {
		return r.remoteAddr
	}
	if r.remoteAddrRef != nil {
		if *r.remoteAddrRef == "" && r.fd > 0 {
			*r.remoteAddrRef = getPeerAddr(r.fd)
		}
		r.remoteAddr = *r.remoteAddrRef
		return r.remoteAddr
	}
	if r.fd > 0 {
		r.remoteAddr = getPeerAddr(r.fd)
	}
	return r.remoteAddr
}
func (r *wingRequest) RawRequest() any { return r }
func (r *wingRequest) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}
func (r *wingRequest) Cookie(name string) string { return parseCookieValue(r.cookie, name) }
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
	if key == "Host" || key == "host" {
		if r.hostUnsafe {
			r.host = strings.Clone(r.host)
			r.hostUnsafe = false
		}
		return r.host
	}
	if key == "Accept" || key == "accept" {
		if r.acceptUnsafe {
			r.accept = strings.Clone(r.accept)
			r.acceptUnsafe = false
		}
		return r.accept
	}
	if strings.EqualFold(key, "x-forwarded-for") {
		return r.forwardedFor
	}
	if strings.EqualFold(key, "x-real-ip") {
		return r.realIP
	}
	lk := strings.ToLower(key)
	for i := range r.extraN {
		if r.extraHdrs[i].k == lk {
			return r.extraHdrs[i].v
		}
	}
	for i := range r.extraOverflow {
		if r.extraOverflow[i].k == lk {
			return r.extraOverflow[i].v
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
	r.headers.reset()
	r.body = r.body[:0]
	r.buf = r.buf[:0]
	r.staticResp = nil
	r.jsonFast = false
	r.fileFd = 0
	r.fileSize = 0
	r.responseMode = responseGeneric
	r.stringFast = false
	r.stringBody = ""
	r.stringContentType = ""
	return r
}

func releaseResponse(r *wingResponse) {
	r.status = 0
	r.staticResp = nil
	r.jsonFast = false
	r.responseMode = responseGeneric
	r.stringFast = false
	r.stringBody = ""
	r.stringContentType = ""
	r.fileFd = 0
	r.fileSize = 0
	r.headers.reset()
	if cap(r.buf) > 65536 {
		r.buf = make([]byte, 0, 2048)
	} else {
		r.buf = r.buf[:0]
	}
	if cap(r.body) > 65536 {
		r.body = make([]byte, 0, 512)
	} else {
		r.body = r.body[:0]
	}
	if r.jsonBuf.Cap() > 65536 {
		r.jsonBuf = bytes.Buffer{}
	} else {
		r.jsonBuf.Reset()
	}
	respPool.Put(r)
}

// wingResponse implements transport.ResponseWriter.
type wingResponse struct {
	status            int
	headers           wingHeaders
	body              []byte
	buf               []byte // scratch buffer for serialization
	jsonBuf           bytes.Buffer
	staticResp        []byte // pre-built full response (if set, buildZeroCopy returns this)
	jsonFast          bool   // SetJSON fast path — skip header interface, write status+json directly
	responseMode      responseMode
	stringFast        bool
	stringBody        string
	stringContentType string
	fileFd            int32 // sendfile fd (0 = not a file response)
	fileSize          int64 // sendfile byte count
}

func (r *wingResponse) WriteHeader(code int)        { r.status = code }
func (r *wingResponse) Header() transport.HeaderMap { return &r.headers }
func (r *wingResponse) Write(data []byte) (int, error) {
	r.body = append(r.body, data...)
	return len(data), nil
}
func (r *wingResponse) SetStaticResponse(data []byte) { r.staticResp = data }

// SetStringBody implements transport.StringResponder — the zero-copy string
// fast lane (twin of SetJSON). The response is serialized in one pass as
// status + Date + Content-Type + Content-Length + body; the body string is
// referenced until serialization, never copied.
func (r *wingResponse) SetStringBody(status int, contentType, body string) {
	r.status = status
	r.headers.reset()
	r.body = r.body[:0]
	r.staticResp = nil
	r.jsonFast = false
	r.stringFast = true
	r.stringBody = body
	r.stringContentType = contentType
}

// SetSendFile configures the response to use sendfile(2) for zero-copy file transfer.
func (r *wingResponse) SetSendFile(fd int32, size int64) {
	r.fileFd = fd
	r.fileSize = size
}
func (r *wingResponse) SetJSON(status int, data []byte) {
	r.status = status
	r.body = data
	r.jsonFast = true
}
func (r *wingResponse) SetJSONStream(status int, enc func(buf *bytes.Buffer, v any) error, v any) error {
	r.jsonBuf.Reset()
	if err := enc(&r.jsonBuf, v); err != nil {
		return err
	}
	r.status = status
	r.body = r.jsonBuf.Bytes()
	r.jsonFast = true
	return nil
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
		{429, "Too Many Requests"}, {431, "Request Header Fields Too Large"},
		{500, "Internal Server Error"}, {501, "Not Implemented"},
		{502, "Bad Gateway"}, {503, "Service Unavailable"},
	}
	for _, pair := range codes {
		code := pair[0].(int)
		text := pair[1].(string)
		statusLines[code] = []byte("HTTP/1.1 " + strconv.Itoa(code) + " " + text + "\r\n")
	}

	// Pre-build status-close responses to avoid any lazy-init races.
	for _, code := range []int{400, 413, 431, 500, 501, 503} {
		if statusLines[code] == nil {
			continue
		}
		b := append([]byte{}, statusLines[code]...)
		b = append(b, "Content-Length: 0\r\nConnection: close\r\n\r\n"...)
		statusCloseCache[code] = b
	}
}

// statusCloseCache holds pre-computed minimal status-close responses.
var statusCloseCache [600][]byte

// wingStatusClose returns a minimal HTTP/1.1 error response with an empty body
// and Connection: close. Safe to call concurrently; responses are pre-built in init().
func wingStatusClose(status int) []byte {
	if status > 0 && status < len(statusCloseCache) && statusCloseCache[status] != nil {
		return statusCloseCache[status]
	}
	// Fallback: build on the fly (won't be called in normal operation).
	line := statusLines[200]
	if status > 0 && status < len(statusLines) && statusLines[status] != nil {
		line = statusLines[status]
	}
	b := append([]byte{}, line...)
	b = append(b, "Content-Length: 0\r\nConnection: close\r\n\r\n"...)
	return b
}

// buildZeroCopy serialises the HTTP response into r.buf and returns
// the buf slice directly (no copy). The caller must hold a reference to
// the returned data (via conn.sendBuf) until the send completes.
// SAFETY: The wingResponse is returned to pool AFTER conn.sendBuf is set,
// but the underlying array is NOT reused until releaseResponse is called
// on the NEXT request cycle, by which time the send has completed.
// cachedDateHdr holds "Date: <RFC1123>\r\n" updated every second.
var cachedDateHdr atomic.Pointer[[]byte]

const jsonHeaderContentLength = "Content-Type: application/json; charset=utf-8\r\nContent-Length: "

func init() {
	updateDateHdr()
	go func() {
		for range time.Tick(time.Second) {
			updateDateHdr()
		}
	}()
}

// methodTable uses XOR hash for O(1) method lookup (silverlining technique).
var methodTable [256]string

func init() {
	methodTable['G'^'E'+'T'] = "GET"
	methodTable['P'^'U'+'T'] = "PUT"
	methodTable['P'^'O'+'S'] = "POST"
	methodTable['H'^'E'+'A'] = "HEAD"
	methodTable['P'^'A'+'T'] = "PATCH"
	methodTable['D'^'E'+'L'] = "DELETE"
}

func wingInternMethod(b []byte) string {
	if len(b) >= 3 {
		if m := methodTable[b[0]^b[1]+b[2]]; m != "" && len(m) == len(b) {
			return m
		}
	}
	return string(b)
}

func updateDateHdr() {
	b := []byte("Date: " + time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT") + "\r\n")
	cachedDateHdr.Store(&b)
}

// dateHdr returns the cached Date header chunk.
func dateHdr() []byte {
	return *cachedDateHdr.Load()
}

func (r *wingResponse) buildZeroCopy() []byte {
	if r.staticResp != nil {
		return r.staticResp
	}

	// Sendfile path: build headers only, body sent via sendfile(2).
	if r.fileFd > 0 {
		return r.buildHeadersOnly()
	}
	b := r.buf[:0]

	if r.stringFast {
		b = r.appendStringTo(b)
		r.buf = nil
		return b
	}

	// JSON fast path — status + Date + Content-Type:json + Content-Length + body
	if r.jsonFast {
		statusLine := jsonStatusLine(r.status)
		date := dateHdr()
		b = growForAppend(b, len(statusLine)+len(date)+len(jsonHeaderContentLength)+contentLengthValueLen(len(r.body))+4+len(r.body))
		b = append(b, statusLine...)
		b = append(b, date...)
		b = append(b, jsonHeaderContentLength...)
		b = appendContentLengthValue(b, len(r.body))
		b = append(b, "\r\n\r\n"...)
		b = append(b, r.body...)
		r.buf = nil
		return b
	}

	b, hasCL := appendStatusAndHeaders(b, r.status, &r.headers)

	// Auto-inject Content-Length when the user did not set it explicitly.
	if !hasCL {
		b = append(b, "Content-Length: "...)
		b = appendContentLengthValue(b, len(r.body))
		b = append(b, "\r\n"...)
	}
	b = append(b, "\r\n"...)
	b = append(b, r.body...)

	// Detach buf from response — caller owns this memory now.
	r.buf = nil
	return b
}

// appendStatusAndHeaders appends the HTTP/1.1 status line, the Date header,
// and all user-set headers in h to b. It does NOT append a Content-Length or
// the blank line that terminates the header section; callers are responsible
// for those (allowing both buffered responses and streaming writers to reuse
// this helper without a Content-Length being injected). hasCL reports whether
// a Content-Length header was already present in h, detected in the same walk
// so buffered callers avoid a second scan.
func appendStatusAndHeaders(b []byte, status int, h *wingHeaders) (out []byte, hasCL bool) {
	if status > 0 && status < len(statusLines) && statusLines[status] != nil {
		b = append(b, statusLines[status]...)
	} else {
		b = append(b, "HTTP/1.1 "...)
		b = strconv.AppendInt(b, int64(status), 10)
		b = append(b, " Unknown\r\n"...)
	}
	b = append(b, dateHdr()...)
	for i := 0; i < h.count; i++ {
		b = append(b, h.keys[i]...)
		b = append(b, ": "...)
		b = append(b, h.vals[i]...)
		b = append(b, "\r\n"...)
		if !hasCL && h.keys[i] == "Content-Length" {
			hasCL = true
		}
	}
	for i := 0; i < len(h.extra); i++ {
		b = append(b, h.extra[i].key...)
		b = append(b, ": "...)
		b = append(b, h.extra[i].val...)
		b = append(b, "\r\n"...)
		if !hasCL && h.extra[i].key == "Content-Length" {
			hasCL = true
		}
	}
	return b, hasCL
}

func (r *wingResponse) appendStringTo(b []byte) []byte {
	b = growForAppend(b, 128+len(r.stringContentType)+len(r.stringBody))
	if r.status > 0 && r.status < len(statusLines) && statusLines[r.status] != nil {
		b = append(b, statusLines[r.status]...)
	} else {
		b = append(b, "HTTP/1.1 "...)
		b = strconv.AppendInt(b, int64(r.status), 10)
		b = append(b, " Unknown\r\n"...)
	}
	b = append(b, dateHdr()...)
	if r.stringContentType != "" {
		b = append(b, "Content-Type: "...)
		b = append(b, r.stringContentType...)
		b = append(b, "\r\n"...)
	}
	b = append(b, "Content-Length: "...)
	b = appendContentLengthValue(b, len(r.stringBody))
	b = append(b, "\r\n\r\n"...)
	b = append(b, r.stringBody...)
	return b
}

func (r *wingResponse) appendJSONTo(b []byte) []byte {
	statusLine := jsonStatusLine(r.status)
	date := dateHdr()
	b = growForAppend(b, len(statusLine)+len(date)+len(jsonHeaderContentLength)+contentLengthValueLen(len(r.body))+4+len(r.body))
	b = append(b, statusLine...)
	b = append(b, date...)
	b = append(b, jsonHeaderContentLength...)
	b = appendContentLengthValue(b, len(r.body))
	b = append(b, "\r\n\r\n"...)
	b = append(b, r.body...)
	return b
}

func jsonStatusLine(status int) []byte {
	if status > 0 && status < len(statusLines) && statusLines[status] != nil {
		return statusLines[status]
	}
	return statusLines[200]
}

func growForAppend(b []byte, extra int) []byte {
	if cap(b)-len(b) >= extra {
		return b
	}
	out := make([]byte, len(b), len(b)+extra)
	copy(out, b)
	return out
}

func appendContentLengthValue(b []byte, n int) []byte {
	if n >= 0 && n < len(contentLengthStrings) {
		return append(b, contentLengthStrings[n]...)
	}
	return strconv.AppendInt(b, int64(n), 10)
}

func contentLengthValueLen(n int) int {
	if n >= 0 && n < len(contentLengthStrings) {
		return len(contentLengthStrings[n])
	}
	if n == 0 {
		return 1
	}
	if n < 0 {
		return 1 + decimalLen(-n)
	}
	return decimalLen(n)
}

func decimalLen(n int) int {
	digits := 0
	for n > 0 {
		digits++
		n /= 10
	}
	return digits
}

// build serialises the HTTP response with a safe copy (for async dispatch).
func (r *wingResponse) build() []byte {
	b := r.buildZeroCopy()
	out := make([]byte, len(b))
	copy(out, b)
	r.buf = b
	return out
}

// buildHeadersOnly builds HTTP headers without body (for sendfile responses).
func (r *wingResponse) buildHeadersOnly() []byte {
	b := r.buf[:0]
	if r.status > 0 && r.status < len(statusLines) && statusLines[r.status] != nil {
		b = append(b, statusLines[r.status]...)
	} else {
		b = append(b, "HTTP/1.1 200 OK\r\n"...)
	}
	b = append(b, dateHdr()...)
	for i := 0; i < r.headers.count; i++ {
		b = append(b, r.headers.keys[i]...)
		b = append(b, ": "...)
		b = append(b, r.headers.vals[i]...)
		b = append(b, "\r\n"...)
	}
	for i := 0; i < len(r.headers.extra); i++ {
		b = append(b, r.headers.extra[i].key...)
		b = append(b, ": "...)
		b = append(b, r.headers.extra[i].val...)
		b = append(b, "\r\n"...)
	}
	b = append(b, "Content-Length: "...)
	if r.fileSize >= 0 && r.fileSize < int64(len(contentLengthStrings)) {
		b = append(b, contentLengthStrings[int(r.fileSize)]...)
	} else {
		b = strconv.AppendInt(b, r.fileSize, 10)
	}
	b = append(b, "\r\n\r\n"...)
	r.buf = nil
	return b
}

// ----------------------------- header adapter -----------------------------

// wingHeaders implements transport.HeaderMap with zero-alloc fixed array.
type wingHeaders struct {
	keys  [8]string
	vals  [8]string
	extra []wingHeader
	count int
}

type wingHeader struct {
	key string
	val string
}

func (h *wingHeaders) reset() {
	h.count = 0
	for i := range h.extra {
		h.extra[i] = wingHeader{}
	}
	h.extra = h.extra[:0]
}

func (h *wingHeaders) Set(key, value string) {
	for i := 0; i < h.count; i++ {
		if h.keys[i] == key {
			h.vals[i] = value
			return
		}
	}
	for i := 0; i < len(h.extra); i++ {
		if h.extra[i].key == key {
			h.extra[i].val = value
			return
		}
	}
	if h.count < len(h.keys) {
		h.keys[h.count] = key
		h.vals[h.count] = value
		h.count++
		return
	}
	h.extra = append(h.extra, wingHeader{key: key, val: value})
}

// Add appends a new key-value pair without replacing existing keys.
// Required for multi-value headers such as Set-Cookie.
func (h *wingHeaders) Add(key, value string) {
	if h.count < len(h.keys) {
		h.keys[h.count] = key
		h.vals[h.count] = value
		h.count++
		return
	}
	h.extra = append(h.extra, wingHeader{key: key, val: value})
}

func (h *wingHeaders) Get(key string) string {
	for i := 0; i < h.count; i++ {
		if h.keys[i] == key {
			return h.vals[i]
		}
	}
	for i := 0; i < len(h.extra); i++ {
		if h.extra[i].key == key {
			return h.extra[i].val
		}
	}
	return ""
}

func (h *wingHeaders) Del(key string) {
	for i := 0; i < h.count; {
		if h.keys[i] == key {
			h.count--
			h.keys[i] = h.keys[h.count]
			h.vals[i] = h.vals[h.count]
			// Don't increment — recheck swapped element.
		} else {
			i++
		}
	}
	for i := 0; i < len(h.extra); {
		if h.extra[i].key == key {
			last := len(h.extra) - 1
			h.extra[i] = h.extra[last]
			h.extra[last] = wingHeader{}
			h.extra = h.extra[:last]
		} else {
			i++
		}
	}
}
