//go:build linux || darwin

package kruda

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// BenchmarkPerfWingBodyRetention parses large POSTs then plain GETs through
// the request pool, and reports what the pool still holds after one GC.
func BenchmarkPerfWingBodyRetention(b *testing.B) {
	body := bytes.Repeat([]byte("x"), 256<<10)
	post := append([]byte("POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
	get := []byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	before := baselineHeap()
	b.ReportAllocs()
	for b.Loop() {
		req, _, ok := parseHTTPRequest(post, noLimits)
		if !ok {
			b.Fatal("post")
		}
		releaseRequest(req)
		for range 10 {
			req, _, _ := parseHTTPRequest(get, noLimits)
			releaseRequest(req)
		}
	}
	b.StopTimer()
	retainedKB(b, before)
}

func BenchmarkPerfWingHEADString(b *testing.B) {
	payload := strings.Repeat("x", 1<<20)
	b.ReportAllocs()
	for b.Loop() {
		r := acquireResponse()
		r.headRequest = true
		r.SetStringBody(200, "text/plain", payload)
		out := r.appendStringTo(nil)
		if len(out) > 4096 {
			b.Fatal("body sent")
		}
		releaseResponse(r)
	}
}
