//go:build linux || darwin

package kruda

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
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

	var location string
	for _, line := range wingResponseHeaderLines(wingRoundTrip(t, addr, "/redirect")) {
		if strings.HasPrefix(line, "Location:") {
			location = line
		}
	}
	if location != "Location: / /evil.example" {
		t.Fatalf("Location line = %q, want %q", location, "Location: / /evil.example")
	}
}

func TestWingKeepAliveSurvivesResponsesWithoutBody(t *testing.T) {
	app := New(Wing())
	code := func(c *Ctx) *Ctx {
		n, err := c.ParamInt("code")
		if err != nil {
			t.Error(err)
		}
		return c.Status(n)
	}
	for _, preset := range []struct {
		name string
		opt  RouteOption
	}{{"inline", Bolt}, {"pool", Arrow}, {"takeover", Spear}, {"spawn", Preset{Dispatch: Spawn}}} {
		p := "/" + preset.name
		app.Get(p+"/json/:code", func(c *Ctx) error { return code(c).JSON(Map{"x": 1}) }, preset.opt)
		app.Get(p+"/text/:code", func(c *Ctx) error { return code(c).Text("body") }, preset.opt)
		app.Get(p+"/generic/:code", func(c *Ctx) error {
			return code(c).SetHeader("X-Lane", "generic").Text("body")
		}, preset.opt)
		app.Get(p+"/stream/:code", func(c *Ctx) error { return code(c).Stream(strings.NewReader("body")) }, preset.opt)
		app.Get(p+"/te/:code", func(c *Ctx) error {
			return code(c).SetHeader("Transfer-Encoding", "chunked").Text("body")
		}, preset.opt)
		app.Head(p+"/head", func(c *Ctx) error { return c.Text("body") }, preset.opt)
		app.Head(p+"/head-json", func(c *Ctx) error { return c.JSON(Map{"x": 1}) }, preset.opt)
	}
	app.All("/static", nil, StaticText(200, "text/plain", "staticbody"))
	app.Head("/static-json", nil, StaticJSON(200, `{"x":1}`))
	app.Get("/getonly", func(c *Ctx) error { return c.Text("body") })
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	addr, shutdown := startWingApp(t, app)
	defer shutdown()

	type tc struct {
		method, target string
		status         int
		length         string // Content-Length header, "" when it must be absent
	}
	var cases []tc
	for _, p := range []string{"/inline", "/pool", "/takeover", "/spawn"} {
		for _, lane := range []string{"json", "text", "generic", "stream", "te"} {
			cases = append(cases,
				tc{"GET", p + "/" + lane + "/204", 204, ""},
				tc{"GET", p + "/" + lane + "/304", 304, ""})
		}
		cases = append(cases,
			tc{"HEAD", p + "/head", 200, "4"},
			tc{"HEAD", p + "/head-json", 200, "7"})
	}
	cases = append(cases,
		tc{"HEAD", "/static", 200, "10"},
		tc{"HACK", "/static", 405, ""},
		tc{"HEAD", "/static-json", 200, "7"},
		tc{"HEAD", "/getonly", 405, ""})

	for _, c := range cases {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(conn, "%s %s HTTP/1.1\r\nHost: test\r\n\r\nGET /next HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n", c.method, c.target)
			br := bufio.NewReader(conn)

			first, err := http.ReadResponse(br, &http.Request{Method: c.method})
			if err != nil {
				t.Fatal(err)
			}
			first.Body.Close()
			if first.StatusCode != c.status {
				t.Fatalf("status = %d, want %d", first.StatusCode, c.status)
			}
			if c.length != "" || first.StatusCode == 204 || first.StatusCode == 304 {
				if got := first.Header.Get("Content-Length"); got != c.length {
					t.Errorf("Content-Length = %q, want %q", got, c.length)
				}
			}
			if first.TransferEncoding != nil || first.Header.Get("Transfer-Encoding") != "" {
				t.Errorf("Transfer-Encoding = %v, want none", first.TransferEncoding)
			}

			second, err := http.ReadResponse(br, &http.Request{Method: "GET"})
			if err != nil {
				t.Fatalf("next response on the same connection: %v", err)
			}
			body, _ := io.ReadAll(second.Body)
			second.Body.Close()
			if string(body) != "next" {
				t.Fatalf("next body = %q, want %q", body, "next")
			}
		})
	}
}

func TestWingStreamPresetSendsNoBodyAfterHeadOr204(t *testing.T) {
	app := New(Wing())
	app.All("/stream/:code", func(c *Ctx) error {
		n, err := c.ParamInt("code")
		if err != nil {
			t.Error(err)
		}
		return c.Status(n).Stream(strings.NewReader("streambody"))
	}, Stream)
	addr, shutdown := startWingApp(t, app)
	defer shutdown()

	for _, c := range []struct {
		method, target string
		status         int
	}{
		{"HEAD", "/stream/200", 200},
		{"GET", "/stream/204", 204},
		{"GET", "/stream/304", 304},
	} {
		t.Run(c.method+" "+c.target, func(t *testing.T) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(conn, "%s %s HTTP/1.1\r\nHost: test\r\n\r\n", c.method, c.target)
			br := bufio.NewReader(conn)
			resp, err := http.ReadResponse(br, &http.Request{Method: c.method})
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != c.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, c.status)
			}
			if !resp.Close {
				t.Error("response does not say Connection: close")
			}
			if rest, _ := io.ReadAll(br); len(rest) != 0 {
				t.Errorf("bytes after the headers = %q, want none", rest)
			}
		})
	}
}

// headThenNext pipelines first and a GET /next on one connection, then checks
// that the first response is headers only and that /next arrives intact.
func headThenNext(t *testing.T, addr, first, wantLength string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, first+"GET /next HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Content-Length"); got != wantLength {
		t.Errorf("Content-Length = %q, want %q", got, wantLength)
	}
	if resp.Close {
		// The server closes after this response, so nothing may follow it.
		if rest, _ := io.ReadAll(br); len(rest) != 0 {
			t.Errorf("bytes after the headers = %q, want none", rest)
		}
		return
	}
	next, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		t.Fatalf("next response on the same connection: %v", err)
	}
	body, _ := io.ReadAll(next.Body)
	next.Body.Close()
	if string(body) != "next" {
		t.Fatalf("next body = %q, want %q", body, "next")
	}
}

const headReq = "HEAD /head HTTP/1.1\r\nHost: test\r\n\r\n"

func TestWingHeadOnFallbackDispatchPaths(t *testing.T) {
	text := func(c *Ctx) error { return c.Text("body") }
	next := func(c *Ctx) error { return c.Text("next") }

	t.Run("accumulated body", func(t *testing.T) {
		app := New(Wing())
		app.Head("/head", text)
		app.Get("/next", next)
		addr, stop := startWingApp(t, app)
		defer stop()
		body := strings.Repeat("x", 64<<10)
		headThenNext(t, addr, fmt.Sprintf("HEAD /head HTTP/1.1\r\nHost: test\r\nContent-Length: %d\r\n\r\n%s", len(body), body), "4")
	})

	t.Run("unknown dispatch without pool", func(t *testing.T) {
		app := New(Wing())
		app.Head("/head", text, Preset{Dispatch: DispatchMode(99)})
		app.Get("/next", next)
		addr, stop := startWingApp(t, app)
		defer stop()
		headThenNext(t, addr, headReq, "4")
	})

	t.Run("pool saturated", func(t *testing.T) {
		app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 1})))
		entered, release := make(chan struct{}, 8), make(chan struct{})
		app.Get("/block", func(c *Ctx) error {
			entered <- struct{}{}
			<-release
			return c.Text("ok")
		}, Arrow)
		app.Head("/head", text, Arrow)
		app.Get("/next", next)
		addr, stop := startWingApp(t, app)
		defer stop()
		defer close(release)
		// One request runs in the single pool goroutine and two fill its queue
		// (capacity 2), so the HEAD request is served inline.
		for i := 0; i < 3; i++ {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if _, err := io.WriteString(conn, "GET /block HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				<-entered
			}
		}
		time.Sleep(100 * time.Millisecond)
		headThenNext(t, addr, headReq, "4")
	})

	t.Run("hijack route that does not hijack", func(t *testing.T) {
		app := New(Wing())
		app.Head("/head", text, Hijack)
		app.Get("/next", next)
		addr, stop := startWingApp(t, app)
		defer stop()
		headThenNext(t, addr, headReq, "4")
	})

	t.Run("handler sets a static response", func(t *testing.T) {
		app := New(Wing())
		app.Head("/head", func(c *Ctx) error {
			sr, ok := c.ResponseWriter().(transport.StaticResponder)
			if !ok {
				t.Skip("writer is not a StaticResponder")
			}
			sr.SetStaticResponse(transport.GetStaticResponseString(200, "text/plain", "staticbody"))
			return nil
		})
		app.Get("/next", next)
		addr, stop := startWingApp(t, app)
		defer stop()
		headThenNext(t, addr, headReq, "10")
	})
}

func TestWingDropsHandlerTransferEncoding(t *testing.T) {
	app := New(Wing())
	app.Get("/ctx", func(c *Ctx) error {
		return c.SetHeader("Transfer-Encoding", "chunked").SetHeader("X-Lane", "generic").Text("body")
	})
	app.Get("/raw", func(c *Ctx) error {
		w := c.ResponseWriter()
		w.Header().Set("transfer-encoding", "chunked")
		w.Header().Set("content-length", "4")
		w.WriteHeader(200)
		_, err := w.Write([]byte("body"))
		return err
	})
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	addr, stop := startWingApp(t, app)
	defer stop()

	for _, p := range []string{"/ctx", "/raw"} {
		t.Run(p, func(t *testing.T) {
			raw := wingPipeline(t, addr, "GET "+p+" HTTP/1.1\r\nHost: test\r\n\r\n"+nextReq)
			wire := strings.ToLower(string(raw))
			if strings.Contains(wire, "transfer-encoding") {
				t.Errorf("Transfer-Encoding sent:\n%s", raw)
			}
			if n := strings.Count(wire, "content-length:"); n != 2 {
				t.Errorf("%d Content-Length headers in two responses, want 2:\n%s", n, raw)
			}
			if !strings.HasSuffix(string(raw), "\r\n\r\nnext") {
				t.Errorf("next response not intact:\n%s", raw)
			}
		})
	}
}

const nextReq = "GET /next HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"

// wingPipeline writes reqs on one connection and returns every byte the
// server sends until it closes.
func wingPipeline(t *testing.T, addr, reqs string) string {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, reqs); err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(conn)
	return string(raw)
}

func TestWingResponseHeaderNamesIgnoreCase(t *testing.T) {
	app := New(Wing())
	app.Get("/length", func(c *Ctx) error {
		c.ResponseWriter().Header().Set("content-length", "10")
		return c.SetHeader("X-Lane", "generic").Text("body")
	})
	app.Get("/del", func(c *Ctx) error {
		h := c.ResponseWriter().Header()
		h.Set("x-drop", "1")
		h.Del("X-Drop")
		return c.SetHeader("X-Lane", "generic").Text("body")
	})
	app.Get("/stream", func(c *Ctx) error {
		c.ResponseWriter().Header().Set("connection", "keep-alive")
		return c.Stream(strings.NewReader("body"))
	}, Stream)
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	addr, stop := startWingApp(t, app)
	defer stop()

	t.Run("content-length", func(t *testing.T) {
		raw := wingPipeline(t, addr, "GET /length HTTP/1.1\r\nHost: test\r\n\r\n"+nextReq)
		first, _, _ := strings.Cut(strings.ToLower(raw), "\r\n\r\n")
		first += "\r\n"
		if n := strings.Count(first, "content-length:"); n != 1 || !strings.Contains(first, "content-length: 4\r\n") {
			t.Errorf("want one Content-Length of 4:\n%s", raw)
		}
		if !strings.HasSuffix(raw, "\r\n\r\nnext") {
			t.Errorf("next response not intact:\n%s", raw)
		}
	})
	t.Run("del", func(t *testing.T) {
		if raw := wingRoundTrip(t, addr, "/del"); strings.Contains(strings.ToLower(raw), "x-drop") {
			t.Errorf("deleted header sent:\n%s", raw)
		}
	})
	t.Run("stream connection", func(t *testing.T) {
		wire := strings.ToLower(wingRoundTrip(t, addr, "/stream"))
		if strings.Count(wire, "connection:") != 1 || !strings.Contains(wire, "connection: close\r\n") {
			t.Errorf("want one Connection: close:\n%s", wire)
		}
	})
}

func TestWingDefaultStaticPresetSendsNoBodyAfterHead(t *testing.T) {
	static := transport.GetStaticResponseString(200, "text/plain", "defaultbody")
	app := New(WithTransport(NewWingTransport(WingConfig{DefaultPreset: Bolt.With(Static(static))})))
	app.Get("/x", func(c *Ctx) error { return c.Text("x") })
	addr, stop := startWingApp(t, app)
	defer stop()

	raw := wingPipeline(t, addr, "HEAD /x HTTP/1.1\r\nHost: test\r\n\r\nGET /x HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
	head, get, _ := strings.Cut(raw, "\r\n\r\n")
	if !strings.Contains(head, "Content-Length: 11") {
		t.Errorf("HEAD headers = %q, want Content-Length: 11", head)
	}
	if !strings.HasPrefix(get, "HTTP/1.1 200 OK\r\n") || !strings.HasSuffix(get, "\r\n\r\ndefaultbody") {
		t.Errorf("after HEAD = %q, want the GET response", get)
	}
}

// TestWingFileResponsesKeepFraming sends sendfile and file-stream responses on
// every dispatch mode, then a pipelined GET /next on the same connection.
func TestWingFileResponsesKeepFraming(t *testing.T) {
	dir := t.TempDir()
	path, other := filepath.Join(dir, "f"), filepath.Join(dir, "other")
	if err := os.WriteFile(path, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("XXXX"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []*os.File
	t.Cleanup(func() {
		for _, f := range held {
			f.Close()
		}
	})
	sendfile := func(status int, twice bool) HandlerFunc {
		return func(c *Ctx) error {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			fd, err := syscall.Dup(int(f.Fd()))
			if err != nil {
				return err
			}
			fs := c.ResponseWriter().(transport.FileSender)
			fs.SetSendFile(int32(fd), 4)
			if twice {
				fs.SetSendFile(int32(fd), 4)
				// Takes the fd number if the response let go of it.
				g, err := os.Open(other)
				if err != nil {
					return err
				}
				mu.Lock()
				held = append(held, g)
				mu.Unlock()
			}
			c.ResponseWriter().WriteHeader(status)
			return nil
		}
	}
	stream := func(c *Ctx) error {
		w := c.ResponseWriter()
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(200)
		w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader("body")), 4)
		return nil
	}
	app := New(Wing())
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	dispatch := []struct {
		name string
		opts []RouteOption
	}{
		{"inline", nil},
		{"pool", []RouteOption{Arrow}},
		{"spawn", []RouteOption{Preset{Dispatch: Spawn}}},
		{"takeover", []RouteOption{Spear}},
	}
	for _, d := range dispatch {
		app.All("/sendfile-"+d.name, sendfile(200, false), d.opts...)
		app.Get("/sendfile204-"+d.name, sendfile(204, false), d.opts...)
		app.Get("/sendfile-twice-"+d.name, sendfile(200, true), d.opts...)
		app.All("/stream-"+d.name, stream, d.opts...)
	}
	addr, stop := startWingApp(t, app)
	defer stop()

	// Larger than the 8 KB read buffer, so the request takes the accumulated path.
	big := strings.Repeat("a", 64<<10)
	for _, d := range dispatch {
		for _, c := range []struct{ name, req, method, length, body string }{
			{"HEAD sendfile", "HEAD /sendfile-" + d.name, "HEAD", "4", ""},
			{"GET sendfile", "GET /sendfile-" + d.name, "GET", "4", "body"},
			{"GET sendfile 204", "GET /sendfile204-" + d.name, "GET", "", ""},
			{"GET sendfile twice", "GET /sendfile-twice-" + d.name, "GET", "4", "body"},
			{"POST sendfile big body", "POST /sendfile-" + d.name, "POST", "4", "body"},
			{"HEAD stream", "HEAD /stream-" + d.name, "HEAD", "4", ""},
			{"GET stream", "GET /stream-" + d.name, "GET", "4", "body"},
		} {
			t.Run(d.name+" "+c.name, func(t *testing.T) {
				req := c.req + " HTTP/1.1\r\nHost: test\r\n\r\n"
				if c.method == "POST" {
					req = c.req + " HTTP/1.1\r\nHost: test\r\nContent-Length: " + strconv.Itoa(len(big)) + "\r\n\r\n" + big
				}
				fileThenNext(t, addr, req, c.method, c.length, c.body)
			})
		}
	}

	// A body that arrives after the headers also takes the accumulated path.
	for _, expect := range []bool{false, true} {
		name := "inline POST sendfile split body"
		if expect {
			name = "inline POST sendfile 100-continue"
		}
		t.Run(name, func(t *testing.T) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			hdr := "POST /sendfile-inline HTTP/1.1\r\nHost: test\r\nContent-Length: 10\r\n"
			if expect {
				hdr += "Expect: 100-continue\r\n"
			}
			if _, err := io.WriteString(conn, hdr+"\r\n"); err != nil {
				t.Fatal(err)
			}
			br := bufio.NewReader(conn)
			if expect {
				resp, err := http.ReadResponse(br, &http.Request{Method: "POST"})
				if err != nil || resp.StatusCode != 100 {
					t.Fatalf("interim response = %v, %v, want 100 Continue", resp, err)
				}
			} else {
				time.Sleep(50 * time.Millisecond)
			}
			if _, err := io.WriteString(conn, "0123456789"+nextReq); err != nil {
				t.Fatal(err)
			}
			readFileThenNext(t, br, "POST", "4", "body")
		})
	}
}

// fileThenNext pipelines req and a GET /next on one connection, then checks the
// first response's Content-Length and body and that /next arrives intact.
func fileThenNext(t *testing.T, addr, req, method, wantLength, wantBody string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, req+nextReq); err != nil {
		t.Fatal(err)
	}
	readFileThenNext(t, bufio.NewReader(conn), method, wantLength, wantBody)
}

func readFileThenNext(t *testing.T, br *bufio.Reader, method, wantLength, wantBody string) {
	t.Helper()
	resp, err := http.ReadResponse(br, &http.Request{Method: method})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if got := resp.Header.Get("Content-Length"); got != wantLength || string(body) != wantBody {
		t.Errorf("Content-Length %q body %q, want %q %q", got, body, wantLength, wantBody)
	}
	if resp.Close {
		if rest, _ := io.ReadAll(br); len(rest) != 0 {
			t.Errorf("bytes after the response = %q, want none", rest)
		}
		return
	}
	next, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		t.Fatalf("next response on the same connection: %v", err)
	}
	nb, _ := io.ReadAll(next.Body)
	next.Body.Close()
	if string(nb) != "next" {
		t.Errorf("next body = %q, want %q", nb, "next")
	}
}

type closeRecorder struct {
	io.Reader
	closed chan struct{}
}

func (c *closeRecorder) Close() error { close(c.closed); return nil }

func TestWingHeadDropsFileBody(t *testing.T) {
	r := acquireResponse()
	defer releaseResponse(r)
	r.headRequest = true
	file := &closeRecorder{Reader: strings.NewReader("body"), closed: make(chan struct{})}
	r.SetFileStream(file, 4)

	head := string(r.buildZeroCopy())
	if !strings.Contains(head, "Content-Length: 4\r\n") || !strings.HasSuffix(head, "\r\n\r\n") {
		t.Fatalf("headers = %q, want Content-Length: 4 and no body", head)
	}
	if r.fileStream != nil {
		t.Fatal("HEAD response kept its file body")
	}
	select {
	case <-file.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("HEAD response did not close its file body")
	}
}

// TestWingStaticRouteAcceptsRequestBody covers static presets on every
// dispatch mode, with and without a body that arrives after the headers.
func TestWingStaticRouteAcceptsRequestBody(t *testing.T) {
	static := transport.GetStaticResponseString(200, "text/plain", "staticbody")
	handler := func(c *Ctx) error { return c.Text("handler") }
	app := New(Wing())
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	app.All("/inline", nil, StaticText(200, "text/plain", "staticbody"))
	app.All("/pool", nil, Arrow.With(Static(static)))
	app.All("/spawn", nil, Preset{Dispatch: Spawn}.With(Static(static)))
	app.All("/takeover", nil, Spear.With(Static(static)))
	app.All("/with-handler", handler, StaticText(200, "text/plain", "staticbody"))
	addr, stop := startWingApp(t, app)
	defer stop()

	big := strings.Repeat("a", 64<<10)
	for _, p := range []string{"/inline", "/pool", "/spawn", "/takeover", "/with-handler"} {
		bare := " " + p + " HTTP/1.1\r\nHost: test\r\n\r\n"
		withBody := " " + p + " HTTP/1.1\r\nHost: test\r\nContent-Length: " + strconv.Itoa(len(big)) + "\r\n\r\n" + big
		t.Run("GET "+p, func(t *testing.T) { fileThenNext(t, addr, "GET"+bare, "GET", "10", "staticbody") })
		t.Run("POST body "+p, func(t *testing.T) { fileThenNext(t, addr, "POST"+withBody, "POST", "10", "staticbody") })
		t.Run("HEAD body "+p, func(t *testing.T) { fileThenNext(t, addr, "HEAD"+withBody, "HEAD", "10", "") })
	}
}

func TestWingStaticResponseReplacesFileBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	static := transport.GetStaticResponseString(200, "text/plain", "static")
	h := func(c *Ctx) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fd, err := syscall.Dup(int(f.Fd()))
		if err != nil {
			return err
		}
		w := c.ResponseWriter()
		w.(transport.FileSender).SetSendFile(int32(fd), 4)
		w.(transport.StaticResponder).SetStaticResponse(static)
		return nil
	}
	stream := func(c *Ctx) error {
		w := c.ResponseWriter()
		w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader("file")), 4)
		w.(transport.StaticResponder).SetStaticResponse(static)
		return nil
	}
	app := New(Wing())
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	var paths []string
	for _, d := range []struct {
		name string
		opts []RouteOption
	}{
		{"inline", nil},
		{"pool", []RouteOption{Arrow}},
		{"spawn", []RouteOption{Preset{Dispatch: Spawn}}},
		{"takeover", []RouteOption{Spear}},
	} {
		app.All("/sendfile-"+d.name, h, d.opts...)
		app.All("/stream-"+d.name, stream, d.opts...)
		paths = append(paths, "/sendfile-"+d.name, "/stream-"+d.name)
	}
	addr, stop := startWingApp(t, app)
	defer stop()

	for _, p := range paths {
		t.Run("GET "+p, func(t *testing.T) {
			fileThenNext(t, addr, "GET "+p+" HTTP/1.1\r\nHost: test\r\n\r\n", "GET", "6", "static")
		})
		t.Run("HEAD "+p, func(t *testing.T) {
			fileThenNext(t, addr, "HEAD "+p+" HTTP/1.1\r\nHost: test\r\n\r\n", "HEAD", "6", "")
		})
	}
}

func TestWingSendsRealLengthOverHandlerContentLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(Wing())
	app.Get("/next", func(c *Ctx) error { return c.Text("next") })
	raw := func(c *Ctx) error {
		w := c.ResponseWriter()
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(200)
		_, err := w.Write([]byte("body"))
		return err
	}
	app.All("/raw", raw)
	app.All("/raw-pool", raw, Arrow)
	app.Head("/head-only", func(c *Ctx) error {
		w := c.ResponseWriter()
		w.Header().Set("Content-Length", "1234")
		w.WriteHeader(200)
		return nil
	})
	app.Get("/sendfile", func(c *Ctx) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fd, err := syscall.Dup(int(f.Fd()))
		if err != nil {
			return err
		}
		w := c.ResponseWriter()
		w.Header().Set("Content-Length", "99")
		w.(transport.FileSender).SetSendFile(int32(fd), 4)
		w.WriteHeader(200)
		return nil
	})
	addr, stop := startWingApp(t, app)
	defer stop()

	for _, c := range []struct{ req, method, length, body string }{
		{"GET /raw", "GET", "4", "body"},
		{"GET /raw-pool", "GET", "4", "body"},
		{"HEAD /raw", "HEAD", "4", ""},
		{"HEAD /head-only", "HEAD", "1234", ""},
		{"GET /sendfile", "GET", "4", "file"},
	} {
		t.Run(c.req, func(t *testing.T) {
			fileThenNext(t, addr, c.req+" HTTP/1.1\r\nHost: test\r\n\r\n", c.method, c.length, c.body)
		})
	}
}
