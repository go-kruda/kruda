//go:build linux || darwin

package kruda

import "testing"

// Guards the no-body / small-request hot path against allocation regressions
// introduced by body-limit work. parseHTTPRequestFast on a complete in-buffer
// request must not allocate more than the current baseline.
func TestParseFastPath_AllocBaseline(t *testing.T) {
	req := []byte("GET /plaintext HTTP/1.1\r\nHost: h\r\nAccept: text/plain\r\n\r\n")
	limits := parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192}

	allocs := testing.AllocsPerRun(1000, func() {
		r, _, ok := parseHTTPRequestFast(req, limits)
		if !ok {
			t.Fatal("expected ok")
		}
		releaseRequest(r)
	})
	// Baseline measured at commit: 0 allocs/op. Update only with justification.
	const baseline = 0
	if allocs > baseline {
		t.Fatalf("fast-path allocs regressed: got %v want <= %v", allocs, baseline)
	}
}
