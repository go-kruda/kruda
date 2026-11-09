//go:build linux || darwin

package kruda

import "testing"

func TestWingForwardedHeaderCounts(t *testing.T) {
	parsers := []struct {
		name  string
		parse func([]byte, parserLimits) (*wingRequest, int, bool)
	}{{"safe", parseHTTPRequest}, {"fast", parseHTTPRequestFast}}
	for _, p := range parsers {
		for _, body := range []struct{ name, suffix string }{{"without-body", "\r\n"}, {"with-body", "Content-Length: 1\r\n\r\nx"}} {
			t.Run(p.name+"/"+body.name, func(t *testing.T) {
				raw := "POST / HTTP/1.1\r\nHost: example.com\r\nX-Forwarded-For:\r\nx-forwarded-for: 192.0.2.1\r\nX-Real-IP: 192.0.2.2\r\nx-real-ip: 192.0.2.3\r\n" + body.suffix
				req, _, ok := p.parse([]byte(raw), parserLimits{})
				if !ok {
					t.Fatal("parse failed")
				}
				a, n, b, m := req.ForwardedHeaders()
				if a != "" || n != 2 || b != "192.0.2.2" || m != 2 {
					t.Fatalf("headers: %q %d %q %d", a, n, b, m)
				}
				releaseRequest(req)
				if req.forwardedForCount != 0 || req.realIPCount != 0 {
					t.Fatal("pool did not reset counts")
				}
			})
		}
	}
}
