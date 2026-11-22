//go:build linux || darwin

package kruda

import (
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingBodyCeilingWithoutBodyLimit declares more than the parser accepts.
// Without a BodyLimit that must still be refused before any body is buffered.
func TestWingBodyCeilingWithoutBodyLimit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		preset  Preset
		first   string
		replies int
	}{
		{"event loop", Bolt, "", 1},
		{"Takeover", Spear, "GET /db HTTP/1.1\r\nHost: x\r\n\r\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, stop := startWingServerWithConfig(t, WingConfig{
				Workers: 1, DefaultPreset: tc.preset,
			}, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
				_, _ = rw.Write([]byte("ok"))
			}))
			defer stop()
			conn, br := framingDial(t, addr)
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
			post := "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: " + strconv.Itoa(maxContentLength+1) + "\r\n\r\n"
			if _, err := io.WriteString(conn, tc.first+post); err != nil {
				t.Fatal(err)
			}
			var last int
			for range tc.replies {
				res, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatalf("waited for an oversized body: %v", err)
				}
				_, _ = io.Copy(io.Discard, res.Body)
				last = res.StatusCode
			}
			if last != http.StatusRequestEntityTooLarge {
				t.Fatalf("status %d, want 413", last)
			}
		})
	}
}
