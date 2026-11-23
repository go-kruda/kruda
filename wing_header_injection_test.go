//go:build linux || darwin

package kruda

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// TestWingRawHeadersCannotAddFields sets raw response headers whose name or
// value carries a line break. Neither may start a field of its own: values get
// the line breaks replaced by spaces, and invalid names are dropped, as in
// net/http.
func TestWingRawHeadersCannotAddFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		preset Preset
		file   bool
	}{
		{name: "buffered", preset: Bolt},
		{name: "Pool", preset: Arrow},
		{name: "Takeover", preset: Spear},
		{name: "Stream", preset: Stream},
		{name: "file stream", preset: Bolt, file: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				h := w.Header()
				h.Set("X-Value", "v\r\nX-Injected: 1\n")
				h.Add("X-Bad\r\nX-Injected", "2")
				h.Add("X-Colon: x", "3")
				h.Add("", "4")
				if tc.file {
					w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader("body")), 4)
					return
				}
				_, _ = w.Write([]byte("body"))
			})
			addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: tc.preset}, handler)
			defer stop()
			conn, br := framingDial(t, addr)
			if _, err := io.WriteString(conn, framingGet("/")); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil || string(body) != "body" {
				t.Fatalf("body %q err %v", body, err)
			}
			if got := res.Header.Values("X-Injected"); len(got) != 0 {
				t.Fatalf("raw header added X-Injected: %q (all: %v)", got, res.Header)
			}
			if got := res.Header.Get("X-Value"); got != "v  X-Injected: 1" {
				t.Fatalf("X-Value %q", got)
			}
			if got := res.Header.Get("X-Colon"); got != "" {
				t.Fatalf("invalid header name sent: X-Colon %q", got)
			}
		})
	}
}

// TestWingStringResponderContentTypeCannotAddFields passes a line break in
// the content type of the string fast lane, which Ctx does not stand in front of.
func TestWingStringResponderContentTypeCannotAddFields(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.(transport.StringResponder).SetStringBody(200, "text/plain\r\nX-Injected: 1", "body")
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1}, handler)
	defer stop()
	conn, br := framingDial(t, addr)
	if _, err := io.WriteString(conn, framingGet("/")); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("X-Injected") != "" || res.Header.Get("Content-Type") != "text/plain  X-Injected: 1" {
		t.Fatalf("headers %v", res.Header)
	}
}
