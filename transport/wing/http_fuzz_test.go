package wing

import (
	"strings"
	"testing"
)

// FuzzParseHTTPRequest fuzzes the Wing HTTP parser with arbitrary byte slices.
// The parser must never panic — it either returns a valid request or false.
//
// Validates: Requirements R5.1-R5.6
func FuzzParseHTTPRequest(f *testing.F) {
	// Seed corpus (R5.4)
	f.Add([]byte("GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"))                                  // valid GET
	f.Add([]byte("POST /api HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello"))                            // valid POST+body
	f.Add([]byte("POST / HTTP/1.1\r\nContent-Length: 99999999999\r\n\r\n"))                          // oversized CL
	f.Add([]byte("GET / HTTP/1.1\r\nHost: x\r\n"))                                                   // missing CRLF terminator
	f.Add([]byte("GET / HTTP/1.1\r\nX-Evil: foo\rbar\r\n\r\n"))                                      // CRLF injection
	f.Add([]byte("POST / HTTP/1.1\r\nContent-Length: 5\r\nContent-Length: 5\r\n\r\nhello"))          // duplicate CL
	f.Add([]byte("POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\nContent-Length: 5\r\n\r\nhello")) // TE+CL
	f.Add([]byte(""))                                                                                // empty
	f.Add([]byte("GET / HTTP/1.1\r\n"))                                                              // request line only

	f.Fuzz(func(t *testing.T, data []byte) {
		// R5.2: fuzz with arbitrary []byte, using noLimits for max coverage
		req, ok := parseHTTPRequest(data, noLimits)

		// R5.3: if parse fails, that's fine — just must not panic
		if !ok {
			return
		}

		// R5.6: if parse succeeds, validate invariants
		if req.Method() == "" {
			t.Error("parsed request has empty Method")
		}
		if !strings.HasPrefix(req.Path(), "/") && req.Path() != "*" {
			t.Errorf("parsed request Path = %q, want prefix '/' or '*'", req.Path())
		}
	})
}
