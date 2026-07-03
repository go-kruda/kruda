//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"testing"
)

// FuzzParserDifferential cross-checks Wing's request parser against
// net/http's http.ReadRequest on identical bytes. Wing is allowed to be
// STRICTER than net/http (rejecting what net/http accepts), but must never
// be LOOSER: accepting a request net/http rejects, or agreeing to accept
// but disagreeing on the body bytes, is the request-smuggling failure
// class (a front proxy and Wing would see two different requests).
func FuzzParserDifferential(f *testing.F) {
	f.Add([]byte("GET / HTTP/1.1\r\nHost: a\r\n\r\n"))
	f.Add([]byte("POST /p HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\n\r\nhello"))
	// obs-fold: folded continuation of Content-Length (RFC 9112 §5.2 forbids
	// obs-fold in requests; a proxy that unfolds sees "5 0").
	f.Add([]byte("POST /p HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\n 0\r\n\r\nhello"))
	// colon-less garbage line inside the header block
	f.Add([]byte("GET / HTTP/1.1\r\nHost: a\r\ngarbage-line-no-colon\r\n\r\n"))
	f.Add([]byte("POST /p HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\nContent-Length: 5\r\n\r\nhello"))
	f.Add([]byte("POST /p HTTP/1.1\r\nHost: a\r\nTransfer-Encoding: chunked\r\nContent-Length: 5\r\n\r\nhello"))

	f.Fuzz(func(t *testing.T, data []byte) {
		wingReq, _, wingOK := parseHTTPRequest(data, noLimits)
		stdReq, stdErr := http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
		if !wingOK {
			return // Wing being stricter is always allowed
		}
		if stdErr != nil {
			t.Errorf("SMUGGLING RISK: Wing accepted a request net/http rejects (%v):\n%q", stdErr, data)
			return
		}
		if wingReq.Method() != stdReq.Method {
			t.Errorf("method disagrees: wing=%q std=%q for %q", wingReq.Method(), stdReq.Method, data)
		}
		stdBody, _ := io.ReadAll(stdReq.Body)
		wingBody, _ := wingReq.Body()
		if !bytes.Equal(wingBody, stdBody) {
			t.Errorf("BODY BOUNDARY disagrees (smuggling class): wing=%q std=%q for %q", wingBody, stdBody, data)
		}
	})
}
