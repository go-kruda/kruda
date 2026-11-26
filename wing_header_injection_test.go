//go:build linux || darwin

package kruda

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
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

// TestWingCleanedConnectionCloseEndsConnection sets a raw Connection value
// that becomes "close" once its line break is cleaned. What is sent decides:
// the next pipelined request must not run.
func TestWingCleanedConnectionCloseEndsConnection(t *testing.T) {
	var ranNext atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/next" {
			ranNext.Add(1)
		}
		w.Header().Set("Connection", "close\r")
		_, _ = w.Write([]byte("first"))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1}, handler)
	defer stop()
	conn, br := framingDial(t, addr)
	if _, err := io.WriteString(conn, framingGet("/first")+framingGet("/next")); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	if _, err := br.ReadByte(); err != io.EOF || ranNext.Load() != 0 {
		t.Fatalf("Connection %q sent but kept open: %v, next ran %d times", res.Header.Get("Connection"), err, ranNext.Load())
	}
}

// TestWingCtxHeadersCannotAddFields goes through Ctx, whose fields Wing sends
// unchecked unless raw header access was handed out.
func TestWingCtxHeadersCannotAddFields(t *testing.T) {
	for name, handler := range map[string]HandlerFunc{
		"SetHeader": func(c *Ctx) error {
			c.SetHeader("X-Value", "v\r\nX-Injected: 1")
			return c.SendBytes([]byte("body"))
		},
		"SendBytesWithType": func(c *Ctx) error {
			return c.SendBytesWithType("text/plain\r\nX-Injected: 1", []byte("body"))
		},
		"raw header then Ctx": func(c *Ctx) error {
			c.ResponseWriter().Header().Set("X-Bad\r\nX-Injected", "1")
			c.SetHeader("X-Other", "o")
			return c.SendBytes([]byte("body"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			app := New(Wing())
			app.Get("/", handler)
			addr, stop := startWingApp(t, app)
			defer stop()
			conn, br := framingDial(t, addr)
			if _, err := io.WriteString(conn, framingGet("/")); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(res.Body)
			if res.Header.Get("X-Injected") != "" || string(body) != "body" {
				t.Fatalf("headers %v body %q", res.Header, body)
			}
		})
	}
}

// TestWingRetainedRawHeaderMapStaysChecked keeps the raw header map, switches
// to the string lane, fills the map, and ends with a file stream, which sends
// that map. Its fields must still be checked.
func TestWingRetainedRawHeaderMapStaysChecked(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		h := w.Header()
		w.(transport.StringResponder).SetStringBody(200, "text/plain", "string")
		h.Set("X-Bad\r\nX-Injected", "1")
		w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader("body")), 4)
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
	if res.Header.Get("X-Injected") != "" {
		t.Fatalf("headers %v", res.Header)
	}
}

// TestWingFrameworkHeaderSourcesAreClean puts a line break in a security
// header from the config, which the framework sends through Ctx unchecked.
func TestWingFrameworkHeaderSourcesAreClean(t *testing.T) {
	app := New(Wing(), WithSecureHeaders())
	app.config.Security.XFrameOptions = "DENY\r\nX-Injected: 1"
	app.Get("/", func(c *Ctx) error { return c.SendBytes([]byte("body")) })
	addr, stop := startWingApp(t, app)
	defer stop()
	conn, br := framingDial(t, addr)
	if _, err := io.WriteString(conn, framingGet("/")); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Header.Values("X-Injected"); len(got) != 0 {
		t.Fatalf("security header added X-Injected %q (all: %v)", got, res.Header)
	}
}

// TestWingFileResponseConnectionCloseEndsConnection fills a retained raw
// header map with Connection: close after selecting the string lane, then
// ends with a file stream, which is what gets sent. The close must hold.
func TestWingFileResponseConnectionCloseEndsConnection(t *testing.T) {
	var ranNext atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/next" {
			ranNext.Add(1)
			return
		}
		h := w.Header()
		w.(transport.StringResponder).SetStringBody(200, "text/plain", "string")
		h.Set("Connection", "close")
		w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader("first")), 5)
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1}, handler)
	defer stop()
	conn, br := framingDial(t, addr)
	if _, err := io.WriteString(conn, framingGet("/first")+framingGet("/next")); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	if _, err := br.ReadByte(); err != io.EOF || ranNext.Load() != 0 {
		t.Fatalf("Connection: close sent but kept open: %v, next ran %d times", err, ranNext.Load())
	}
}
