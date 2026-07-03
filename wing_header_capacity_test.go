//go:build linux || darwin

package kruda

import (
	"fmt"
	"strings"
	"testing"
)

// chromeWSUpgradeRequest is a realistic Chrome WebSocket upgrade request.
// It carries 12 headers outside Wing's fast-path set (knownHeader), which
// exceeded the former 8-slot inline extras capacity — the 9th..12th extras
// (including Sec-WebSocket-Key) were silently dropped.
func chromeWSUpgradeRequest() string {
	return strings.Join([]string{
		"GET /ws HTTP/1.1",
		"Host: example.com",       // fast path
		"Connection: Upgrade",     // fast path
		"Pragma: no-cache",        // extra 1
		"Cache-Control: no-cache", // extra 2
		"User-Agent: Mozilla/5.0 (X11; Linux x86_64) Chrome/126.0", // extra 3
		"Upgrade: websocket",                           // extra 4
		"Origin: https://example.com",                  // extra 5
		"Sec-WebSocket-Version: 13",                    // extra 6
		"Accept-Encoding: gzip, deflate, br",           // extra 7
		"Accept-Language: th-TH,th;q=0.9,en;q=0.8",     // extra 8
		"Sec-Fetch-Dest: websocket",                    // extra 9
		"Sec-Fetch-Mode: websocket",                    // extra 10
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==",  // extra 11 — the handshake key
		"Sec-WebSocket-Extensions: permessage-deflate", // extra 12
		"", "",
	}, "\r\n")
}

func TestWingHeader_ChromeWSUpgradeRetainsAllHeaders(t *testing.T) {
	req, _, ok := parseHTTPRequest([]byte(chromeWSUpgradeRequest()), noLimits)
	if !ok {
		t.Fatal("parse failed for a valid Chrome WS upgrade request")
	}
	want := map[string]string{
		"Pragma":                   "no-cache",
		"Upgrade":                  "websocket",
		"Origin":                   "https://example.com",
		"Sec-WebSocket-Version":    "13",
		"Sec-Fetch-Dest":           "websocket",
		"Sec-Fetch-Mode":           "websocket",
		"Sec-WebSocket-Key":        "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Extensions": "permessage-deflate",
	}
	for k, v := range want {
		if got := req.Header(k); got != v {
			t.Errorf("Header(%q) = %q, want %q", k, got, v)
		}
	}
}

func TestWingHeader_SpillBeyondInlineCapacity(t *testing.T) {
	var b strings.Builder
	b.WriteString("GET / HTTP/1.1\r\nHost: example.com\r\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "X-Custom-%02d: value-%02d\r\n", i, i)
	}
	b.WriteString("\r\n")
	req, _, ok := parseHTTPRequest([]byte(b.String()), noLimits)
	if !ok {
		t.Fatal("parse failed")
	}
	for i := 0; i < 40; i++ {
		want := fmt.Sprintf("value-%02d", i)
		if got := req.Header(fmt.Sprintf("X-Custom-%02d", i)); got != want {
			t.Errorf("Header(X-Custom-%02d) = %q, want %q", i, got, want)
		}
	}
}

func TestWingHeaderSpills_CounterIncrements(t *testing.T) {
	before := WingHeaderSpills()
	var b strings.Builder
	b.WriteString("GET / HTTP/1.1\r\nHost: example.com\r\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "X-C-%02d: v\r\n", i)
	}
	b.WriteString("\r\n")
	if _, _, ok := parseHTTPRequest([]byte(b.String()), noLimits); !ok {
		t.Fatal("parse failed")
	}
	if got := WingHeaderSpills(); got < before+1 {
		t.Errorf("WingHeaderSpills() = %d, want >= %d", got, before+1)
	}
}
