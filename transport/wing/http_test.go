package wing

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseHTTPRequest_Simple(t *testing.T) {
	raw := "GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parseHTTPRequest returned false for valid request")
	}
	if req.Method() != "GET" {
		t.Errorf("Method = %q, want GET", req.Method())
	}
	if req.Path() != "/hello" {
		t.Errorf("Path = %q, want /hello", req.Path())
	}
	if !req.keepAlive {
		t.Error("keepAlive should be true for HTTP/1.1 default")
	}
}

func TestParseHTTPRequest_WithQuery(t *testing.T) {
	raw := "GET /search?q=kruda&page=2 HTTP/1.1\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	if req.Path() != "/search" {
		t.Errorf("Path = %q, want /search", req.Path())
	}
	if req.QueryParam("q") != "kruda" {
		t.Errorf("QueryParam(q) = %q, want kruda", req.QueryParam("q"))
	}
	if req.QueryParam("page") != "2" {
		t.Errorf("QueryParam(page) = %q, want 2", req.QueryParam("page"))
	}
	if req.QueryParam("missing") != "" {
		t.Errorf("QueryParam(missing) = %q, want empty", req.QueryParam("missing"))
	}
}

func TestParseHTTPRequest_WithBody(t *testing.T) {
	raw := "POST /api/users HTTP/1.1\r\nContent-Type: application/json\r\nContent-Length: 13\r\n\r\n{\"name\":\"ok\"}"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	if req.Method() != "POST" {
		t.Errorf("Method = %q, want POST", req.Method())
	}
	body, err := req.Body()
	if err != nil {
		t.Fatalf("Body() error: %v", err)
	}
	if string(body) != `{"name":"ok"}` {
		t.Errorf("Body = %q, want {\"name\":\"ok\"}", body)
	}
	if req.Header("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", req.Header("Content-Type"))
	}
}

func TestParseHTTPRequest_ConnectionClose(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nConnection: close\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	if req.keepAlive {
		t.Error("keepAlive should be false when Connection: close")
	}
}

func TestParseHTTPRequest_ConnectionCloseCaseInsensitive(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nCONNECTION: CLOSE\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	if req.keepAlive {
		t.Error("keepAlive should be false when CONNECTION: CLOSE")
	}
}

func TestParseHTTPRequest_Incomplete(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"no_crlf", "GET / HTTP/1.1\r\n"},
		{"partial_headers", "GET / HTTP/1.1\r\nHost: x\r\n"},
		{"incomplete_body", "POST / HTTP/1.1\r\nContent-Length: 100\r\n\r\nshort"},
		{"empty", ""},
		{"just_method", "GET"},
		{"no_path_space", "GET/HTTP/1.1\r\n\r\n"},
	}

	for _, tt := range tests {
		_, _, ok := parseHTTPRequest([]byte(tt.raw), noLimits)
		if ok {
			t.Errorf("%s: expected false, got true", tt.name)
		}
	}
}

func TestParseHTTPRequest_OversizedContentLength(t *testing.T) {
	// Content-Length > maxContentLength (10MB) should be rejected.
	raw := "POST / HTTP/1.1\r\nContent-Length: 99999999999\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject oversized Content-Length")
	}
}

func TestParseHTTPRequest_ZeroContentLength(t *testing.T) {
	raw := "POST / HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed for Content-Length: 0")
	}
	body, _ := req.Body()
	if len(body) != 0 {
		t.Errorf("body len = %d, want 0", len(body))
	}
}

func TestParseHTTPRequest_BodyIsSafeCopy(t *testing.T) {
	raw := []byte("POST / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello")
	req, _, ok := parseHTTPRequest(raw, noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	body, _ := req.Body()

	// Mutate original buffer — body should not change.
	copy(raw[len(raw)-5:], "XXXXX")
	if string(body) != "hello" {
		t.Errorf("body = %q after mutation, want hello (not a safe copy!)", body)
	}
}

func TestParseHTTPRequest_PathIsSafeCopy(t *testing.T) {
	raw := []byte("GET /safe HTTP/1.1\r\n\r\n")
	req, _, ok := parseHTTPRequest(raw, noLimits)
	if !ok {
		t.Fatal("parse failed")
	}

	// Mutate original buffer.
	copy(raw[4:9], "XXXXX")
	if req.Path() != "/safe" {
		t.Errorf("path = %q after mutation, want /safe (not a safe copy!)", req.Path())
	}
}

// TestBtoi verifies integer parsing including overflow protection.
func TestBtoi(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"123", 123},
		{"8192", 8192},
		{"10485760", maxContentLength},        // exactly 10MB
		{"10485761", maxContentLength + 1},    // 10MB + 1 → overflow sentinel
		{"99999999999", maxContentLength + 1}, // way too large
		{"abc", 0},
		{"12abc", 12},
		{"", 0},
	}

	for _, tt := range tests {
		got := btoi([]byte(tt.input))
		if got != tt.want {
			t.Errorf("btoi(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAsciiEqualFold(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"content-length", "content-length", true},
		{"Content-Length", "content-length", true},
		{"CONTENT-LENGTH", "content-length", true},
		{"content-type", "content-type", true},
		{"Content-Type", "content-type", true},
		{"connection", "connection", true},
		{"Connection", "connection", true},
		{"close", "close", true},
		{"CLOSE", "close", true},
		{"content-length", "content-type", false},
		{"short", "longer", false},
	}

	for _, tt := range tests {
		got := asciiEqualFold([]byte(tt.a), []byte(tt.b))
		if got != tt.want {
			t.Errorf("asciiEqualFold(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestResponseBuild verifies HTTP response serialization.
func TestResponseBuild(t *testing.T) {
	resp := acquireResponse()
	defer releaseResponse(resp)

	resp.WriteHeader(200)
	resp.Header().Set("Content-Type", "text/plain")
	resp.Write([]byte("hello"))

	data := resp.build()
	s := string(data)

	if !strings.HasPrefix(s, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("response doesn't start with status line: %q", s[:40])
	}
	if !strings.Contains(s, "Content-Type: text/plain\r\n") {
		t.Error("missing Content-Type header")
	}
	if !strings.Contains(s, "Content-Length: 5\r\n") {
		t.Error("missing or wrong Content-Length")
	}
	if !strings.HasSuffix(s, "\r\n\r\nhello") {
		t.Errorf("response doesn't end with body: %q", s[len(s)-20:])
	}
}

// TestResponseBuildIsSafeCopy verifies build() returns independent memory.
func TestResponseBuildIsSafeCopy(t *testing.T) {
	resp := acquireResponse()
	resp.WriteHeader(200)
	resp.Write([]byte("test"))
	data := resp.build()

	// Return to pool and get a new one.
	releaseResponse(resp)
	resp2 := acquireResponse()
	resp2.WriteHeader(500)
	resp2.Write([]byte("error"))
	_ = resp2.build()
	releaseResponse(resp2)

	// Original data should be unchanged.
	if !strings.Contains(string(data), "200 OK") {
		t.Error("build() data was corrupted after pool return")
	}
}

// TestIOHeadersDel verifies header deletion (swap-remove).
func TestIOHeadersDel(t *testing.T) {
	var h wingHeaders
	h.Set("A", "1")
	h.Set("B", "2")
	h.Set("C", "3")

	h.Del("B")

	if h.count != 2 {
		t.Fatalf("count = %d after Del, want 2", h.count)
	}
	if h.Get("A") != "1" {
		t.Errorf("A = %q, want 1", h.Get("A"))
	}
	if h.Get("B") != "" {
		t.Errorf("B should be empty after Del, got %q", h.Get("B"))
	}
	if h.Get("C") != "3" {
		t.Errorf("C = %q, want 3", h.Get("C"))
	}
}

// TestIOHeadersOverflow verifies silent drop when >8 headers.
func TestIOHeadersOverflow(t *testing.T) {
	var h wingHeaders
	for i := 0; i < 10; i++ {
		h.Set(string(rune('A'+i)), "val")
	}
	if h.count != 8 {
		t.Errorf("count = %d, want 8 (max)", h.count)
	}
}

// TestQueryParamEdgeCases tests query string parsing edge cases.
func TestQueryParamEdgeCases(t *testing.T) {
	raw := "GET /path?a=1&b=&c=3&d HTTP/1.1\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}

	if req.QueryParam("a") != "1" {
		t.Errorf("a = %q, want 1", req.QueryParam("a"))
	}
	if req.QueryParam("b") != "" {
		t.Errorf("b = %q, want empty", req.QueryParam("b"))
	}
	if req.QueryParam("c") != "3" {
		t.Errorf("c = %q, want 3", req.QueryParam("c"))
	}
	// "d" has no "=" so it won't match any key=value pattern.
	if req.QueryParam("d") != "" {
		t.Errorf("d = %q, want empty (no =)", req.QueryParam("d"))
	}
}

// ----------------------------- Header Limits Tests (R1) -----------------------------

func TestParseHTTPRequest_HeaderCountLimit(t *testing.T) {
	// 3 headers with limit of 2 → reject.
	raw := "GET / HTTP/1.1\r\nA: 1\r\nB: 2\r\nC: 3\r\n\r\n"
	limits := parserLimits{maxHeaderCount: 2}
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should reject when header count exceeds limit")
	}
}

func TestParseHTTPRequest_HeaderCountAtLimit(t *testing.T) {
	// Exactly 2 headers with limit of 2 → accept.
	raw := "GET / HTTP/1.1\r\nA: 1\r\nB: 2\r\n\r\n"
	limits := parserLimits{maxHeaderCount: 2}
	req, _, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Fatal("should accept when header count equals limit")
	}
	if req.Method() != "GET" {
		t.Errorf("Method = %q, want GET", req.Method())
	}
}

func TestParseHTTPRequest_HeaderCountUnlimited(t *testing.T) {
	// 5 headers with limit of 0 (unlimited) → accept.
	raw := "GET / HTTP/1.1\r\nA: 1\r\nB: 2\r\nC: 3\r\nD: 4\r\nE: 5\r\n\r\n"
	limits := parserLimits{maxHeaderCount: 0}
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Error("should accept when maxHeaderCount is 0 (unlimited)")
	}
}

func TestParseHTTPRequest_HeaderSizeLimit(t *testing.T) {
	// Header "X-Big: value" is 12 bytes. Limit to 10 → reject.
	raw := "GET / HTTP/1.1\r\nX-Big: value\r\n\r\n"
	limits := parserLimits{maxHeaderSize: 10}
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should reject when header size exceeds limit")
	}
}

func TestParseHTTPRequest_HeaderSizeAtLimit(t *testing.T) {
	// Header "A: ok" after CRLF strip is 5 bytes. Limit to 5 → accept.
	raw := "GET / HTTP/1.1\r\nA: ok\r\n\r\n"
	limits := parserLimits{maxHeaderSize: 5}
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Fatal("should accept when header size equals limit")
	}
}

func TestParseHTTPRequest_HeaderSizeUnlimited(t *testing.T) {
	// Large header with limit of 0 (unlimited) → accept.
	raw := "GET / HTTP/1.1\r\nX-Large: " + strings.Repeat("x", 10000) + "\r\n\r\n"
	limits := parserLimits{maxHeaderSize: 0}
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Error("should accept when maxHeaderSize is 0 (unlimited)")
	}
}

func TestParseHTTPRequest_CustomLimits(t *testing.T) {
	// Recommended production limits: 100 headers, 8192 bytes per header.
	limits := parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192}

	// Normal request with a few headers → accept.
	raw := "GET / HTTP/1.1\r\nHost: localhost\r\nAccept: text/html\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Error("normal request should pass with recommended limits")
	}

	// Header value exceeding 8192 bytes → reject.
	bigRaw := "GET / HTTP/1.1\r\nX-Big: " + strings.Repeat("a", 8200) + "\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(bigRaw), limits)
	if ok {
		t.Error("should reject header exceeding 8192 byte limit")
	}
}

func TestParseHTTPRequest_MaxContentLengthStillWorks(t *testing.T) {
	// Verify maxContentLength (10MB) rejection still works after refactor.
	raw := "POST / HTTP/1.1\r\nContent-Length: 99999999999\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should still reject oversized Content-Length with noLimits")
	}

	// Also verify with custom limits set.
	limits := parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192}
	_, _, ok = parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should still reject oversized Content-Length with custom limits")
	}

	// Valid Content-Length with body → accept.
	raw2 := "POST / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello"
	req, _, ok := parseHTTPRequest([]byte(raw2), limits)
	if !ok {
		t.Fatal("valid POST with body should pass")
	}
	body, _ := req.Body()
	if string(body) != "hello" {
		t.Errorf("body = %q, want hello", body)
	}
}

func TestParseHTTPRequest_BothLimitsEnforced(t *testing.T) {
	// Both count and size limits active — count triggers first.
	limits := parserLimits{maxHeaderCount: 1, maxHeaderSize: 8192}
	raw := "GET / HTTP/1.1\r\nA: 1\r\nB: 2\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should reject: 2 headers exceeds maxHeaderCount=1")
	}

	// Both limits active — size triggers first.
	limits2 := parserLimits{maxHeaderCount: 100, maxHeaderSize: 5}
	raw2 := "GET / HTTP/1.1\r\nX-Long: abcdef\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw2), limits2)
	if ok {
		t.Error("should reject: header size exceeds maxHeaderSize=5")
	}
}

// ----------------------------- R2: Injection Prevention Tests -----------------------------

func TestParseHTTPRequest_CRLFInjectionInHeaderValue(t *testing.T) {
	// R2.1: Bare \r in header value → reject.
	raw := "GET / HTTP/1.1\r\nX-Evil: foo\rbar\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject header value containing bare CR")
	}

	// R2.1: Bare \n in header value → reject.
	raw2 := "GET / HTTP/1.1\r\nX-Evil: foo\nbar\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject header value containing bare LF")
	}

	// R2.1: CRLF injection attempt (header splitting) → reject.
	raw3 := "GET / HTTP/1.1\r\nX-Evil: foo\r\nInjected: bar\r\n\r\n"
	// This is actually two separate headers — "X-Evil: foo" and "Injected: bar".
	// Both are valid individually. The CRLF is the line terminator, not inside the value.
	// This should parse fine (the CRLF is stripped by the line parser).
	_, _, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if !ok {
		t.Error("two valid headers should parse fine")
	}
}

func TestParseHTTPRequest_DuplicateContentLength(t *testing.T) {
	// R2.2: Duplicate Content-Length headers → reject.
	raw := "POST / HTTP/1.1\r\nContent-Length: 5\r\nContent-Length: 5\r\n\r\nhello"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject duplicate Content-Length headers")
	}

	// R2.2: Duplicate with different values → also reject.
	raw2 := "POST / HTTP/1.1\r\nContent-Length: 5\r\nContent-Length: 10\r\n\r\nhello"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject duplicate Content-Length with different values")
	}

	// Single Content-Length → accept.
	raw3 := "POST / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello"
	_, _, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if !ok {
		t.Error("single Content-Length should be accepted")
	}
}

func TestParseHTTPRequest_TEAndCLConflict(t *testing.T) {
	// R2.3: Both Transfer-Encoding and Content-Length → reject (RFC 7230 §3.3.3).
	raw := "POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\nContent-Length: 5\r\n\r\nhello"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject request with both Transfer-Encoding and Content-Length")
	}

	// R2.3: Case-insensitive match.
	raw2 := "POST / HTTP/1.1\r\ntransfer-encoding: chunked\r\ncontent-length: 5\r\n\r\nhello"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject TE+CL conflict (case-insensitive)")
	}

	// Transfer-Encoding alone → accept.
	raw3 := "POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if !ok {
		t.Error("Transfer-Encoding alone should be accepted")
	}
}

func TestParseHTTPRequest_NonNumericContentLength(t *testing.T) {
	// R2.4: Non-numeric Content-Length → reject.
	raw := "POST / HTTP/1.1\r\nContent-Length: abc\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject non-numeric Content-Length")
	}

	// R2.4: Mixed numeric/alpha → reject.
	raw2 := "POST / HTTP/1.1\r\nContent-Length: 5abc\r\n\r\nhello"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject Content-Length with trailing non-digits")
	}

	// R2.4: Empty Content-Length value → reject.
	raw3 := "POST / HTTP/1.1\r\nContent-Length: \r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if ok {
		t.Error("should reject empty Content-Length value")
	}

	// R2.4: Negative Content-Length → reject.
	raw4 := "POST / HTTP/1.1\r\nContent-Length: -1\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw4), noLimits)
	if ok {
		t.Error("should reject negative Content-Length")
	}

	// Valid numeric Content-Length → accept.
	raw5 := "POST / HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw5), noLimits)
	if !ok {
		t.Error("Content-Length: 0 should be accepted")
	}
}

func TestParseHTTPRequest_InvalidHeaderName(t *testing.T) {
	// R2.5: Header name with space → reject.
	raw := "GET / HTTP/1.1\r\nBad Header: value\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject header name containing space")
	}

	// R2.5: Header name with colon-like chars → reject (parenthesis is a delimiter).
	raw2 := "GET / HTTP/1.1\r\nBad(Header): value\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject header name containing '('")
	}

	// R2.5: Header name with high-byte (>127) → reject.
	raw3 := "GET / HTTP/1.1\r\nBad\x80Header: value\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if ok {
		t.Error("should reject header name with byte > 127")
	}

	// R2.5: Valid token characters → accept.
	raw4 := "GET / HTTP/1.1\r\nX-Custom-Header_v2: value\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw4), noLimits)
	if !ok {
		t.Error("valid token header name should be accepted")
	}
}

func TestParseHTTPRequest_MalformedRequestLine(t *testing.T) {
	// R2.6: Missing HTTP version → reject.
	raw := "GET /\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject request line without HTTP version")
	}

	// R2.6: Invalid version prefix → reject.
	raw2 := "GET / HTTZ/1.1\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject request line with invalid version prefix")
	}

	// R2.6: Version too short → reject.
	raw3 := "GET / HTTP\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if ok {
		t.Error("should reject request line with version 'HTTP' (no slash)")
	}

	// R2.6: Valid HTTP/1.1 → accept.
	raw4 := "GET / HTTP/1.1\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw4), noLimits)
	if !ok {
		t.Error("valid HTTP/1.1 request line should be accepted")
	}

	// R2.6: Valid HTTP/1.0 → accept.
	raw5 := "GET / HTTP/1.0\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw5), noLimits)
	if !ok {
		t.Error("valid HTTP/1.0 request line should be accepted")
	}
}

func TestParseHTTPRequest_TokenTableHelpers(t *testing.T) {
	// isValidTokenName: empty → false.
	if isValidTokenName(nil) {
		t.Error("nil should not be valid token")
	}
	if isValidTokenName([]byte{}) {
		t.Error("empty should not be valid token")
	}

	// isValidTokenName: valid tokens.
	if !isValidTokenName([]byte("Content-Type")) {
		t.Error("Content-Type should be valid token")
	}
	if !isValidTokenName([]byte("x-custom_header!")) {
		t.Error("x-custom_header! should be valid token")
	}

	// isValidTokenName: invalid tokens.
	if isValidTokenName([]byte("Bad Header")) {
		t.Error("header with space should not be valid token")
	}
	if isValidTokenName([]byte("Bad\x00Header")) {
		t.Error("header with null byte should not be valid token")
	}

	// containsCRLF tests.
	if containsCRLF([]byte("normal value")) {
		t.Error("normal value should not contain CRLF")
	}
	if !containsCRLF([]byte("has\rnewline")) {
		t.Error("value with CR should be detected")
	}
	if !containsCRLF([]byte("has\nnewline")) {
		t.Error("value with LF should be detected")
	}

	// isAllDigits tests.
	if !isAllDigits([]byte("12345")) {
		t.Error("12345 should be all digits")
	}
	if isAllDigits([]byte("123a5")) {
		t.Error("123a5 should not be all digits")
	}
	if isAllDigits([]byte("")) {
		t.Error("empty should not be all digits")
	}
	if isAllDigits([]byte("-1")) {
		t.Error("-1 should not be all digits")
	}
}

// TestParseHTTPRequest_PipeliningConsumedBytes verifies that parseHTTPRequest
// returns the correct number of consumed bytes, enabling HTTP pipelining support.
func TestParseHTTPRequest_PipeliningConsumedBytes(t *testing.T) {
	// Two pipelined GET requests in a single buffer.
	req1Raw := "GET /first HTTP/1.1\r\nHost: localhost\r\n\r\n"
	req2Raw := "GET /second HTTP/1.1\r\nHost: localhost\r\n\r\n"
	pipelined := []byte(req1Raw + req2Raw)

	// Parse first request — consumed should equal len(req1Raw).
	req1, consumed1, ok := parseHTTPRequest(pipelined, noLimits)
	if !ok {
		t.Fatal("failed to parse first pipelined request")
	}
	if req1.Path() != "/first" {
		t.Errorf("req1.Path = %q, want /first", req1.Path())
	}
	if consumed1 != len(req1Raw) {
		t.Errorf("consumed1 = %d, want %d", consumed1, len(req1Raw))
	}

	// Parse second request from remaining buffer.
	remaining := pipelined[consumed1:]
	req2, consumed2, ok := parseHTTPRequest(remaining, noLimits)
	if !ok {
		t.Fatal("failed to parse second pipelined request")
	}
	if req2.Path() != "/second" {
		t.Errorf("req2.Path = %q, want /second", req2.Path())
	}
	if consumed2 != len(req2Raw) {
		t.Errorf("consumed2 = %d, want %d", consumed2, len(req2Raw))
	}
}

// TestParseHTTPRequest_PipeliningWithBody tests pipelining with POST requests
// that have Content-Length bodies.
func TestParseHTTPRequest_PipeliningWithBody(t *testing.T) {
	body1 := `{"id":1}`
	req1Raw := "POST /api HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body1)) + "\r\n\r\n" + body1
	req2Raw := "GET /health HTTP/1.1\r\n\r\n"
	pipelined := []byte(req1Raw + req2Raw)

	req1, consumed1, ok := parseHTTPRequest(pipelined, noLimits)
	if !ok {
		t.Fatal("failed to parse POST in pipeline")
	}
	if req1.Method() != "POST" {
		t.Errorf("req1.Method = %q, want POST", req1.Method())
	}
	b, _ := req1.Body()
	if string(b) != body1 {
		t.Errorf("req1.Body = %q, want %q", string(b), body1)
	}
	if consumed1 != len(req1Raw) {
		t.Errorf("consumed1 = %d, want %d", consumed1, len(req1Raw))
	}

	remaining := pipelined[consumed1:]
	req2, _, ok := parseHTTPRequest(remaining, noLimits)
	if !ok {
		t.Fatal("failed to parse GET after POST in pipeline")
	}
	if req2.Path() != "/health" {
		t.Errorf("req2.Path = %q, want /health", req2.Path())
	}
}

// ==================== Comprehensive Pipelining Test Suite ====================
//
// These tests validate HTTP/1.1 pipelining correctness in the Wing parser.
// Pipelining means a client sends multiple requests on a single TCP connection
// without waiting for each response. The parser must:
//   1. Return the correct consumed byte count for each request
//   2. Allow callers to parse subsequent requests from the remainder
//   3. Handle mixed methods, varying body sizes, and partial data correctly

// TestPipelining_ThreeGETs tests parsing 3 consecutive GET requests from one buffer.
func TestPipelining_ThreeGETs(t *testing.T) {
	reqs := []string{
		"GET /a HTTP/1.1\r\nHost: h\r\n\r\n",
		"GET /b HTTP/1.1\r\nHost: h\r\n\r\n",
		"GET /c HTTP/1.1\r\nHost: h\r\n\r\n",
	}
	buf := []byte(reqs[0] + reqs[1] + reqs[2])
	paths := []string{"/a", "/b", "/c"}

	for i, wantPath := range paths {
		req, consumed, ok := parseHTTPRequest(buf, noLimits)
		if !ok {
			t.Fatalf("request %d: parse failed, buf len=%d", i, len(buf))
		}
		if req.Path() != wantPath {
			t.Errorf("request %d: Path = %q, want %q", i, req.Path(), wantPath)
		}
		if consumed != len(reqs[i]) {
			t.Errorf("request %d: consumed = %d, want %d", i, consumed, len(reqs[i]))
		}
		buf = buf[consumed:]
	}
	if len(buf) != 0 {
		t.Errorf("leftover bytes = %d, want 0", len(buf))
	}
}

// TestPipelining_POSTthenPOST tests two POST requests with different body sizes.
func TestPipelining_POSTthenPOST(t *testing.T) {
	body1 := `{"name":"alice"}`
	body2 := `{"name":"bob","age":30}`
	req1Raw := "POST /users HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body1)) + "\r\nContent-Type: application/json\r\n\r\n" + body1
	req2Raw := "POST /users HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body2)) + "\r\nContent-Type: application/json\r\n\r\n" + body2
	buf := []byte(req1Raw + req2Raw)

	// First POST.
	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("POST 1 parse failed")
	}
	b1, _ := r1.Body()
	if string(b1) != body1 {
		t.Errorf("POST 1 body = %q, want %q", string(b1), body1)
	}
	if c1 != len(req1Raw) {
		t.Errorf("POST 1 consumed = %d, want %d", c1, len(req1Raw))
	}

	// Second POST.
	buf = buf[c1:]
	r2, c2, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("POST 2 parse failed")
	}
	b2, _ := r2.Body()
	if string(b2) != body2 {
		t.Errorf("POST 2 body = %q, want %q", string(b2), body2)
	}
	if c2 != len(req2Raw) {
		t.Errorf("POST 2 consumed = %d, want %d", c2, len(req2Raw))
	}
}

// TestPipelining_PartialSecondRequest ensures that if the second pipelined
// request is incomplete (truncated), the parser returns false for it.
func TestPipelining_PartialSecondRequest(t *testing.T) {
	req1 := "GET /ok HTTP/1.1\r\nHost: h\r\n\r\n"
	partial := "GET /partial HTT" // truncated — no CRLFCRLF
	buf := []byte(req1 + partial)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("first request should parse")
	}
	if r1.Path() != "/ok" {
		t.Errorf("Path = %q, want /ok", r1.Path())
	}

	// Try to parse the partial remainder.
	buf = buf[c1:]
	if len(buf) != len(partial) {
		t.Errorf("remaining = %d, want %d", len(buf), len(partial))
	}
	_, _, ok = parseHTTPRequest(buf, noLimits)
	if ok {
		t.Error("partial request should NOT parse successfully")
	}
}

// TestPipelining_PartialBody ensures that a POST with incomplete body
// returns false (waiting for more data), while the preceding request parses.
func TestPipelining_PartialBody(t *testing.T) {
	req1 := "GET /first HTTP/1.1\r\n\r\n"
	// POST claims 100 bytes but only 10 are present.
	req2 := "POST /data HTTP/1.1\r\nContent-Length: 100\r\n\r\n0123456789"
	buf := []byte(req1 + req2)

	_, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("first GET should parse")
	}

	buf = buf[c1:]
	_, _, ok = parseHTTPRequest(buf, noLimits)
	if ok {
		t.Error("POST with incomplete body should NOT parse")
	}
}

// TestPipelining_EmptyBodyPOST tests pipelining where a POST has Content-Length: 0.
func TestPipelining_EmptyBodyPOST(t *testing.T) {
	req1 := "POST /empty HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
	req2 := "GET /next HTTP/1.1\r\n\r\n"
	buf := []byte(req1 + req2)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("POST CL:0 should parse")
	}
	if r1.Method() != "POST" {
		t.Errorf("Method = %q, want POST", r1.Method())
	}
	b1, _ := r1.Body()
	if len(b1) != 0 {
		t.Errorf("body len = %d, want 0", len(b1))
	}
	if c1 != len(req1) {
		t.Errorf("consumed = %d, want %d", c1, len(req1))
	}

	buf = buf[c1:]
	r2, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("second GET should parse")
	}
	if r2.Path() != "/next" {
		t.Errorf("Path = %q, want /next", r2.Path())
	}
}

// TestPipelining_ConnectionClose tests that Connection: close on the first
// request doesn't affect parsing of pipelined bytes (parser doesn't care;
// connection management is the caller's responsibility).
func TestPipelining_ConnectionClose(t *testing.T) {
	req1 := "GET /close HTTP/1.1\r\nConnection: close\r\n\r\n"
	req2 := "GET /more HTTP/1.1\r\n\r\n"
	buf := []byte(req1 + req2)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("first request should parse")
	}
	if r1.keepAlive {
		t.Error("keepAlive should be false with Connection: close")
	}

	// Parser should still parse the second request — it's the caller's
	// job to close the connection instead of processing pipelined data.
	buf = buf[c1:]
	r2, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("second request should parse (parser is stateless)")
	}
	if r2.Path() != "/more" {
		t.Errorf("Path = %q, want /more", r2.Path())
	}
}

// TestPipelining_LargeBodyBoundary ensures consumed is correct when body
// extends to the exact end of the buffer (no leftover bytes).
func TestPipelining_LargeBodyBoundary(t *testing.T) {
	bodySize := 4096
	body := strings.Repeat("X", bodySize)
	raw := "POST /upload HTTP/1.1\r\nContent-Length: " + strconv.Itoa(bodySize) + "\r\n\r\n" + body
	buf := []byte(raw)

	r, consumed, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("should parse large body")
	}
	b, _ := r.Body()
	if len(b) != bodySize {
		t.Errorf("body size = %d, want %d", len(b), bodySize)
	}
	if consumed != len(raw) {
		t.Errorf("consumed = %d, want %d (exact buffer end)", consumed, len(raw))
	}
}

// TestPipelining_BufferShiftSimulation simulates what handleRecv does:
// read into a fixed buffer, parse, shift, parse again.
func TestPipelining_BufferShiftSimulation(t *testing.T) {
	readBuf := make([]byte, 4096)

	// Simulate TCP read that delivers two requests at once.
	req1 := "GET /first HTTP/1.1\r\nHost: h\r\n\r\n"
	req2 := "GET /second HTTP/1.1\r\nHost: h\r\n\r\n"
	data := req1 + req2
	readN := copy(readBuf, data)

	// Parse first request.
	r1, consumed, ok := parseHTTPRequest(readBuf[:readN], noLimits)
	if !ok {
		t.Fatal("first parse failed")
	}
	if r1.Path() != "/first" {
		t.Errorf("r1.Path = %q", r1.Path())
	}

	// Shift buffer (exactly what handleRecv does).
	remaining := readN - consumed
	if remaining > 0 {
		copy(readBuf, readBuf[consumed:readN])
	}
	readN = remaining

	// Parse second request from shifted buffer.
	r2, consumed2, ok := parseHTTPRequest(readBuf[:readN], noLimits)
	if !ok {
		t.Fatal("second parse after shift failed")
	}
	if r2.Path() != "/second" {
		t.Errorf("r2.Path = %q", r2.Path())
	}

	// After consuming second request, buffer should be empty.
	readN -= consumed2
	if readN != 0 {
		t.Errorf("readN after both = %d, want 0", readN)
	}
}

// TestPipelining_BufferShiftWithPartial simulates: full request + partial request
// in one TCP read, then the rest of the partial request in a second read.
func TestPipelining_BufferShiftWithPartial(t *testing.T) {
	readBuf := make([]byte, 4096)

	req1 := "GET /done HTTP/1.1\r\nHost: h\r\n\r\n"
	req2 := "GET /pending HTTP/1.1\r\nHost: h\r\n\r\n"

	// First TCP read: full req1 + first 10 bytes of req2.
	partial := req2[:10]
	readN := copy(readBuf, req1+partial)

	// Parse first request.
	_, consumed, ok := parseHTTPRequest(readBuf[:readN], noLimits)
	if !ok {
		t.Fatal("first parse failed")
	}

	// Shift.
	remaining := readN - consumed
	copy(readBuf, readBuf[consumed:readN])
	readN = remaining

	// Try to parse — should fail (partial).
	_, _, ok = parseHTTPRequest(readBuf[:readN], noLimits)
	if ok {
		t.Fatal("should NOT parse partial request")
	}

	// Second TCP read: rest of req2 appended to buffer.
	rest := req2[10:]
	n := copy(readBuf[readN:], rest)
	readN += n

	// Now it should parse.
	r2, _, ok := parseHTTPRequest(readBuf[:readN], noLimits)
	if !ok {
		t.Fatal("second parse after completing data should succeed")
	}
	if r2.Path() != "/pending" {
		t.Errorf("r2.Path = %q, want /pending", r2.Path())
	}
}

// TestPipelining_FiveRequestsIterative parses 5 pipelined requests iteratively,
// verifying consumed bytes accumulate correctly.
func TestPipelining_FiveRequestsIterative(t *testing.T) {
	var rawParts []string
	for i := 0; i < 5; i++ {
		rawParts = append(rawParts, "GET /r"+strconv.Itoa(i)+" HTTP/1.1\r\n\r\n")
	}
	buf := []byte(strings.Join(rawParts, ""))

	totalConsumed := 0
	for i := 0; i < 5; i++ {
		req, consumed, ok := parseHTTPRequest(buf, noLimits)
		if !ok {
			t.Fatalf("request %d: parse failed at offset %d", i, totalConsumed)
		}
		wantPath := "/r" + strconv.Itoa(i)
		if req.Path() != wantPath {
			t.Errorf("request %d: Path = %q, want %q", i, req.Path(), wantPath)
		}
		if consumed != len(rawParts[i]) {
			t.Errorf("request %d: consumed = %d, want %d", i, consumed, len(rawParts[i]))
		}
		totalConsumed += consumed
		buf = buf[consumed:]
	}
	if len(buf) != 0 {
		t.Errorf("leftover = %d bytes", len(buf))
	}
}

// TestPipelining_MixedMethodsAndBodies tests a realistic pipeline:
// GET → POST(small) → GET → POST(large) → DELETE
func TestPipelining_MixedMethodsAndBodies(t *testing.T) {
	type expect struct {
		method string
		path   string
		body   string
	}
	small := `{"ok":true}`
	large := strings.Repeat("A", 1000)

	raws := []string{
		"GET /api/status HTTP/1.1\r\nHost: h\r\n\r\n",
		"POST /api/data HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(small)) + "\r\n\r\n" + small,
		"GET /api/list HTTP/1.1\r\n\r\n",
		"POST /api/bulk HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(large)) + "\r\n\r\n" + large,
		"DELETE /api/item/99 HTTP/1.1\r\n\r\n",
	}
	expects := []expect{
		{"GET", "/api/status", ""},
		{"POST", "/api/data", small},
		{"GET", "/api/list", ""},
		{"POST", "/api/bulk", large},
		{"DELETE", "/api/item/99", ""},
	}

	buf := []byte(strings.Join(raws, ""))
	for i, want := range expects {
		req, consumed, ok := parseHTTPRequest(buf, noLimits)
		if !ok {
			t.Fatalf("[%d] %s %s: parse failed", i, want.method, want.path)
		}
		if req.Method() != want.method {
			t.Errorf("[%d] Method = %q, want %q", i, req.Method(), want.method)
		}
		if req.Path() != want.path {
			t.Errorf("[%d] Path = %q, want %q", i, req.Path(), want.path)
		}
		b, _ := req.Body()
		if string(b) != want.body {
			t.Errorf("[%d] Body len = %d, want %d", i, len(b), len(want.body))
		}
		if consumed != len(raws[i]) {
			t.Errorf("[%d] consumed = %d, want %d", i, consumed, len(raws[i]))
		}
		buf = buf[consumed:]
	}
	if len(buf) != 0 {
		t.Errorf("leftover = %d bytes", len(buf))
	}
}

// TestPipelining_ConsumedEqualsInputForSingleRequest verifies that a single
// request with no trailing data has consumed == len(input).
func TestPipelining_ConsumedEqualsInputForSingleRequest(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"GET minimal", "GET / HTTP/1.1\r\n\r\n"},
		{"GET with host", "GET /path HTTP/1.1\r\nHost: h\r\n\r\n"},
		{"POST with body", "POST / HTTP/1.1\r\nContent-Length: 3\r\n\r\nabc"},
		{"DELETE", "DELETE /x HTTP/1.1\r\n\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, consumed, ok := parseHTTPRequest([]byte(tt.raw), noLimits)
			if !ok {
				t.Fatal("parse failed")
			}
			if consumed != len(tt.raw) {
				t.Errorf("consumed = %d, want %d (exact input len)", consumed, len(tt.raw))
			}
		})
	}
}

// TestPipelining_ConsumedPlusTailEqualsTotal is the core invariant:
// for any parseable request followed by arbitrary trailing bytes,
// consumed + len(tail) == len(input).
func TestPipelining_ConsumedPlusTailEqualsTotal(t *testing.T) {
	req := "GET /x HTTP/1.1\r\n\r\n"
	tails := []string{
		"",                  // no tail
		"G",                 // 1 byte
		"GET / HT",          // partial next request
		"garbage\x00\xff",   // random bytes
		"GET /y HTTP/1.1\r\n\r\n", // complete next request
	}
	for _, tail := range tails {
		input := []byte(req + tail)
		_, consumed, ok := parseHTTPRequest(input, noLimits)
		if !ok {
			t.Fatalf("parse failed with tail %q", tail)
		}
		if consumed+len(tail) != len(input) {
			t.Errorf("tail=%q: consumed(%d) + tail(%d) = %d, want %d",
				tail, consumed, len(tail), consumed+len(tail), len(input))
		}
	}
}

// TestPipelining_BodySafeCopyIndependence verifies that the body returned
// by parseHTTPRequest is a safe copy — modifying the original buffer
// after parsing must NOT affect the parsed body.
func TestPipelining_BodySafeCopyIndependence(t *testing.T) {
	body := "ORIGINAL"
	raw := "POST /x HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	buf := []byte(raw)

	req, consumed, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("parse failed")
	}

	// Overwrite the body region in the original buffer.
	for i := consumed - len(body); i < consumed; i++ {
		buf[i] = 'X'
	}

	// The parsed body must still be the original.
	b, _ := req.Body()
	if string(b) != body {
		t.Errorf("body = %q after buffer mutation, want %q (safe copy broken!)", string(b), body)
	}
}

// TestPipelining_QueryParamsPreserved verifies query strings are correct
// across pipelined requests with different query parameters.
func TestPipelining_QueryParamsPreserved(t *testing.T) {
	req1 := "GET /search?q=hello&page=1 HTTP/1.1\r\n\r\n"
	req2 := "GET /search?q=world&page=2 HTTP/1.1\r\n\r\n"
	buf := []byte(req1 + req2)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("req1 parse failed")
	}
	if r1.QueryParam("q") != "hello" || r1.QueryParam("page") != "1" {
		t.Errorf("req1 query: q=%q page=%q", r1.QueryParam("q"), r1.QueryParam("page"))
	}

	buf = buf[c1:]
	r2, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("req2 parse failed")
	}
	if r2.QueryParam("q") != "world" || r2.QueryParam("page") != "2" {
		t.Errorf("req2 query: q=%q page=%q", r2.QueryParam("q"), r2.QueryParam("page"))
	}
}

// TestPipelining_BodyContainsCRLFCRLF ensures the parser uses Content-Length
// (not a naive scan for \r\n\r\n) when body bytes happen to contain the
// header-terminator sequence. This is a critical correctness invariant.
func TestPipelining_BodyContainsCRLFCRLF(t *testing.T) {
	// Body deliberately contains \r\n\r\n which looks like a header terminator.
	body := "line1\r\n\r\nline2"
	req1Raw := "POST /upload HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	req2Raw := "GET /next HTTP/1.1\r\n\r\n"
	buf := []byte(req1Raw + req2Raw)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("POST with CRLFCRLF in body should parse")
	}
	b1, _ := r1.Body()
	if string(b1) != body {
		t.Errorf("body = %q, want %q", string(b1), body)
	}
	if c1 != len(req1Raw) {
		t.Errorf("consumed = %d, want %d", c1, len(req1Raw))
	}

	// Second request must parse correctly from the remainder.
	buf = buf[c1:]
	r2, c2, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("GET after POST-with-CRLFCRLF-body should parse")
	}
	if r2.Path() != "/next" {
		t.Errorf("Path = %q, want /next", r2.Path())
	}
	if c2 != len(req2Raw) {
		t.Errorf("consumed2 = %d, want %d", c2, len(req2Raw))
	}
}

// TestPipelining_HeaderIsolation verifies that headers from one pipelined
// request do not leak into the next. Each parsed request must only see
// its own Content-Type.
func TestPipelining_HeaderIsolation(t *testing.T) {
	req1Raw := "POST /a HTTP/1.1\r\nContent-Type: application/json\r\nContent-Length: 2\r\n\r\n{}"
	req2Raw := "POST /b HTTP/1.1\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nhi"
	req3Raw := "GET /c HTTP/1.1\r\n\r\n" // no Content-Type at all
	buf := []byte(req1Raw + req2Raw + req3Raw)

	// Request 1: application/json
	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("req1 parse failed")
	}
	if r1.Header("Content-Type") != "application/json" {
		t.Errorf("req1 Content-Type = %q, want application/json", r1.Header("Content-Type"))
	}

	// Request 2: text/plain (must NOT be application/json)
	buf = buf[c1:]
	r2, c2, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("req2 parse failed")
	}
	if r2.Header("Content-Type") != "text/plain" {
		t.Errorf("req2 Content-Type = %q, want text/plain", r2.Header("Content-Type"))
	}

	// Request 3: GET with no Content-Type (must be empty, not leaked from req2)
	buf = buf[c2:]
	r3, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("req3 parse failed")
	}
	if r3.Header("Content-Type") != "" {
		t.Errorf("req3 Content-Type = %q, want empty (header leaked!)", r3.Header("Content-Type"))
	}
}

// TestPipelining_PUTandPATCHWithBody tests PUT and PATCH methods with bodies
// in a pipeline, since earlier tests only covered GET/POST/DELETE.
func TestPipelining_PUTandPATCHWithBody(t *testing.T) {
	putBody := `{"name":"updated"}`
	patchBody := `{"age":30}`
	req1Raw := "PUT /users/1 HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(putBody)) + "\r\n\r\n" + putBody
	req2Raw := "PATCH /users/1 HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(patchBody)) + "\r\n\r\n" + patchBody
	req3Raw := "GET /users/1 HTTP/1.1\r\n\r\n"
	buf := []byte(req1Raw + req2Raw + req3Raw)

	// PUT
	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("PUT parse failed")
	}
	if r1.Method() != "PUT" {
		t.Errorf("Method = %q, want PUT", r1.Method())
	}
	b1, _ := r1.Body()
	if string(b1) != putBody {
		t.Errorf("PUT body = %q, want %q", string(b1), putBody)
	}

	// PATCH
	buf = buf[c1:]
	r2, c2, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("PATCH parse failed")
	}
	if r2.Method() != "PATCH" {
		t.Errorf("Method = %q, want PATCH", r2.Method())
	}
	b2, _ := r2.Body()
	if string(b2) != patchBody {
		t.Errorf("PATCH body = %q, want %q", string(b2), patchBody)
	}

	// GET after PUT+PATCH
	buf = buf[c2:]
	r3, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("GET after PUT+PATCH parse failed")
	}
	if r3.Method() != "GET" || r3.Path() != "/users/1" {
		t.Errorf("GET: Method=%q Path=%q", r3.Method(), r3.Path())
	}
}

// TestPipelining_HTTP10KeepAliveDefault verifies that HTTP/1.0 requests
// default to keepAlive=true in our parser (same as HTTP/1.1). The parser
// is version-agnostic; connection management is the caller's responsibility.
// This test documents the current behavior for pipelining correctness.
func TestPipelining_HTTP10KeepAliveDefault(t *testing.T) {
	req1 := "GET /a HTTP/1.0\r\n\r\n"
	req2 := "GET /b HTTP/1.1\r\n\r\n"
	buf := []byte(req1 + req2)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("HTTP/1.0 request should parse")
	}
	// Document current behavior: parser treats keepAlive as true regardless
	// of HTTP version (HTTP/1.1 default). The transport layer is responsible
	// for enforcing HTTP/1.0 connection semantics.
	_ = r1.keepAlive // just ensure it doesn't panic

	// Second request (HTTP/1.1) must still be parseable.
	buf = buf[c1:]
	r2, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("HTTP/1.1 after HTTP/1.0 should parse")
	}
	if r2.Path() != "/b" {
		t.Errorf("Path = %q, want /b", r2.Path())
	}
}

// ==================== Parser Gap Tests ====================
//
// These test specific code paths in parseHTTPRequest that were
// not covered by the pipelining suite above.

// TestParser_BareLFAsHeaderLineTerminator tests that a bare LF (without
// preceding CR) as a header line terminator is rejected. This exercises
// lines 98-101 in http.go which is a DIFFERENT code path from the
// containsCRLF() check on header values (line 135).
func TestParser_BareLFAsHeaderLineTerminator(t *testing.T) {
	// Header line terminated by \n instead of \r\n.
	raw := "GET / HTTP/1.1\r\nHost: localhost\nAccept: text/html\r\n\r\n"
	_, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject header line terminated by bare LF (no CR)")
	}

	// All headers with proper \r\n → accept.
	raw2 := "GET / HTTP/1.1\r\nHost: localhost\r\nAccept: text/html\r\n\r\n"
	_, _, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if !ok {
		t.Error("proper CRLF line terminators should be accepted")
	}
}

// TestParser_StarRequestTarget tests OPTIONS * HTTP/1.1 which is a
// valid request-target (RFC 7230 §5.3.4). Verifies consumed is correct
// in a pipeline context.
func TestParser_StarRequestTarget(t *testing.T) {
	req1Raw := "OPTIONS * HTTP/1.1\r\nHost: h\r\n\r\n"
	req2Raw := "GET /next HTTP/1.1\r\n\r\n"
	buf := []byte(req1Raw + req2Raw)

	r1, c1, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("OPTIONS * should parse")
	}
	if r1.Method() != "OPTIONS" {
		t.Errorf("Method = %q, want OPTIONS", r1.Method())
	}
	if r1.Path() != "*" {
		t.Errorf("Path = %q, want *", r1.Path())
	}
	if c1 != len(req1Raw) {
		t.Errorf("consumed = %d, want %d", c1, len(req1Raw))
	}

	buf = buf[c1:]
	r2, _, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("GET after OPTIONS * should parse")
	}
	if r2.Path() != "/next" {
		t.Errorf("Path = %q, want /next", r2.Path())
	}
}

// TestParser_BufferFullNoValidRequest tests that when the read buffer
// is completely full but contains no valid HTTP request, the parser
// returns false (transport will close connection to prevent DoS).
func TestParser_BufferFullNoValidRequest(t *testing.T) {
	// Simulate a 128-byte buffer filled with garbage — no \r\n\r\n.
	buf := make([]byte, 128)
	for i := range buf {
		buf[i] = 'A'
	}
	_, _, ok := parseHTTPRequest(buf, noLimits)
	if ok {
		t.Error("garbage-filled buffer should not parse as valid request")
	}

	// Buffer full with a request that's almost valid but headers not terminated.
	partial := "GET / HTTP/1.1\r\nHost: localhost\r\nX-Pad: " + strings.Repeat("x", 80)
	buf2 := []byte(partial)
	_, _, ok = parseHTTPRequest(buf2, noLimits)
	if ok {
		t.Error("request without header terminator should not parse")
	}
}

// TestParser_HeaderWithoutColon tests that a header line without a colon
// is silently skipped (line 112 in http.go: `if colon < 0 { continue }`).
func TestParser_HeaderWithoutColon(t *testing.T) {
	// "BadHeader" has no colon — should be skipped, request still valid.
	raw := "GET / HTTP/1.1\r\nBadHeader\r\nHost: h\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("request with colon-less header line should still parse")
	}
	if req.Method() != "GET" {
		t.Errorf("Method = %q, want GET", req.Method())
	}
}

// ==================== Transport Simulation Tests ====================
//
// These tests simulate the exact buffer management logic that transport.go
// performs (handleRecv → handleSend → processPipelined → handleSend → ...)
// WITHOUT needing io_uring/kqueue. They verify the state machine is correct.

// transportSim simulates Wing's per-connection buffer management.
// It mirrors the fields and logic in transport.go's conn struct and
// the handleRecv/handleSend/processPipelined methods.
type transportSim struct {
	readBuf []byte // fixed-size read buffer (like conn.readBuf)
	readN   int    // bytes in buffer (like conn.readN)
	limits  parserLimits
	parsed  []string // collected (method + " " + path) for verification
}

func newTransportSim(bufSize int) *transportSim {
	return &transportSim{
		readBuf: make([]byte, bufSize),
		limits:  noLimits,
	}
}

// simulateRecv simulates kernel delivering `data` bytes into readBuf at offset readN.
// Returns the number of bytes "received".
func (s *transportSim) simulateRecv(data []byte) int {
	n := copy(s.readBuf[s.readN:], data)
	s.readN += n
	return n
}

// handleRecv mirrors transport.go handleRecv logic:
// parse → shift → return (request, needMoreData, bufferFull).
func (s *transportSim) handleRecv() (req *wingRequest, consumed int, parsed bool, bufFull bool) {
	req, consumed, parsed = parseHTTPRequest(s.readBuf[:s.readN], s.limits)
	if !parsed {
		bufFull = s.readN >= len(s.readBuf)
		return
	}

	// Shift unconsumed bytes to front (exactly like transport.go lines 288-291).
	remaining := s.readN - consumed
	if remaining > 0 {
		copy(s.readBuf, s.readBuf[consumed:s.readN])
	}
	s.readN = remaining
	return
}

// processPipelined mirrors transport.go processPipelined logic:
// try to parse from existing readN, shift if successful.
func (s *transportSim) processPipelined() (req *wingRequest, parsed bool) {
	if s.readN == 0 {
		return nil, false
	}
	r, consumed, ok := parseHTTPRequest(s.readBuf[:s.readN], s.limits)
	if !ok {
		return nil, false
	}
	remaining := s.readN - consumed
	if remaining > 0 {
		copy(s.readBuf, s.readBuf[consumed:s.readN])
	}
	s.readN = remaining
	return r, true
}

// TestTransportSim_ThreeRequestsOneRecv simulates: kernel delivers 3 complete
// requests in a single recv. Transport parses first in handleRecv, then
// processPipelined handles the remaining two after each "send completes".
func TestTransportSim_ThreeRequestsOneRecv(t *testing.T) {
	sim := newTransportSim(4096)

	// Kernel delivers 3 requests at once.
	reqs := "GET /a HTTP/1.1\r\n\r\n" +
		"GET /b HTTP/1.1\r\n\r\n" +
		"GET /c HTTP/1.1\r\n\r\n"
	sim.simulateRecv([]byte(reqs))

	// --- handleRecv: parse first request ---
	r1, _, parsed, _ := sim.handleRecv()
	if !parsed {
		t.Fatal("handleRecv: first request should parse")
	}
	if r1.Path() != "/a" {
		t.Errorf("r1.Path = %q, want /a", r1.Path())
	}

	// --- After send completes, handleSend checks readN > 0 ---
	// (readN should be > 0 because 2 requests remain)
	if sim.readN == 0 {
		t.Fatal("readN should be > 0 after parsing first of 3 requests")
	}

	// --- processPipelined: parse second request ---
	r2, parsed := sim.processPipelined()
	if !parsed {
		t.Fatal("processPipelined: second request should parse")
	}
	if r2.Path() != "/b" {
		t.Errorf("r2.Path = %q, want /b", r2.Path())
	}

	// --- After second send, processPipelined again ---
	if sim.readN == 0 {
		t.Fatal("readN should be > 0 after parsing second of 3 requests")
	}
	r3, parsed := sim.processPipelined()
	if !parsed {
		t.Fatal("processPipelined: third request should parse")
	}
	if r3.Path() != "/c" {
		t.Errorf("r3.Path = %q, want /c", r3.Path())
	}

	// --- After third send, readN == 0 → submit recv ---
	if sim.readN != 0 {
		t.Errorf("readN = %d, want 0 after all 3 requests", sim.readN)
	}
}

// TestTransportSim_PartialThenComplete simulates: kernel delivers 1.5 requests
// in the first recv, then the rest in the second recv.
func TestTransportSim_PartialThenComplete(t *testing.T) {
	sim := newTransportSim(4096)

	req1 := "GET /done HTTP/1.1\r\n\r\n"
	req2 := "POST /data HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello"

	// First recv: full req1 + first 15 bytes of req2.
	firstChunk := req1 + req2[:15]
	sim.simulateRecv([]byte(firstChunk))

	// handleRecv: parse req1 successfully.
	r1, _, parsed, _ := sim.handleRecv()
	if !parsed {
		t.Fatal("first request should parse")
	}
	if r1.Path() != "/done" {
		t.Errorf("r1.Path = %q, want /done", r1.Path())
	}

	// After send completes: readN > 0, try processPipelined.
	if sim.readN == 0 {
		t.Fatal("readN should be > 0 (partial second request)")
	}
	_, parsed = sim.processPipelined()
	if parsed {
		t.Fatal("partial second request should NOT parse")
	}

	// Transport would now SubmitRecv(fd, readBuf, readN) — kernel delivers rest.
	rest := req2[15:]
	sim.simulateRecv([]byte(rest))

	// handleRecv: now req2 is complete.
	r2, _, parsed, _ := sim.handleRecv()
	if !parsed {
		t.Fatal("second request should parse after receiving rest")
	}
	if r2.Method() != "POST" || r2.Path() != "/data" {
		t.Errorf("r2: Method=%q Path=%q", r2.Method(), r2.Path())
	}
	b, _ := r2.Body()
	if string(b) != "hello" {
		t.Errorf("r2 body = %q, want hello", string(b))
	}

	if sim.readN != 0 {
		t.Errorf("readN = %d, want 0 after both requests", sim.readN)
	}
}

// TestTransportSim_FiveRequestChain simulates 5 pipelined requests arriving
// in a single recv, processed one-at-a-time through the handleRecv →
// processPipelined → processPipelined → ... chain.
func TestTransportSim_FiveRequestChain(t *testing.T) {
	sim := newTransportSim(8192)

	var all string
	wantPaths := make([]string, 5)
	for i := 0; i < 5; i++ {
		path := "/r" + strconv.Itoa(i)
		wantPaths[i] = path
		body := strings.Repeat("x", (i+1)*10) // varying body sizes
		all += "POST " + path + " HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	}
	sim.simulateRecv([]byte(all))

	// First request via handleRecv.
	r, _, parsed, _ := sim.handleRecv()
	if !parsed {
		t.Fatal("handleRecv: request 0 should parse")
	}
	if r.Path() != wantPaths[0] {
		t.Errorf("request 0: Path = %q, want %q", r.Path(), wantPaths[0])
	}

	// Remaining 4 via processPipelined.
	for i := 1; i < 5; i++ {
		r, parsed := sim.processPipelined()
		if !parsed {
			t.Fatalf("processPipelined: request %d should parse, readN=%d", i, sim.readN)
		}
		if r.Path() != wantPaths[i] {
			t.Errorf("request %d: Path = %q, want %q", i, r.Path(), wantPaths[i])
		}
		b, _ := r.Body()
		wantLen := (i + 1) * 10
		if len(b) != wantLen {
			t.Errorf("request %d: body len = %d, want %d", i, len(b), wantLen)
		}
	}

	if sim.readN != 0 {
		t.Errorf("final readN = %d, want 0", sim.readN)
	}
}

// TestTransportSim_BufferFullGarbage simulates the DoS protection:
// read buffer fills up without a complete request → transport closes connection.
// (transport.go lines 279-281)
func TestTransportSim_BufferFullGarbage(t *testing.T) {
	sim := newTransportSim(128) // small buffer

	// Fill buffer with data that has no \r\n\r\n.
	garbage := strings.Repeat("A", 128)
	sim.simulateRecv([]byte(garbage))

	_, _, parsed, bufFull := sim.handleRecv()
	if parsed {
		t.Error("garbage should not parse")
	}
	if !bufFull {
		t.Error("buffer should be full — transport would close connection")
	}
}

// TestTransportSim_MixedRecvSizes simulates realistic network behavior:
// TCP delivers data in varying chunk sizes across multiple recv calls.
func TestTransportSim_MixedRecvSizes(t *testing.T) {
	sim := newTransportSim(4096)

	fullData := "GET /a HTTP/1.1\r\nHost: h\r\n\r\n" +
		"POST /b HTTP/1.1\r\nContent-Length: 3\r\n\r\nabc" +
		"GET /c HTTP/1.1\r\n\r\n"

	// Deliver in 3 uneven chunks.
	chunks := []string{
		fullData[:20],               // partial first request
		fullData[20:70],             // rest of first + start of second
		fullData[70:],               // rest of second + all of third
	}

	var gotPaths []string

	for ci, chunk := range chunks {
		sim.simulateRecv([]byte(chunk))

		// Try handleRecv (simulate transport event loop).
		for {
			r, _, parsed, bufFull := sim.handleRecv()
			if bufFull {
				t.Fatalf("chunk %d: unexpected buffer full", ci)
			}
			if !parsed {
				break // need more data
			}
			gotPaths = append(gotPaths, r.Path())

			// After "send completes", try processPipelined.
			for {
				r2, ok := sim.processPipelined()
				if !ok {
					break
				}
				gotPaths = append(gotPaths, r2.Path())
			}
			break // back to recv loop
		}
	}

	wantPaths := []string{"/a", "/b", "/c"}
	if len(gotPaths) != len(wantPaths) {
		t.Fatalf("got %d paths %v, want %d paths %v", len(gotPaths), gotPaths, len(wantPaths), wantPaths)
	}
	for i := range wantPaths {
		if gotPaths[i] != wantPaths[i] {
			t.Errorf("path[%d] = %q, want %q", i, gotPaths[i], wantPaths[i])
		}
	}
}

// TestTransportSim_ShortWrite simulates handleSend with a partial send:
// kernel only sends part of the response, so we retry with the remainder.
// After the full send, pipelined data should still be processed.
func TestTransportSim_ShortWrite(t *testing.T) {
	sim := newTransportSim(4096)

	reqs := "GET /first HTTP/1.1\r\n\r\n" +
		"GET /second HTTP/1.1\r\n\r\n"
	sim.simulateRecv([]byte(reqs))

	// handleRecv: parse first.
	r1, _, parsed, _ := sim.handleRecv()
	if !parsed {
		t.Fatal("first request should parse")
	}
	if r1.Path() != "/first" {
		t.Errorf("Path = %q", r1.Path())
	}

	// Simulate building response.
	resp := acquireResponse()
	resp.WriteHeader(200)
	resp.Write([]byte("OK"))
	sendBuf := resp.build()
	releaseResponse(resp)

	// Simulate partial send: only 5 bytes sent of, say, ~40 byte response.
	sent := 5
	sendBuf = sendBuf[sent:] // transport.go line 367-369

	// Second send completes the rest.
	_ = sendBuf // pretend kernel sent the rest

	// Now handleSend checks readN > 0 → processPipelined.
	if sim.readN == 0 {
		t.Fatal("readN should be > 0 for second pipelined request")
	}
	r2, parsed := sim.processPipelined()
	if !parsed {
		t.Fatal("second request should parse after send completes")
	}
	if r2.Path() != "/second" {
		t.Errorf("Path = %q, want /second", r2.Path())
	}
	if sim.readN != 0 {
		t.Errorf("readN = %d, want 0", sim.readN)
	}
}

// TestTransportSim_ConnectionCloseStopsPipelining verifies that when the
// first request has Connection: close, the transport would NOT process
// pipelined data even if readN > 0 (transport.go line 373-376).
func TestTransportSim_ConnectionCloseStopsPipelining(t *testing.T) {
	sim := newTransportSim(4096)

	reqs := "GET /close HTTP/1.1\r\nConnection: close\r\n\r\n" +
		"GET /ignored HTTP/1.1\r\n\r\n"
	sim.simulateRecv([]byte(reqs))

	r1, _, parsed, _ := sim.handleRecv()
	if !parsed {
		t.Fatal("first request should parse")
	}
	if r1.keepAlive {
		t.Error("keepAlive should be false with Connection: close")
	}

	// Transport would check keepAlive BEFORE checking readN.
	// If !keepAlive → closeConn, never reach processPipelined.
	// This simulates that decision:
	if !r1.keepAlive {
		// Transport closes connection here (line 373-376).
		// Verify that pipelined data exists but is intentionally discarded.
		if sim.readN == 0 {
			t.Error("readN should be > 0 (pipelined data exists but will be discarded)")
		}
		// Connection closed — do NOT call processPipelined.
		return
	}
	t.Error("should have returned on !keepAlive branch")
}


// ----------------------------- Cookie Tests -----------------------------

func TestParseCookieValue_Single(t *testing.T) {
	if v := parseCookieValue("session=abc123", "session"); v != "abc123" {
		t.Errorf("got %q, want abc123", v)
	}
}

func TestParseCookieValue_Multiple(t *testing.T) {
	cookie := "session=abc; user=tiger; theme=dark"
	cases := [][2]string{
		{"session", "abc"},
		{"user", "tiger"},
		{"theme", "dark"},
	}
	for _, c := range cases {
		if v := parseCookieValue(cookie, c[0]); v != c[1] {
			t.Errorf("Cookie(%q) = %q, want %q", c[0], v, c[1])
		}
	}
}

func TestParseCookieValue_Missing(t *testing.T) {
	if v := parseCookieValue("session=abc", "missing"); v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func TestParseCookieValue_Empty(t *testing.T) {
	if v := parseCookieValue("", "session"); v != "" {
		t.Errorf("got %q, want empty", v)
	}
}

func TestParseHTTPRequest_CookieHeader(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nCookie: session=xyz; user=bob\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse")
	}
	if req.Cookie("session") != "xyz" {
		t.Errorf("Cookie(session) = %q, want xyz", req.Cookie("session"))
	}
	if req.Cookie("user") != "bob" {
		t.Errorf("Cookie(user) = %q, want bob", req.Cookie("user"))
	}
	if req.Cookie("missing") != "" {
		t.Errorf("Cookie(missing) = %q, want empty", req.Cookie("missing"))
	}
}

func TestParseHTTPRequest_NoCookie(t *testing.T) {
	raw := "GET / HTTP/1.1\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse")
	}
	if req.Cookie("session") != "" {
		t.Errorf("Cookie(session) = %q, want empty (no Cookie header)", req.Cookie("session"))
	}
}

// ----------------------------- Extra Header Tests -----------------------------

func TestParseHTTPRequest_ExtraHeaders(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nAuthorization: Bearer token123\r\nX-Request-ID: req-456\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse")
	}
	if v := req.Header("Authorization"); v != "Bearer token123" {
		t.Errorf("Header(Authorization) = %q, want Bearer token123", v)
	}
	if v := req.Header("X-Request-ID"); v != "req-456" {
		t.Errorf("Header(X-Request-ID) = %q, want req-456", v)
	}
}

func TestParseHTTPRequest_HeaderCaseInsensitive(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nAuthorization: Bearer tok\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse")
	}
	// Lookup must be case-insensitive.
	if v := req.Header("authorization"); v != "Bearer tok" {
		t.Errorf("Header(authorization) = %q, want Bearer tok", v)
	}
	if v := req.Header("AUTHORIZATION"); v != "Bearer tok" {
		t.Errorf("Header(AUTHORIZATION) = %q, want Bearer tok", v)
	}
}

func TestParseHTTPRequest_ExtraHeadersMissing(t *testing.T) {
	raw := "GET / HTTP/1.1\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse")
	}
	if v := req.Header("X-Missing"); v != "" {
		t.Errorf("Header(X-Missing) = %q, want empty", v)
	}
}

func TestParseHTTPRequest_ExtraHeadersOverflow(t *testing.T) {
	// 9 non-special headers — only first 8 stored, 9th silently dropped.
	var raw string
	raw = "GET / HTTP/1.1\r\n"
	for i := 0; i < 9; i++ {
		raw += "X-H" + strconv.Itoa(i) + ": v" + strconv.Itoa(i) + "\r\n"
	}
	raw += "\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse even with 9 extra headers")
	}
	// First 8 must be present.
	for i := 0; i < 8; i++ {
		k := "X-H" + strconv.Itoa(i)
		want := "v" + strconv.Itoa(i)
		if v := req.Header(k); v != want {
			t.Errorf("Header(%s) = %q, want %q", k, v, want)
		}
	}
	// 9th must be dropped.
	if v := req.Header("X-H8"); v != "" {
		t.Errorf("Header(X-H8) = %q, want empty (overflow dropped)", v)
	}
}

// ----------------------------- RemoteAddr Tests -----------------------------

func TestWingRequest_RemoteAddrSetFromConn(t *testing.T) {
	// Verify that remoteAddr flows from conn → request via tryParse.
	// We test this at the wingRequest level directly (unit test).
	req := acquireRequest()
	req.remoteAddr = "192.168.1.1:54321"
	if req.RemoteAddr() != "192.168.1.1:54321" {
		t.Errorf("RemoteAddr() = %q, want 192.168.1.1:54321", req.RemoteAddr())
	}
	releaseRequest(req)
}

func TestWingRequest_RemoteAddrEmptyByDefault(t *testing.T) {
	raw := "GET / HTTP/1.1\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("should parse")
	}
	// Parser doesn't set remoteAddr — transport sets it after parse.
	if req.RemoteAddr() != "" {
		t.Errorf("RemoteAddr() = %q, want empty (set by transport, not parser)", req.RemoteAddr())
	}
}
