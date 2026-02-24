//go:build !windows

package transport

import (
	"context"
	"net"
	"testing"
	"testing/quick"
)

// --- parseRequestLine tests ---

func TestParseRequestLine(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		method string
		path   string
		query  string
		ok     bool
	}{
		{
			name:   "GET with query",
			input:  "GET /path?q=1 HTTP/1.1\r\n",
			method: "GET", path: "/path", query: "q=1", ok: true,
		},
		{
			name:   "POST no query",
			input:  "POST /users HTTP/1.1\r\n",
			method: "POST", path: "/users", query: "", ok: true,
		},
		{
			name:   "GET root",
			input:  "GET / HTTP/1.1\r\n",
			method: "GET", path: "/", query: "", ok: true,
		},
		{
			name:   "DELETE with path",
			input:  "DELETE /items/42 HTTP/1.1\r\n",
			method: "DELETE", path: "/items/42", query: "", ok: true,
		},
		{
			name:   "GET with multiple query params",
			input:  "GET /search?q=hello&page=2 HTTP/1.1\r\n",
			method: "GET", path: "/search", query: "q=hello&page=2", ok: true,
		},
		{
			name:   "no trailing CRLF",
			input:  "GET /path HTTP/1.1",
			method: "GET", path: "/path", query: "", ok: true,
		},
		{
			name:   "LF only",
			input:  "GET /path HTTP/1.1\n",
			method: "GET", path: "/path", query: "", ok: true,
		},
		{
			name:   "empty path becomes slash",
			input:  "GET ?q=1 HTTP/1.1\r\n",
			method: "GET", path: "/", query: "q=1", ok: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, query, ok := parseRequestLine(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if method != tt.method {
				t.Errorf("method = %q, want %q", method, tt.method)
			}
			if path != tt.path {
				t.Errorf("path = %q, want %q", path, tt.path)
			}
			if query != tt.query {
				t.Errorf("query = %q, want %q", query, tt.query)
			}
		})
	}
}

func TestParseRequestLine_Malformed(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"single word", "GET"},
		{"no HTTP version", "GET /path"},
		{"garbage", "asdfghjkl"},
		{"no space after method", "GET/path HTTP/1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, ok := parseRequestLine(tt.input)
			if ok {
				t.Errorf("expected ok=false for input %q", tt.input)
			}
		})
	}
}

// --- trimCRLF tests ---

func TestTrimCRLF(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"CRLF ending", "hello\r\n", "hello"},
		{"LF only", "hello\n", "hello"},
		{"CR only", "hello\r", "hello"},
		{"no trailing", "hello", "hello"},
		{"empty string", "", ""},
		{"only CRLF", "\r\n", ""},
		{"only LF", "\n", ""},
		{"multiple CRLF", "hello\r\n\r\n", "hello\r\n"},
		{"content with inner newlines", "a\nb\r\n", "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimCRLF(tt.input)
			if got != tt.want {
				t.Errorf("trimCRLF(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseQuery tests ---

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "normal",
			input: "a=1&b=2",
			want:  map[string]string{"a": "1", "b": "2"},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "single key-value",
			input: "key=value",
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "key without value",
			input: "key",
			want:  map[string]string{"key": ""},
		},
		{
			name:  "key with empty value",
			input: "key=",
			want:  map[string]string{"key": ""},
		},
		{
			name:  "duplicate keys first wins",
			input: "a=1&a=2",
			want:  map[string]string{"a": "1"},
		},
		{
			name:  "trailing ampersand",
			input: "a=1&",
			want:  map[string]string{"a": "1"},
		},
		{
			name:  "leading ampersand",
			input: "&a=1",
			want:  map[string]string{"a": "1"},
		},
		{
			name:  "double ampersand",
			input: "a=1&&b=2",
			want:  map[string]string{"a": "1", "b": "2"},
		},
		{
			name:  "multiple keys no values",
			input: "a&b&c",
			want:  map[string]string{"a": "", "b": "", "c": ""},
		},
		{
			name:  "value with equals",
			input: "a=1=2",
			want:  map[string]string{"a": "1=2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseQuery(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// --- statusText tests ---

func TestStatusText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{405, "Method Not Allowed"},
		{413, "Content Too Large"},
		{422, "Unprocessable Entity"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{504, "Gateway Timeout"},
		{999, "Unknown"},
		{0, "Unknown"},
	}

	for _, tt := range tests {
		got := statusText(tt.code)
		if got != tt.want {
			t.Errorf("statusText(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// --- netpollRequest tests ---

func TestNetpollRequest_Method(t *testing.T) {
	req := &netpollRequest{method: "POST"}
	if got := req.Method(); got != "POST" {
		t.Errorf("Method() = %q, want %q", got, "POST")
	}
}

func TestNetpollRequest_Path(t *testing.T) {
	req := &netpollRequest{path: "/api/v1/users"}
	if got := req.Path(); got != "/api/v1/users" {
		t.Errorf("Path() = %q, want %q", got, "/api/v1/users")
	}
}

func TestNetpollRequest_Header(t *testing.T) {
	req := &netpollRequest{
		headers: [][2]string{
			{"Content-Type", "application/json"},
			{"X-Custom", "value"},
		},
	}

	// Case-insensitive lookup.
	if got := req.Header("content-type"); got != "application/json" {
		t.Errorf("Header(content-type) = %q, want %q", got, "application/json")
	}
	if got := req.Header("CONTENT-TYPE"); got != "application/json" {
		t.Errorf("Header(CONTENT-TYPE) = %q, want %q", got, "application/json")
	}
	if got := req.Header("X-Custom"); got != "value" {
		t.Errorf("Header(X-Custom) = %q, want %q", got, "value")
	}
	// Missing header returns empty.
	if got := req.Header("X-Missing"); got != "" {
		t.Errorf("Header(X-Missing) = %q, want empty", got)
	}
}

func TestNetpollRequest_HeaderVal(t *testing.T) {
	req := &netpollRequest{
		headers: [][2]string{
			{"Content-Type", "application/json"},
			{"X-Custom", "value"},
		},
	}

	// Case-insensitive match (RFC 7230 compliant).
	if got := req.headerVal("Content-Type"); got != "application/json" {
		t.Errorf("headerVal(Content-Type) = %q, want %q", got, "application/json")
	}
	if got := req.headerVal("content-type"); got != "application/json" {
		t.Errorf("headerVal(content-type) = %q, want %q", got, "application/json")
	}
	if got := req.headerVal("CONTENT-TYPE"); got != "application/json" {
		t.Errorf("headerVal(CONTENT-TYPE) = %q, want %q", got, "application/json")
	}
	// Missing header returns empty.
	if got := req.headerVal("X-Missing"); got != "" {
		t.Errorf("headerVal(X-Missing) = %q, want empty", got)
	}
}

func TestNetpollRequest_QueryParam(t *testing.T) {
	req := &netpollRequest{rawQuery: "name=kruda&version=1"}

	// First access triggers lazy parse.
	if got := req.QueryParam("name"); got != "kruda" {
		t.Errorf("QueryParam(name) = %q, want %q", got, "kruda")
	}
	if got := req.QueryParam("version"); got != "1" {
		t.Errorf("QueryParam(version) = %q, want %q", got, "1")
	}
	// Missing key returns empty.
	if got := req.QueryParam("missing"); got != "" {
		t.Errorf("QueryParam(missing) = %q, want empty", got)
	}
	// Verify queryDone flag is set (lazy parse happened once).
	if !req.queryDone {
		t.Error("expected queryDone to be true after first QueryParam call")
	}
}

func TestNetpollRequest_QueryParam_Empty(t *testing.T) {
	req := &netpollRequest{rawQuery: ""}
	if got := req.QueryParam("any"); got != "" {
		t.Errorf("QueryParam on empty query = %q, want empty", got)
	}
}

func TestNetpollRequest_RemoteAddr(t *testing.T) {
	tcpAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}

	t.Run("direct no trust proxy", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: tcpAddr,
			trustProxy: false,
			headers: [][2]string{
				{"X-Forwarded-For", "10.0.0.1"},
			},
		}
		got := req.RemoteAddr()
		if got != "192.168.1.1" {
			t.Errorf("RemoteAddr() = %q, want %q", got, "192.168.1.1")
		}
	})

	t.Run("trust proxy XFF", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: tcpAddr,
			trustProxy: true,
			headers: [][2]string{
				{"X-Forwarded-For", "10.0.0.1, 10.0.0.2"},
			},
		}
		got := req.RemoteAddr()
		if got != "10.0.0.1" {
			t.Errorf("RemoteAddr() = %q, want %q", got, "10.0.0.1")
		}
	})

	t.Run("trust proxy XFF single IP", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: tcpAddr,
			trustProxy: true,
			headers: [][2]string{
				{"X-Forwarded-For", "10.0.0.5"},
			},
		}
		got := req.RemoteAddr()
		if got != "10.0.0.5" {
			t.Errorf("RemoteAddr() = %q, want %q", got, "10.0.0.5")
		}
	})

	t.Run("trust proxy XRI", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: tcpAddr,
			trustProxy: true,
			headers: [][2]string{
				{"X-Real-Ip", "172.16.0.1"},
			},
		}
		got := req.RemoteAddr()
		if got != "172.16.0.1" {
			t.Errorf("RemoteAddr() = %q, want %q", got, "172.16.0.1")
		}
	})

	t.Run("trust proxy no headers falls back to remote", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: tcpAddr,
			trustProxy: true,
		}
		got := req.RemoteAddr()
		if got != "192.168.1.1" {
			t.Errorf("RemoteAddr() = %q, want %q", got, "192.168.1.1")
		}
	})

	t.Run("nil remote addr", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: nil,
			trustProxy: false,
		}
		got := req.RemoteAddr()
		if got != "" {
			t.Errorf("RemoteAddr() = %q, want empty", got)
		}
	})

	t.Run("XFF with spaces", func(t *testing.T) {
		req := &netpollRequest{
			remoteAddr: tcpAddr,
			trustProxy: true,
			headers: [][2]string{
				{"X-Forwarded-For", " 10.0.0.1 , 10.0.0.2"},
			},
		}
		got := req.RemoteAddr()
		if got != "10.0.0.1" {
			t.Errorf("RemoteAddr() = %q, want %q", got, "10.0.0.1")
		}
	})
}

// --- Cookie tests ---

func TestNetpollRequest_Cookie(t *testing.T) {
	t.Run("single cookie", func(t *testing.T) {
		req := &netpollRequest{
			headers: [][2]string{{"Cookie", "session=abc123"}},
		}
		if got := req.Cookie("session"); got != "abc123" {
			t.Errorf("Cookie(session) = %q, want %q", got, "abc123")
		}
	})

	t.Run("multiple cookies", func(t *testing.T) {
		req := &netpollRequest{
			headers: [][2]string{{"Cookie", "a=1; b=2; c=3"}},
		}
		if got := req.Cookie("a"); got != "1" {
			t.Errorf("Cookie(a) = %q, want %q", got, "1")
		}
		if got := req.Cookie("b"); got != "2" {
			t.Errorf("Cookie(b) = %q, want %q", got, "2")
		}
		if got := req.Cookie("c"); got != "3" {
			t.Errorf("Cookie(c) = %q, want %q", got, "3")
		}
	})

	t.Run("missing cookie", func(t *testing.T) {
		req := &netpollRequest{
			headers: [][2]string{{"Cookie", "a=1; b=2"}},
		}
		if got := req.Cookie("missing"); got != "" {
			t.Errorf("Cookie(missing) = %q, want empty", got)
		}
	})

	t.Run("no cookie header", func(t *testing.T) {
		req := &netpollRequest{}
		if got := req.Cookie("any"); got != "" {
			t.Errorf("Cookie(any) = %q, want empty", got)
		}
	})

	t.Run("cookie with equals in value", func(t *testing.T) {
		req := &netpollRequest{
			headers: [][2]string{{"Cookie", "token=abc=def"}},
		}
		if got := req.Cookie("token"); got != "abc=def" {
			t.Errorf("Cookie(token) = %q, want %q", got, "abc=def")
		}
	})
}

// --- RawRequest test ---

func TestNetpollRequest_RawRequest(t *testing.T) {
	req := &netpollRequest{}
	if got := req.RawRequest(); got != nil {
		t.Errorf("RawRequest() = %v, want nil", got)
	}
}

// --- netpollHeaderMap tests ---

func TestNetpollHeaderMap_Set(t *testing.T) {
	m := &netpollHeaderMap{headers: make([][2]string, 0, 4)}

	// Set new header.
	m.Set("Content-Type", "application/json")
	if got := m.Get("Content-Type"); got != "application/json" {
		t.Errorf("Get after Set = %q, want %q", got, "application/json")
	}

	// Replace existing (case-insensitive).
	m.Set("content-type", "text/plain")
	if got := m.Get("Content-Type"); got != "text/plain" {
		t.Errorf("Get after replace = %q, want %q", got, "text/plain")
	}

	// Should not add a duplicate — still one entry.
	count := 0
	for _, h := range m.headers {
		if h[0] == "Content-Type" || h[0] == "content-type" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 content-type header, got %d", count)
	}
}

func TestNetpollHeaderMap_Add(t *testing.T) {
	m := &netpollHeaderMap{headers: make([][2]string, 0, 4)}

	m.Add("Set-Cookie", "a=1")
	m.Add("Set-Cookie", "b=2")

	count := 0
	for _, h := range m.headers {
		if h[0] == "Set-Cookie" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 Set-Cookie headers, got %d", count)
	}
}

func TestNetpollHeaderMap_Get(t *testing.T) {
	m := &netpollHeaderMap{
		headers: [][2]string{
			{"Content-Type", "application/json"},
			{"X-Request-Id", "abc"},
		},
	}

	// Case-insensitive.
	if got := m.Get("content-type"); got != "application/json" {
		t.Errorf("Get(content-type) = %q, want %q", got, "application/json")
	}
	if got := m.Get("X-REQUEST-ID"); got != "abc" {
		t.Errorf("Get(X-REQUEST-ID) = %q, want %q", got, "abc")
	}
	// Missing.
	if got := m.Get("X-Missing"); got != "" {
		t.Errorf("Get(X-Missing) = %q, want empty", got)
	}
}

func TestNetpollHeaderMap_Del(t *testing.T) {
	m := &netpollHeaderMap{
		headers: [][2]string{
			{"Content-Type", "application/json"},
			{"X-Custom", "val1"},
			{"x-custom", "val2"},
			{"Accept", "text/html"},
		},
	}

	// Del removes all matching (case-insensitive).
	m.Del("x-custom")
	for _, h := range m.headers {
		if h[0] == "X-Custom" || h[0] == "x-custom" {
			t.Errorf("expected X-Custom to be deleted, found %v", h)
		}
	}
	// Other headers remain.
	if got := m.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type should remain, got %q", got)
	}
	if got := m.Get("Accept"); got != "text/html" {
		t.Errorf("Accept should remain, got %q", got)
	}
	if len(m.headers) != 2 {
		t.Errorf("expected 2 headers remaining, got %d", len(m.headers))
	}
}

func TestNetpollHeaderMap_Del_NonExistent(t *testing.T) {
	m := &netpollHeaderMap{
		headers: [][2]string{{"Content-Type", "text/plain"}},
	}
	m.Del("X-Missing")
	if len(m.headers) != 1 {
		t.Errorf("expected 1 header, got %d", len(m.headers))
	}
}

// --- Shutdown test ---

func TestNetpollTransport_Shutdown_NoListener(t *testing.T) {
	tr := &NetpollTransport{}
	err := tr.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown on fresh transport = %v, want nil", err)
	}
}

// --- Property-based test ---

func TestPropertyParseRequestLine(t *testing.T) {
	// Property: for any valid method and path, parseRequestLine reconstructs them.
	f := func(methodIdx uint8, pathSuffix string) bool {
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		method := methods[int(methodIdx)%len(methods)]

		// Build a safe path — filter out spaces, ?, \r, \n to keep it valid.
		safePath := "/"
		for _, c := range pathSuffix {
			if c != ' ' && c != '?' && c != '\r' && c != '\n' && c > 0x1f && c < 0x7f {
				safePath += string(c)
			}
		}

		line := method + " " + safePath + " HTTP/1.1\r\n"
		gotMethod, gotPath, gotQuery, ok := parseRequestLine(line)
		if !ok {
			return false
		}
		if gotMethod != method {
			return false
		}
		if gotPath != safePath {
			return false
		}
		if gotQuery != "" {
			return false
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property failed: %v", err)
	}
}

func TestPropertyParseRequestLineWithQuery(t *testing.T) {
	// Property: for any valid method, path, and query, parseRequestLine splits correctly.
	f := func(methodIdx uint8, pathSuffix, querySuffix string) bool {
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		method := methods[int(methodIdx)%len(methods)]

		// Build safe path.
		safePath := "/"
		for _, c := range pathSuffix {
			if c != ' ' && c != '?' && c != '\r' && c != '\n' && c > 0x1f && c < 0x7f {
				safePath += string(c)
			}
		}

		// Build safe query (no spaces, no \r\n).
		safeQuery := ""
		for _, c := range querySuffix {
			if c != ' ' && c != '\r' && c != '\n' && c > 0x1f && c < 0x7f {
				safeQuery += string(c)
			}
		}

		if safeQuery == "" {
			return true // skip empty query — tested in no-query property
		}

		line := method + " " + safePath + "?" + safeQuery + " HTTP/1.1\r\n"
		gotMethod, gotPath, gotQuery, ok := parseRequestLine(line)
		if !ok {
			return false
		}
		if gotMethod != method {
			return false
		}
		if gotPath != safePath {
			return false
		}
		if gotQuery != safeQuery {
			return false
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property failed: %v", err)
	}
}
