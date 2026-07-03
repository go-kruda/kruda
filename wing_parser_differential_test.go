//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	// whitespace between the field-name and the colon (RFC 9112 §5.1
	// forbids it; net/http rejects it outright, Wing must too).
	f.Add([]byte("POST / HTTP/1.1\r\nHost: a\r\nContent-Length\t: 5\r\n\r\n12345"))
	// obs-fold on a NON-FIRST header line that itself contains a colon: an
	// earlier fix only rejected leading SP/HTAB on the first header line,
	// missing this case — the continuation is mistaken for an ordinary new
	// header ("x: y") instead of an illegal fold of Content-Length's value
	// (RFC 9112 §5.2 forbids obs-fold on every line of a request, not just
	// the first).
	f.Add([]byte("POST /p HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\n x: y\r\n\r\nhello"))
	// invalid percent-escape in the request path — not a framing/smuggling
	// divergence (both parsers agree on where the request ends), just a
	// URL-validity divergence that WithPathTraversal() covers when enabled.
	// See the guard below for why this is intentionally not a failure.
	f.Add([]byte("GET /%zz HTTP/1.1\r\nHost: a\r\n\r\n"))
	// method-interning hash collision: "0.00" is a valid RFC 7230 token that
	// hashes+length-collides with "HEAD" in wingInternMethod's fast lookup;
	// without a byte comparison it was misreported as method HEAD.
	f.Add([]byte("0.00 * HTTP/0.0\r\n\r\n"))
	// duplicate Host header: RFC 7230 §5.4 requires servers to reject a
	// request with more than one Host header; net/http's ReadRequest
	// enforces this ("too many Host headers"). Found by round-3 fuzzing.
	f.Add([]byte("GET / HTTP/1.1\r\nHost: a\r\nHost: b\r\n\r\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		wingReq, _, wingOK := parseHTTPRequest(data, noLimits)
		stdReq, stdErr := http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
		if !wingOK {
			return // Wing being stricter is always allowed
		}
		if stdErr != nil {
			// Known, accepted divergence: net/http validates percent-escapes
			// in the request path (via url.Parse) and Wing does not — this is
			// NOT a smuggling/body-boundary risk, since both parsers agree on
			// request framing (where the request ends); they only disagree on
			// whether the path STRING is well-formed URL syntax. Path-level
			// validation (including percent-escape checks via
			// url.PathUnescape) already exists one layer up in cleanPath()
			// (router.go), gated behind the opt-in WithPathTraversal()/
			// WithSecurity() config — duplicating it into the always-on,
			// per-request byte-level parser would be a scope expansion beyond
			// HTTP/1.1 framing correctness. Guard narrowly: only skip when the
			// net/http error is specifically a URL-parsing/escape error, so a
			// real framing/header divergence that happens to involve a '%'
			// byte is never silently swallowed.
			var urlErr *url.Error
			if errors.As(stdErr, &urlErr) || strings.Contains(stdErr.Error(), "invalid URL escape") {
				return
			}
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

// TestWingParser_RejectsMalformedHeaderLines is the regression test for the
// anti-smuggling fixes to header-line parsing: obs-fold continuations,
// colon-less lines, and whitespace between the field-name and colon must all
// be rejected, not silently accepted or normalized.
func TestWingParser_RejectsMalformedHeaderLines(t *testing.T) {
	cases := []string{
		"POST /p HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\n 0\r\n\r\nhello",
		"GET / HTTP/1.1\r\nHost: a\r\ngarbage\r\n\r\n",
		// whitespace between field-name and colon (RFC 9112 §5.1) — must not
		// be silently trimmed and matched as a known header.
		"POST / HTTP/1.1\r\nHost: a\r\nContent-Length\t: 5\r\n\r\n12345",
		// obs-fold on a NON-FIRST header line that happens to contain a
		// colon — must be rejected on every header line, not just the
		// first (RFC 9112 §5.2). See FuzzParserDifferential's seed comment.
		"POST /p HTTP/1.1\r\nHost: a\r\nContent-Length: 5\r\n x: y\r\n\r\nhello",
	}
	for _, raw := range cases {
		if _, _, ok := parseHTTPRequest([]byte(raw), noLimits); ok {
			t.Errorf("parser accepted a malformed header line: %q", raw)
		}
	}
}

// TestWingParser_MethodInterningNoCollision is the regression test for a
// hash-collision bug in wingInternMethod's fast-path lookup: the XOR/add
// hash used for O(1) interning of common methods is not collision-free, and
// a length-only check after the hash lookup let a distinct method token
// ("0.00", a valid RFC 7230 token) be silently reported as an entirely
// different, unrelated method ("HEAD") that happens to hash+length-collide
// with it. This corrupts Method() for any handler, middleware, or router
// decision that branches on it.
func TestWingParser_MethodInterningNoCollision(t *testing.T) {
	raw := "0.00 * HTTP/0.0\r\n\r\n"
	req, _, ok := parseHTTPRequest([]byte(raw), noLimits)
	if !ok {
		t.Fatalf("parser rejected a well-formed request line: %q", raw)
	}
	if req.Method() != "0.00" {
		t.Errorf("Method() = %q, want %q (must not collide with an interned method string)", req.Method(), "0.00")
	}
}

// TestWingParser_RejectsDuplicateHost is the regression test for a missing
// duplicate-Host check: RFC 7230 §5.4 requires a server to reject any
// request that contains more than one Host header field, and net/http's
// ReadRequest enforces exactly that ("too many Host headers"). Wing had no
// dedup tracking for Host (unlike Content-Length) and silently kept the
// last value, which would let a request-smuggling-style divergence occur
// between Wing and any strict Host-aware intermediary. Found by round-3
// differential fuzzing.
func TestWingParser_RejectsDuplicateHost(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nHost: a\r\nHost: b\r\n\r\n"
	if _, _, ok := parseHTTPRequest([]byte(raw), noLimits); ok {
		t.Errorf("parser accepted a request with duplicate Host headers: %q", raw)
	}
}
