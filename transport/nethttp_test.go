package transport

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// stripPort tests
// ---------------------------------------------------------------------------

func TestStripPort_IPv4WithPort(t *testing.T) {
	if got := stripPort("192.168.1.1:8080"); got != "192.168.1.1" {
		t.Errorf("stripPort(192.168.1.1:8080) = %q, want 192.168.1.1", got)
	}
}

func TestStripPort_IPv4NoPort(t *testing.T) {
	if got := stripPort("192.168.1.1"); got != "192.168.1.1" {
		t.Errorf("stripPort(192.168.1.1) = %q, want 192.168.1.1", got)
	}
}

func TestStripPort_IPv6Brackets(t *testing.T) {
	if got := stripPort("[::1]:8080"); got != "::1" {
		t.Errorf("stripPort([::1]:8080) = %q, want ::1", got)
	}
}

func TestStripPort_IPv6BracketsNoPort(t *testing.T) {
	if got := stripPort("[::1]"); got != "::1" {
		t.Errorf("stripPort([::1]) = %q, want ::1", got)
	}
}

func TestStripPort_IPv6Full(t *testing.T) {
	if got := stripPort("[2001:db8::1]:443"); got != "2001:db8::1" {
		t.Errorf("stripPort([2001:db8::1]:443) = %q, want 2001:db8::1", got)
	}
}

func TestStripPort_Empty(t *testing.T) {
	if got := stripPort(""); got != "" {
		t.Errorf("stripPort('') = %q, want empty", got)
	}
}

func TestStripPort_HostnameWithPort(t *testing.T) {
	if got := stripPort("example.com:443"); got != "example.com" {
		t.Errorf("stripPort(example.com:443) = %q, want example.com", got)
	}
}

func TestStripPort_IPv6UnclosedBracket(t *testing.T) {
	// Malformed — no closing bracket, returns input unchanged
	got := stripPort("[::1")
	if got != "[::1" {
		t.Errorf("stripPort([::1) = %q, want [::1", got)
	}
}

// ---------------------------------------------------------------------------
// trimSpace tests
// ---------------------------------------------------------------------------

func TestTrimSpace(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello", "hello"},
		{" hello ", "hello"},
		{"  spaced  ", "spaced"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		if got := trimSpace(tt.in); got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// RemoteAddr tests
// ---------------------------------------------------------------------------

func TestRemoteAddr_NoTrustProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("X-Real-Ip", "5.6.7.8")

	req := &netHTTPRequest{r: r, trustProxy: false}
	if got := req.RemoteAddr(); got != "10.0.0.1" {
		t.Errorf("RemoteAddr() = %q, want 10.0.0.1 (should ignore proxy headers)", got)
	}
}

func TestRemoteAddr_TrustProxy_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")

	req := &netHTTPRequest{r: r, trustProxy: true}
	if got := req.RemoteAddr(); got != "1.2.3.4" {
		t.Errorf("RemoteAddr() = %q, want 1.2.3.4 (first in chain)", got)
	}
}

func TestRemoteAddr_TrustProxy_XForwardedFor_SingleIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", " 1.2.3.4 ")

	req := &netHTTPRequest{r: r, trustProxy: true}
	if got := req.RemoteAddr(); got != "1.2.3.4" {
		t.Errorf("RemoteAddr() = %q, want 1.2.3.4 (trimmed)", got)
	}
}

func TestRemoteAddr_TrustProxy_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Real-Ip", "5.6.7.8")

	req := &netHTTPRequest{r: r, trustProxy: true}
	if got := req.RemoteAddr(); got != "5.6.7.8" {
		t.Errorf("RemoteAddr() = %q, want 5.6.7.8", got)
	}
}

func TestRemoteAddr_TrustProxy_NoHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"

	req := &netHTTPRequest{r: r, trustProxy: true}
	if got := req.RemoteAddr(); got != "10.0.0.1" {
		t.Errorf("RemoteAddr() = %q, want 10.0.0.1 (fallback to RemoteAddr)", got)
	}
}

func TestRemoteAddr_IPv6(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "[::1]:8080"

	req := &netHTTPRequest{r: r, trustProxy: false}
	if got := req.RemoteAddr(); got != "::1" {
		t.Errorf("RemoteAddr() = %q, want ::1", got)
	}
}

// ---------------------------------------------------------------------------
// Body tests
// ---------------------------------------------------------------------------

func TestBody_ReadOnce(t *testing.T) {
	body := `{"key":"value"}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))

	req := &netHTTPRequest{r: r}
	data, err := req.Body()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != body {
		t.Errorf("Body() = %q, want %q", data, body)
	}

	// Second call returns cached result
	data2, err2 := req.Body()
	if err2 != nil {
		t.Fatalf("unexpected error on second call: %v", err2)
	}
	if string(data2) != body {
		t.Errorf("Body() second call = %q, want %q", data2, body)
	}
}

func TestBody_NilBody(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)

	req := &netHTTPRequest{r: r}
	data, err := req.Body()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// httptest.NewRequest with nil body produces http.NoBody → empty read
	if len(data) != 0 {
		t.Errorf("Body() len = %d, want 0", len(data))
	}
}

func TestBody_ExceedsMaxBody(t *testing.T) {
	body := strings.Repeat("x", 100)
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))

	req := &netHTTPRequest{r: r, maxBody: 50}
	_, err := req.Body()
	if err == nil {
		t.Fatal("expected ErrBodyTooLarge, got nil")
	}
	if err != ErrBodyTooLarge {
		t.Errorf("error = %v, want ErrBodyTooLarge", err)
	}
}

func TestBody_ExactlyMaxBody(t *testing.T) {
	body := strings.Repeat("x", 50)
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))

	req := &netHTTPRequest{r: r, maxBody: 50}
	data, err := req.Body()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 50 {
		t.Errorf("Body() len = %d, want 50", len(data))
	}
}

// ---------------------------------------------------------------------------
// QueryParam caching tests
// ---------------------------------------------------------------------------

func TestQueryParam_Cached(t *testing.T) {
	r := httptest.NewRequest("GET", "/path?a=1&b=2&c=3", nil)
	req := &netHTTPRequest{r: r}

	if got := req.QueryParam("a"); got != "1" {
		t.Errorf("QueryParam(a) = %q, want 1", got)
	}
	if got := req.QueryParam("b"); got != "2" {
		t.Errorf("QueryParam(b) = %q, want 2", got)
	}
	// queryDone should be true after first call — subsequent calls use cache
	if !req.queryDone {
		t.Error("queryDone should be true after first QueryParam call")
	}
	// Third call still works
	if got := req.QueryParam("c"); got != "3" {
		t.Errorf("QueryParam(c) = %q, want 3", got)
	}
}

func TestQueryParam_Missing(t *testing.T) {
	r := httptest.NewRequest("GET", "/path?a=1", nil)
	req := &netHTTPRequest{r: r}

	if got := req.QueryParam("missing"); got != "" {
		t.Errorf("QueryParam(missing) = %q, want empty", got)
	}
}

func TestQueryParam_NoQueryString(t *testing.T) {
	r := httptest.NewRequest("GET", "/path", nil)
	req := &netHTTPRequest{r: r}

	if got := req.QueryParam("any"); got != "" {
		t.Errorf("QueryParam(any) = %q, want empty", got)
	}
}

func TestQueryParam_NoReParseOnMiss(t *testing.T) {
	r := httptest.NewRequest("GET", "/path?a=1", nil)
	req := &netHTTPRequest{r: r}

	// First call parses and caches
	req.QueryParam("a")
	if !req.queryDone {
		t.Fatal("queryDone should be true after first call")
	}

	// Lookup a missing key — queryDone stays true (no re-parse)
	if got := req.QueryParam("missing"); got != "" {
		t.Errorf("QueryParam(missing) = %q, want empty", got)
	}
	if !req.queryDone {
		t.Error("queryDone should still be true after missing key lookup")
	}

	// Lookup the same missing key again — still cached
	if got := req.QueryParam("missing"); got != "" {
		t.Errorf("QueryParam(missing) second call = %q, want empty", got)
	}

	// Original key still accessible from cache
	if got := req.QueryParam("a"); got != "1" {
		t.Errorf("QueryParam(a) after miss = %q, want 1", got)
	}
}

func TestQueryParam_SingleParseMultipleKeys(t *testing.T) {
	r := httptest.NewRequest("GET", "/path?x=10&y=20&z=30", nil)
	req := &netHTTPRequest{r: r}

	// Access keys in different order — all should work from single parse
	if got := req.QueryParam("z"); got != "30" {
		t.Errorf("QueryParam(z) = %q, want 30", got)
	}
	if !req.queryDone {
		t.Fatal("queryDone should be true after first call")
	}

	if got := req.QueryParam("x"); got != "10" {
		t.Errorf("QueryParam(x) = %q, want 10", got)
	}
	if got := req.QueryParam("y"); got != "20" {
		t.Errorf("QueryParam(y) = %q, want 20", got)
	}

	// All three keys accessible — proves single parse cached all values
	if len(req.queryVals) < 3 {
		t.Errorf("queryVals has %d entries, want at least 3", len(req.queryVals))
	}
}

// ---------------------------------------------------------------------------
// ResponseWriter tests
// ---------------------------------------------------------------------------

func TestResponseWriter_WriteHeader_DoubleWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}

	w.WriteHeader(201)
	w.WriteHeader(500) // should be ignored

	if rec.Code != 201 {
		t.Errorf("status = %d, want 201 (second WriteHeader should be ignored)", rec.Code)
	}
}

func TestResponseWriter_Write_ImplicitHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
	if rec.Code != 200 {
		t.Errorf("status = %d, want 200 (implicit)", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want hello", rec.Body.String())
	}
}

func TestResponseWriter_Header(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}

	h := w.Header()
	h.Set("X-Custom", "value")
	h.Add("X-Multi", "a")
	h.Add("X-Multi", "b")

	if got := h.Get("X-Custom"); got != "value" {
		t.Errorf("Get(X-Custom) = %q, want value", got)
	}
	if got := rec.Header().Values("X-Multi"); len(got) != 2 {
		t.Errorf("X-Multi values = %v, want 2 values", got)
	}

	h.Del("X-Custom")
	if got := h.Get("X-Custom"); got != "" {
		t.Errorf("Get(X-Custom) after Del = %q, want empty", got)
	}
}

func TestResponseWriter_Unwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}

	if w.Unwrap() != rec {
		t.Error("Unwrap() should return the underlying http.ResponseWriter")
	}
}

// ---------------------------------------------------------------------------
// Cookie test
// ---------------------------------------------------------------------------

func TestCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

	req := &netHTTPRequest{r: r}
	if got := req.Cookie("session"); got != "abc123" {
		t.Errorf("Cookie(session) = %q, want abc123", got)
	}
	if got := req.Cookie("missing"); got != "" {
		t.Errorf("Cookie(missing) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Path tests
// ---------------------------------------------------------------------------

func TestPath_Normal(t *testing.T) {
	r := httptest.NewRequest("GET", "/users/123", nil)
	req := &netHTTPRequest{r: r}
	if got := req.Path(); got != "/users/123" {
		t.Errorf("Path() = %q, want /users/123", got)
	}
}

func TestPath_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.URL.Path = ""
	req := &netHTTPRequest{r: r}
	if got := req.Path(); got != "/" {
		t.Errorf("Path() = %q, want / (empty path defaults to root)", got)
	}
}

// ---------------------------------------------------------------------------
// RawRequest test
// ---------------------------------------------------------------------------

func TestRawRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	req := &netHTTPRequest{r: r}
	if req.RawRequest() != r {
		t.Error("RawRequest() should return the underlying *http.Request")
	}
}

// ---------------------------------------------------------------------------
// Integration: netHTTPAdapter.ServeHTTP
// ---------------------------------------------------------------------------

func TestNetHTTPAdapter_ServeHTTP(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody []byte

	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		capturedMethod = r.Method()
		capturedPath = r.Path()
		capturedBody, _ = r.Body()
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(201)
		w.Write([]byte("created"))
	})

	adapter := &netHTTPAdapter{handler: handler, maxBody: 1024}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/items", bytes.NewReader([]byte(`{"name":"test"}`)))

	adapter.ServeHTTP(rec, r)

	if capturedMethod != "POST" {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
	if capturedPath != "/items" {
		t.Errorf("path = %q, want /items", capturedPath)
	}
	if string(capturedBody) != `{"name":"test"}` {
		t.Errorf("body = %q, want {\"name\":\"test\"}", capturedBody)
	}
	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Body.String() != "created" {
		t.Errorf("response body = %q, want created", rec.Body.String())
	}
	if rec.Header().Get("X-Test") != "ok" {
		t.Errorf("X-Test header = %q, want ok", rec.Header().Get("X-Test"))
	}
}

// ---------------------------------------------------------------------------
// Header method test
// ---------------------------------------------------------------------------

func TestHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer token123")

	req := &netHTTPRequest{r: r}
	if got := req.Header("Authorization"); got != "Bearer token123" {
		t.Errorf("Header(Authorization) = %q, want Bearer token123", got)
	}
	if got := req.Header("Missing"); got != "" {
		t.Errorf("Header(Missing) = %q, want empty", got)
	}
}
