package transport

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// ---------------------------------------------------------------------------
// New provider methods tests
// ---------------------------------------------------------------------------

func TestMultipartForm(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	req := &netHTTPRequest{r: r}
	_, err := req.MultipartForm(1024)
	// Should return error for non-multipart request
	if err == nil {
		t.Error("expected error for non-multipart request")
	}
}

func TestContext(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	req := &netHTTPRequest{r: r}
	if req.Context() != r.Context() {
		t.Error("Context() should return the request context")
	}
}

func TestAllHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Test", "value1")
	r.Header.Set("X-Custom", "value2")
	r.Header.Add("X-Multi", "a")
	r.Header.Add("X-Multi", "b")

	req := &netHTTPRequest{r: r}
	headers := req.AllHeaders()
	if headers["X-Test"] != "value1" {
		t.Errorf("AllHeaders()[X-Test] = %q, want value1", headers["X-Test"])
	}
	if headers["X-Custom"] != "value2" {
		t.Errorf("AllHeaders()[X-Custom] = %q, want value2", headers["X-Custom"])
	}
	if headers["X-Multi"] != "a, b" {
		t.Errorf("AllHeaders()[X-Multi] = %q, want 'a, b'", headers["X-Multi"])
	}
}

func TestAllQuery(t *testing.T) {
	r := httptest.NewRequest("GET", "/path?a=1&b=2&c=3&b=4", nil)
	req := &netHTTPRequest{r: r}
	query := req.AllQuery()
	if query["a"] != "1" {
		t.Errorf("AllQuery()[a] = %q, want 1", query["a"])
	}
	if query["c"] != "3" {
		t.Errorf("AllQuery()[c] = %q, want 3", query["c"])
	}
	if query["b"] != "2, 4" {
		t.Errorf("AllQuery()[b] = %q, want '2, 4'", query["b"])
	}
}

func TestDirectHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}
	h := w.Header()
	
	// Cast to access DirectHeader method
	if dhm, ok := h.(*netHTTPHeaderMap); ok {
		direct := dhm.DirectHeader()
		// Test that we can use the direct header
		direct.Set("X-Direct", "test")
		if rec.Header().Get("X-Direct") != "test" {
			t.Error("DirectHeader() should provide direct access to underlying header")
		}
	} else {
		t.Error("Header() should return *netHTTPHeaderMap")
	}
}

func TestNewNetHTTPRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	req := NewNetHTTPRequest(r)
	if req.Method() != "GET" {
		t.Errorf("expected GET, got %s", req.Method())
	}
	if req.Path() != "/test" {
		t.Errorf("expected /test, got %s", req.Path())
	}
}

func TestNewNetHTTPRequestWithLimit(t *testing.T) {
	r := httptest.NewRequest("POST", "/test", strings.NewReader("hello"))
	req := NewNetHTTPRequestWithLimit(r, 10)
	body, err := req.Body()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("expected hello, got %s", body)
	}
}

func TestNewNetHTTPResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewNetHTTPResponseWriter(rec)
	w.WriteHeader(201)
	w.Write([]byte("test"))
	if rec.Code != 201 {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if rec.Body.String() != "test" {
		t.Errorf("expected test, got %s", rec.Body.String())
	}
}

func TestResponseWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}
	// Flush should not panic even if underlying writer doesn't support it
	w.Flush()
}

func TestBodyTooLargeError(t *testing.T) {
	err := &BodyTooLargeError{}
	if err.Error() != "request body too large" {
		t.Errorf("expected 'request body too large', got %s", err.Error())
	}
}

func TestNetHTTPTransport_Config(t *testing.T) {
	cfg := NetHTTPConfig{
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    15 * time.Second,
		MaxBodySize:    1024,
		MaxHeaderBytes: 2048,
		TrustProxy:     true,
		TLSCertFile:    "/path/to/cert.pem",
		TLSKeyFile:     "/path/to/key.pem",
	}
	transport := NewNetHTTP(cfg)
	
	if transport.config.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", transport.config.ReadTimeout)
	}
	if transport.config.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", transport.config.WriteTimeout)
	}
	if transport.config.IdleTimeout != 15*time.Second {
		t.Errorf("IdleTimeout = %v, want 15s", transport.config.IdleTimeout)
	}
	if transport.config.MaxBodySize != 1024 {
		t.Errorf("MaxBodySize = %d, want 1024", transport.config.MaxBodySize)
	}
	if transport.config.MaxHeaderBytes != 2048 {
		t.Errorf("MaxHeaderBytes = %d, want 2048", transport.config.MaxHeaderBytes)
	}
	if !transport.config.TrustProxy {
		t.Error("TrustProxy should be true")
	}
	if transport.config.TLSCertFile != "/path/to/cert.pem" {
		t.Errorf("TLSCertFile = %s, want /path/to/cert.pem", transport.config.TLSCertFile)
	}
	if transport.config.TLSKeyFile != "/path/to/key.pem" {
		t.Errorf("TLSKeyFile = %s, want /path/to/key.pem", transport.config.TLSKeyFile)
	}
}

func TestNetHTTPTransport_ZeroConfig(t *testing.T) {
	cfg := NetHTTPConfig{} // All zero values
	transport := NewNetHTTP(cfg)
	
	if transport.config.ReadTimeout != 0 {
		t.Errorf("ReadTimeout = %v, want 0", transport.config.ReadTimeout)
	}
	if transport.config.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0", transport.config.WriteTimeout)
	}
	if transport.config.IdleTimeout != 0 {
		t.Errorf("IdleTimeout = %v, want 0", transport.config.IdleTimeout)
	}
	if transport.config.MaxBodySize != 0 {
		t.Errorf("MaxBodySize = %d, want 0", transport.config.MaxBodySize)
	}
	if transport.config.MaxHeaderBytes != 0 {
		t.Errorf("MaxHeaderBytes = %d, want 0", transport.config.MaxHeaderBytes)
	}
	if transport.config.TrustProxy {
		t.Error("TrustProxy should be false by default")
	}
	if transport.config.TLSCertFile != "" {
		t.Errorf("TLSCertFile = %s, want empty", transport.config.TLSCertFile)
	}
	if transport.config.TLSKeyFile != "" {
		t.Errorf("TLSKeyFile = %s, want empty", transport.config.TLSKeyFile)
	}
}

func TestAllQuery_EmptyQuery(t *testing.T) {
	r := httptest.NewRequest("GET", "/path", nil)
	req := &netHTTPRequest{r: r}
	query := req.AllQuery()
	if len(query) != 0 {
		t.Errorf("expected empty map, got %v", query)
	}
}

func TestAllHeaders_EmptyHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	req := &netHTTPRequest{r: r}
	headers := req.AllHeaders()
	// Should have at least some default headers from httptest
	if headers == nil {
		t.Error("AllHeaders should not return nil")
	}
}

func TestNetHTTPAdapter_MaxBodyZero(t *testing.T) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		body, err := r.Body()
		if err != nil {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
		w.Write(body)
	})
	
	adapter := &netHTTPAdapter{handler: handler, maxBody: 0} // No limit
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader("test body"))
	
	adapter.ServeHTTP(rec, r)
	
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "test body" {
		t.Errorf("expected 'test body', got %s", rec.Body.String())
	}
}

func TestNetHTTPAdapter_TrustProxyFalse(t *testing.T) {
	var capturedAddr string
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		capturedAddr = r.RemoteAddr()
		w.WriteHeader(200)
	})
	
	adapter := &netHTTPAdapter{handler: handler, trustProxy: false}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.RemoteAddr = "10.0.0.1:12345"
	
	adapter.ServeHTTP(rec, r)
	
	if capturedAddr != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s (should ignore proxy headers)", capturedAddr)
	}
}

func TestNetHTTPAdapter_TrustProxyTrue(t *testing.T) {
	var capturedAddr string
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		capturedAddr = r.RemoteAddr()
		w.WriteHeader(200)
	})
	
	adapter := &netHTTPAdapter{handler: handler, trustProxy: true}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.RemoteAddr = "10.0.0.1:12345"
	
	adapter.ServeHTTP(rec, r)
	
	if capturedAddr != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s (should use proxy header)", capturedAddr)
	}
}

func TestNetHTTPTransport_ServerConfiguration(t *testing.T) {
	cfg := NetHTTPConfig{
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    15 * time.Second,
		MaxHeaderBytes: 2048,
	}
	transport := NewNetHTTP(cfg)
	
	// Test that ListenAndServe would configure the server properly
	// We can't actually start the server in tests, but we can verify
	// the configuration is stored correctly
	if transport.config.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", transport.config.ReadTimeout)
	}
	if transport.config.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", transport.config.WriteTimeout)
	}
	if transport.config.IdleTimeout != 15*time.Second {
		t.Errorf("IdleTimeout = %v, want 15s", transport.config.IdleTimeout)
	}
	if transport.config.MaxHeaderBytes != 2048 {
		t.Errorf("MaxHeaderBytes = %d, want 2048", transport.config.MaxHeaderBytes)
	}
}

func TestResponseWriter_WriteAfterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &netHTTPResponseWriter{w: rec, statusCode: 200}
	
	w.WriteHeader(201)
	w.Write([]byte("first"))
	w.Write([]byte("second"))
	
	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Body.String() != "firstsecond" {
		t.Errorf("body = %q, want firstsecond", rec.Body.String())
	}
}
