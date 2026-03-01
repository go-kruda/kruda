package wing

import (
	"strings"
	"testing"
)

func TestParseHTTPRequest_Simple(t *testing.T) {
	raw := "GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"
	req, ok := parseHTTPRequest([]byte(raw))
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
	req, ok := parseHTTPRequest([]byte(raw))
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
	req, ok := parseHTTPRequest([]byte(raw))
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
	req, ok := parseHTTPRequest([]byte(raw))
	if !ok {
		t.Fatal("parse failed")
	}
	if req.keepAlive {
		t.Error("keepAlive should be false when Connection: close")
	}
}

func TestParseHTTPRequest_ConnectionCloseCaseInsensitive(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nCONNECTION: CLOSE\r\n\r\n"
	req, ok := parseHTTPRequest([]byte(raw))
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
		_, ok := parseHTTPRequest([]byte(tt.raw))
		if ok {
			t.Errorf("%s: expected false, got true", tt.name)
		}
	}
}

func TestParseHTTPRequest_OversizedContentLength(t *testing.T) {
	// Content-Length > maxContentLength (10MB) should be rejected.
	raw := "POST / HTTP/1.1\r\nContent-Length: 99999999999\r\n\r\n"
	_, ok := parseHTTPRequest([]byte(raw))
	if ok {
		t.Error("should reject oversized Content-Length")
	}
}

func TestParseHTTPRequest_ZeroContentLength(t *testing.T) {
	raw := "POST / HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
	req, ok := parseHTTPRequest([]byte(raw))
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
	req, ok := parseHTTPRequest(raw)
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
	req, ok := parseHTTPRequest(raw)
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
		{"10485760", maxContentLength},      // exactly 10MB
		{"10485761", maxContentLength + 1},   // 10MB + 1 → overflow sentinel
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
	req, ok := parseHTTPRequest([]byte(raw))
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
