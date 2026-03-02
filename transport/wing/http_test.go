package wing

import (
	"strings"
	"testing"
)

func TestParseHTTPRequest_Simple(t *testing.T) {
	raw := "GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
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
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
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
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
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
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	if req.keepAlive {
		t.Error("keepAlive should be false when Connection: close")
	}
}

func TestParseHTTPRequest_ConnectionCloseCaseInsensitive(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nCONNECTION: CLOSE\r\n\r\n"
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
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
		_, ok := parseHTTPRequest([]byte(tt.raw), noLimits)
		if ok {
			t.Errorf("%s: expected false, got true", tt.name)
		}
	}
}

func TestParseHTTPRequest_OversizedContentLength(t *testing.T) {
	// Content-Length > maxContentLength (10MB) should be rejected.
	raw := "POST / HTTP/1.1\r\nContent-Length: 99999999999\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject oversized Content-Length")
	}
}

func TestParseHTTPRequest_ZeroContentLength(t *testing.T) {
	raw := "POST / HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
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
	req, ok := parseHTTPRequest(raw, noLimits)
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
	req, ok := parseHTTPRequest(raw, noLimits)
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
	req, ok := parseHTTPRequest([]byte(raw), noLimits)
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
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should reject when header count exceeds limit")
	}
}

func TestParseHTTPRequest_HeaderCountAtLimit(t *testing.T) {
	// Exactly 2 headers with limit of 2 → accept.
	raw := "GET / HTTP/1.1\r\nA: 1\r\nB: 2\r\n\r\n"
	limits := parserLimits{maxHeaderCount: 2}
	req, ok := parseHTTPRequest([]byte(raw), limits)
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
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Error("should accept when maxHeaderCount is 0 (unlimited)")
	}
}

func TestParseHTTPRequest_HeaderSizeLimit(t *testing.T) {
	// Header "X-Big: value" is 12 bytes. Limit to 10 → reject.
	raw := "GET / HTTP/1.1\r\nX-Big: value\r\n\r\n"
	limits := parserLimits{maxHeaderSize: 10}
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should reject when header size exceeds limit")
	}
}

func TestParseHTTPRequest_HeaderSizeAtLimit(t *testing.T) {
	// Header "A: ok" after CRLF strip is 5 bytes. Limit to 5 → accept.
	raw := "GET / HTTP/1.1\r\nA: ok\r\n\r\n"
	limits := parserLimits{maxHeaderSize: 5}
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Fatal("should accept when header size equals limit")
	}
}

func TestParseHTTPRequest_HeaderSizeUnlimited(t *testing.T) {
	// Large header with limit of 0 (unlimited) → accept.
	raw := "GET / HTTP/1.1\r\nX-Large: " + strings.Repeat("x", 10000) + "\r\n\r\n"
	limits := parserLimits{maxHeaderSize: 0}
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Error("should accept when maxHeaderSize is 0 (unlimited)")
	}
}

func TestParseHTTPRequest_CustomLimits(t *testing.T) {
	// Recommended production limits: 100 headers, 8192 bytes per header.
	limits := parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192}

	// Normal request with a few headers → accept.
	raw := "GET / HTTP/1.1\r\nHost: localhost\r\nAccept: text/html\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if !ok {
		t.Error("normal request should pass with recommended limits")
	}

	// Header value exceeding 8192 bytes → reject.
	bigRaw := "GET / HTTP/1.1\r\nX-Big: " + strings.Repeat("a", 8200) + "\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(bigRaw), limits)
	if ok {
		t.Error("should reject header exceeding 8192 byte limit")
	}
}

func TestParseHTTPRequest_MaxContentLengthStillWorks(t *testing.T) {
	// Verify maxContentLength (10MB) rejection still works after refactor.
	raw := "POST / HTTP/1.1\r\nContent-Length: 99999999999\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should still reject oversized Content-Length with noLimits")
	}

	// Also verify with custom limits set.
	limits := parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192}
	_, ok = parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should still reject oversized Content-Length with custom limits")
	}

	// Valid Content-Length with body → accept.
	raw2 := "POST / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello"
	req, ok := parseHTTPRequest([]byte(raw2), limits)
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
	_, ok := parseHTTPRequest([]byte(raw), limits)
	if ok {
		t.Error("should reject: 2 headers exceeds maxHeaderCount=1")
	}

	// Both limits active — size triggers first.
	limits2 := parserLimits{maxHeaderCount: 100, maxHeaderSize: 5}
	raw2 := "GET / HTTP/1.1\r\nX-Long: abcdef\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw2), limits2)
	if ok {
		t.Error("should reject: header size exceeds maxHeaderSize=5")
	}
}

// ----------------------------- R2: Injection Prevention Tests -----------------------------

func TestParseHTTPRequest_CRLFInjectionInHeaderValue(t *testing.T) {
	// R2.1: Bare \r in header value → reject.
	raw := "GET / HTTP/1.1\r\nX-Evil: foo\rbar\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject header value containing bare CR")
	}

	// R2.1: Bare \n in header value → reject.
	raw2 := "GET / HTTP/1.1\r\nX-Evil: foo\nbar\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject header value containing bare LF")
	}

	// R2.1: CRLF injection attempt (header splitting) → reject.
	raw3 := "GET / HTTP/1.1\r\nX-Evil: foo\r\nInjected: bar\r\n\r\n"
	// This is actually two separate headers — "X-Evil: foo" and "Injected: bar".
	// Both are valid individually. The CRLF is the line terminator, not inside the value.
	// This should parse fine (the CRLF is stripped by the line parser).
	_, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if !ok {
		t.Error("two valid headers should parse fine")
	}
}

func TestParseHTTPRequest_DuplicateContentLength(t *testing.T) {
	// R2.2: Duplicate Content-Length headers → reject.
	raw := "POST / HTTP/1.1\r\nContent-Length: 5\r\nContent-Length: 5\r\n\r\nhello"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject duplicate Content-Length headers")
	}

	// R2.2: Duplicate with different values → also reject.
	raw2 := "POST / HTTP/1.1\r\nContent-Length: 5\r\nContent-Length: 10\r\n\r\nhello"
	_, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject duplicate Content-Length with different values")
	}

	// Single Content-Length → accept.
	raw3 := "POST / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello"
	_, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if !ok {
		t.Error("single Content-Length should be accepted")
	}
}

func TestParseHTTPRequest_TEAndCLConflict(t *testing.T) {
	// R2.3: Both Transfer-Encoding and Content-Length → reject (RFC 7230 §3.3.3).
	raw := "POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\nContent-Length: 5\r\n\r\nhello"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject request with both Transfer-Encoding and Content-Length")
	}

	// R2.3: Case-insensitive match.
	raw2 := "POST / HTTP/1.1\r\ntransfer-encoding: chunked\r\ncontent-length: 5\r\n\r\nhello"
	_, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject TE+CL conflict (case-insensitive)")
	}

	// Transfer-Encoding alone → accept.
	raw3 := "POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if !ok {
		t.Error("Transfer-Encoding alone should be accepted")
	}
}

func TestParseHTTPRequest_NonNumericContentLength(t *testing.T) {
	// R2.4: Non-numeric Content-Length → reject.
	raw := "POST / HTTP/1.1\r\nContent-Length: abc\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject non-numeric Content-Length")
	}

	// R2.4: Mixed numeric/alpha → reject.
	raw2 := "POST / HTTP/1.1\r\nContent-Length: 5abc\r\n\r\nhello"
	_, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject Content-Length with trailing non-digits")
	}

	// R2.4: Empty Content-Length value → reject.
	raw3 := "POST / HTTP/1.1\r\nContent-Length: \r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if ok {
		t.Error("should reject empty Content-Length value")
	}

	// R2.4: Negative Content-Length → reject.
	raw4 := "POST / HTTP/1.1\r\nContent-Length: -1\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw4), noLimits)
	if ok {
		t.Error("should reject negative Content-Length")
	}

	// Valid numeric Content-Length → accept.
	raw5 := "POST / HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw5), noLimits)
	if !ok {
		t.Error("Content-Length: 0 should be accepted")
	}
}

func TestParseHTTPRequest_InvalidHeaderName(t *testing.T) {
	// R2.5: Header name with space → reject.
	raw := "GET / HTTP/1.1\r\nBad Header: value\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject header name containing space")
	}

	// R2.5: Header name with colon-like chars → reject (parenthesis is a delimiter).
	raw2 := "GET / HTTP/1.1\r\nBad(Header): value\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject header name containing '('")
	}

	// R2.5: Header name with high-byte (>127) → reject.
	raw3 := "GET / HTTP/1.1\r\nBad\x80Header: value\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if ok {
		t.Error("should reject header name with byte > 127")
	}

	// R2.5: Valid token characters → accept.
	raw4 := "GET / HTTP/1.1\r\nX-Custom-Header_v2: value\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw4), noLimits)
	if !ok {
		t.Error("valid token header name should be accepted")
	}
}

func TestParseHTTPRequest_MalformedRequestLine(t *testing.T) {
	// R2.6: Missing HTTP version → reject.
	raw := "GET /\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw), noLimits)
	if ok {
		t.Error("should reject request line without HTTP version")
	}

	// R2.6: Invalid version prefix → reject.
	raw2 := "GET / HTTZ/1.1\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw2), noLimits)
	if ok {
		t.Error("should reject request line with invalid version prefix")
	}

	// R2.6: Version too short → reject.
	raw3 := "GET / HTTP\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw3), noLimits)
	if ok {
		t.Error("should reject request line with version 'HTTP' (no slash)")
	}

	// R2.6: Valid HTTP/1.1 → accept.
	raw4 := "GET / HTTP/1.1\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw4), noLimits)
	if !ok {
		t.Error("valid HTTP/1.1 request line should be accepted")
	}

	// R2.6: Valid HTTP/1.0 → accept.
	raw5 := "GET / HTTP/1.0\r\n\r\n"
	_, ok = parseHTTPRequest([]byte(raw5), noLimits)
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
