//go:build linux || darwin

package kruda

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"
)

// splitWingResponse parses a serialized HTTP/1.1 response into status line,
// header map, and body so tests compare semantics, not header order.
func splitWingResponse(t *testing.T, raw []byte) (status string, headers map[string]string, body string) {
	t.Helper()
	headEnd := bytes.Index(raw, []byte("\r\n\r\n"))
	if headEnd < 0 {
		t.Fatalf("no header terminator in %q", raw)
	}
	head := string(raw[:headEnd])
	body = string(raw[headEnd+4:])
	lines := strings.Split(head, "\r\n")
	status = lines[0]
	headers = make(map[string]string, len(lines)-1)
	for _, ln := range lines[1:] {
		k, v, ok := strings.Cut(ln, ": ")
		if !ok {
			t.Fatalf("malformed header line %q", ln)
		}
		headers[k] = v
	}
	return status, headers, body
}

func assertFreshDate(t *testing.T, headers map[string]string) {
	t.Helper()
	d, err := time.Parse(time.RFC1123, headers["Date"])
	if err != nil {
		t.Fatalf("Date header %q not RFC1123: %v", headers["Date"], err)
	}
	if age := time.Since(d); age < -2*time.Second || age > 5*time.Second {
		t.Fatalf("Date header stale by %v", age)
	}
}

func TestStringLaneGoldenVsGenericPath(t *testing.T) {
	const ct = "text/html; charset=utf-8"
	const payload = "<h1>fortune</h1><p>A computer scientist is someone who fixes things that aren't broken.</p>"

	lane := acquireResponse()
	lane.SetStringBody(200, ct, payload)
	laneRaw := append([]byte(nil), lane.appendStringTo(nil)...)
	releaseResponse(lane)

	gen := acquireResponse()
	gen.WriteHeader(200)
	gen.Header().Set("Content-Type", ct)
	_, _ = gen.Write([]byte(payload))
	genRaw := append([]byte(nil), gen.buildZeroCopy()...)
	releaseResponse(gen)

	laneStatus, laneHdr, laneBody := splitWingResponse(t, laneRaw)
	genStatus, genHdr, genBody := splitWingResponse(t, genRaw)

	if laneStatus != genStatus {
		t.Fatalf("status line: lane %q vs generic %q", laneStatus, genStatus)
	}
	if laneBody != genBody {
		t.Fatalf("body: lane %q vs generic %q", laneBody, genBody)
	}
	for _, k := range []string{"Content-Type", "Content-Length"} {
		if laneHdr[k] != genHdr[k] {
			t.Fatalf("%s: lane %q vs generic %q", k, laneHdr[k], genHdr[k])
		}
	}
	assertFreshDate(t, laneHdr)
}

func TestStringLaneMultibyteContentLength(t *testing.T) {
	const payload = "こんにちは、クルダ！" // multibyte: rune count != byte count
	r := acquireResponse()
	r.SetStringBody(200, "text/plain; charset=utf-8", payload)
	raw := append([]byte(nil), r.appendStringTo(nil)...)
	releaseResponse(r)

	_, hdr, body := splitWingResponse(t, raw)
	if body != payload {
		t.Fatalf("body roundtrip: %q", body)
	}
	if want := strconv.Itoa(len(payload)); hdr["Content-Length"] != want {
		t.Fatalf("Content-Length = %s, want %s (bytes, not runes)", hdr["Content-Length"], want)
	}
}

func TestStringLaneNon200Status(t *testing.T) {
	r := acquireResponse()
	r.SetStringBody(404, "text/plain; charset=utf-8", "nope")
	raw := append([]byte(nil), r.appendStringTo(nil)...)
	releaseResponse(r)
	status, _, _ := splitWingResponse(t, raw)
	if status != "HTTP/1.1 404 Not Found" {
		t.Fatalf("status = %q", status)
	}

	r2 := acquireResponse()
	r2.SetStringBody(599, "text/plain; charset=utf-8", "odd")
	raw2 := append([]byte(nil), r2.appendStringTo(nil)...)
	releaseResponse(r2)
	status2, _, _ := splitWingResponse(t, raw2)
	if status2 != "HTTP/1.1 599 Unknown" {
		t.Fatalf("fallback status = %q", status2)
	}
}

func TestStringLaneEmptyBody(t *testing.T) {
	r := acquireResponse()
	r.SetStringBody(200, "text/plain; charset=utf-8", "")
	raw := append([]byte(nil), r.appendStringTo(nil)...)
	releaseResponse(r)
	_, hdr, body := splitWingResponse(t, raw)
	if hdr["Content-Length"] != "0" || body != "" {
		t.Fatalf("empty body: CL=%s body=%q", hdr["Content-Length"], body)
	}
}

func TestStringLaneAppendsToPrefix(t *testing.T) {
	r := acquireResponse()
	r.SetStringBody(200, "text/plain; charset=utf-8", "x")
	out := r.appendStringTo([]byte("PREVIOUS"))
	releaseResponse(r)
	if !bytes.HasPrefix(out, []byte("PREVIOUS")) {
		t.Fatalf("appendStringTo must append, got %q", out[:16])
	}
}
