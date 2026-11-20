//go:build linux || darwin

package kruda

import "testing"

func TestWingRequestHeaderAnyCase(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\nHost: example.com\r\nContent-Type: text/plain\r\nCookie: a=1\r\nAccept: */*\r\nConnection: keep-alive\r\nX-Trace: t1\r\n\r\n")
	r, _, ok := parseHTTPRequest(raw, noLimits)
	if !ok {
		t.Fatal("parse")
	}
	defer releaseRequest(r)
	for _, tc := range []struct{ key, want string }{
		{"HOST", "example.com"}, {"hOsT", "example.com"},
		{"CONTENT-TYPE", "text/plain"}, {"Content-type", "text/plain"},
		{"COOKIE", "a=1"}, {"ACCEPT", "*/*"}, {"CONNECTION", "keep-alive"},
		{"x-trace", "t1"}, {"X-TRACE", "t1"},
	} {
		if got := r.Header(tc.key); got != tc.want {
			t.Errorf("Header(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
