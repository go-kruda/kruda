//go:build linux || darwin

package kruda

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func wingRoundTrip(t *testing.T, addr, target string) string {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n", target); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func wingResponseHeaderLines(raw string) []string {
	head, _, _ := strings.Cut(raw, "\r\n\r\n")
	return strings.Split(head, "\r\n")
}

func TestWingKeepsStatusOnEveryResponseLane(t *testing.T) {
	app := New(Wing())
	status := func(c *Ctx) *Ctx {
		code, err := c.ParamInt("code")
		if err != nil {
			t.Error(err)
		}
		return c.Status(code)
	}
	app.Get("/json/:code", func(c *Ctx) error { return status(c).JSON(Map{"ok": true}) })
	app.Get("/text/:code", func(c *Ctx) error { return status(c).Text("ok") })
	app.Get("/generic/:code", func(c *Ctx) error {
		return status(c).SetHeader("X-Lane", "generic").JSON(Map{"ok": true})
	})
	app.Compile()
	addr, shutdown := startWingServer(t, app)
	defer shutdown()

	want := map[int]string{
		202:  "202 Accepted",
		308:  "308 Permanent Redirect",
		410:  "410 Gone",
		413:  "413 Content Too Large",
		415:  "415 Unsupported Media Type",
		451:  "451 Unavailable For Legal Reasons",
		504:  "504 Gateway Timeout",
		599:  "599 Unknown",
		700:  "700 Unknown",
		0:    "200 OK",
		-1:   "200 OK",
		100:  "200 OK",
		1000: "200 OK",
	}
	for code, text := range want {
		for _, lane := range []string{"json", "text", "generic"} {
			line := wingResponseHeaderLines(wingRoundTrip(t, addr, "/"+lane+"/"+strconv.Itoa(code)))[0]
			if expected := "HTTP/1.1 " + text; line != expected {
				t.Errorf("%s lane, status %d: status line = %q, want %q", lane, code, line, expected)
			}
		}
	}
}

func TestWingKeepsStatusOnSendfileHeaders(t *testing.T) {
	r := acquireResponse()
	defer releaseResponse(r)
	r.WriteHeader(599)
	if line := wingResponseHeaderLines(string(r.buildHeadersOnly()))[0]; line != "HTTP/1.1 599 Unknown" {
		t.Fatalf("status line = %q", line)
	}
}

func TestWingResponseHeadersRejectCRLFInjection(t *testing.T) {
	const injected = "\r\nSet-Cookie: pwn=1"
	app := New(Wing())
	app.Get("/redirect", func(c *Ctx) error { return c.Redirect("/next" + injected) })
	app.Get("/content-type", func(c *Ctx) error {
		return c.SetContentType("text/plain" + injected).SendBytes([]byte("ok"))
	})
	app.Compile()
	addr, shutdown := startWingServer(t, app)
	defer shutdown()

	for _, target := range []string{"/redirect", "/content-type"} {
		for _, line := range wingResponseHeaderLines(wingRoundTrip(t, addr, target)) {
			if strings.HasPrefix(line, "Set-Cookie:") {
				t.Errorf("%s: injected header line %q", target, line)
			}
		}
	}
}

func TestWingRedirectNewlineDoesNotLeaveSite(t *testing.T) {
	app := New(Wing())
	app.Get("/redirect", func(c *Ctx) error { return c.Redirect("/\n/evil.example") })
	app.Compile()
	addr, shutdown := startWingServer(t, app)
	defer shutdown()

	for _, line := range wingResponseHeaderLines(wingRoundTrip(t, addr, "/redirect")) {
		if strings.HasPrefix(line, "Location:") && line != "Location: / /evil.example" {
			t.Fatalf("Location line = %q, want %q", line, "Location: / /evil.example")
		}
	}
}
