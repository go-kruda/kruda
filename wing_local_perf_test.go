//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"syscall"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

const localWingRequest = "GET /queued HTTP/1.1\r\nHost: localhost\r\n\r\n"

// Keep the socket full so tryParse leaves serialized responses in sendBuf.
// This measures the actual Inline path, including its copy and pool ownership.
func localWingQueuedWorker(tb testing.TB, handler transport.Handler, presets map[string]Preset, capacity int) (*worker, *conn) {
	tb.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		syscall.Close(fds[0])
		syscall.Close(fds[1])
	})
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		tb.Fatal(err)
	}
	padding := make([]byte, 4096)
	for _, size := range []int{len(padding), 1} {
		for {
			_, err := syscall.Write(fds[0], padding[:size])
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				break
			}
			if err != nil {
				tb.Fatal(err)
			}
		}
	}
	w, _ := newTestWorker(0)
	w.handler = handler
	w.presets = NewPresetTable(presets, Bolt)
	c := &conn{
		fd:      int32(fds[0]),
		ctx:     context.Background(),
		readBuf: []byte(localWingRequest),
		sendBuf: make([]byte, 0, capacity),
	}
	w.conns[c.fd] = c
	return w, c
}

func localWingHeaderApp(payload []byte) *App {
	app := New(Wing(), WithSecureHeaders())
	cookies := [2]Cookie{
		{Name: "session", Value: "queued", Path: "/", HTTPOnly: true, Secure: true, SameSite: http.SameSiteStrictMode},
		{Name: "theme", Value: "dark", Path: "/"},
	}
	contentLength := strconv.Itoa(len(payload))
	app.Get("/queued", func(c *Ctx) error {
		c.Status(http.StatusCreated)
		c.SetHeader("X-Request-Id", "local-performance")
		c.SetHeader("Content-Length", contentLength)
		c.AddHeader("Vary", "Origin")
		c.AddHeader("Vary", "Accept-Encoding")
		c.SetCookie(&cookies[0])
		c.SetCookie(&cookies[1])
		return c.SendBytesWithType("application/octet-stream", payload)
	}, Bolt)
	app.Compile()
	return app
}

func TestLocalWingInlineQueuedHeaders(t *testing.T) {
	for _, size := range []int{1024, 8192, 65536} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			payload := bytes.Repeat([]byte("a"), size)
			app := localWingHeaderApp(payload)
			w, c := localWingQueuedWorker(t, app, app.transport.(*Transport).config.Presets, 2*size+2048)
			prefix := []byte("HTTP/1.1 200 OK\r\nContent-Length: 6\r\n\r\nprefix")
			c.sendBuf = append(c.sendBuf, prefix...)
			c.sendN = 9
			c.readN = len(localWingRequest)
			w.tryParse(c)
			first := bytes.Clone(c.sendBuf)

			// A later request reuses response pools while the first stays queued.
			for i := range payload {
				payload[i] = 'b'
			}
			c.readN = len(localWingRequest)
			w.tryParse(c)
			if c.readN != 0 || c.sendN != 9 || !c.keepAlive {
				t.Fatalf("readN=%d sendN=%d keepAlive=%v", c.readN, c.sendN, c.keepAlive)
			}
			if !bytes.HasPrefix(c.sendBuf, first) || !bytes.HasPrefix(first, prefix) {
				t.Fatal("queued bytes changed after a later response reused the pool")
			}
			reader := bufio.NewReader(bytes.NewReader(c.sendBuf[len(prefix):]))
			for _, fill := range []byte{'a', 'b'} {
				resp, err := http.ReadResponse(reader, nil)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != http.StatusCreated || !bytes.Equal(body, bytes.Repeat([]byte{fill}, size)) {
					t.Fatalf("response status=%d body length=%d, want 201 and %d %q bytes", resp.StatusCode, len(body), size, fill)
				}
				for key, want := range map[string]string{
					"Content-Type":           "application/octet-stream",
					"Content-Length":         strconv.Itoa(size),
					"X-Request-Id":           "local-performance",
					"X-Content-Type-Options": "nosniff",
					"X-Frame-Options":        "DENY",
				} {
					if got := resp.Header.Get(key); got != want {
						t.Errorf("%s=%q, want %q", key, got, want)
					}
				}
				cookies := resp.Cookies()
				if len(cookies) != 2 || cookies[0].Name != "session" || cookies[0].Value != "queued" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[1].Name != "theme" || cookies[1].Value != "dark" {
					t.Fatalf("cookies changed: %+v", cookies)
				}
				if len(resp.Header.Values("Vary")) != 2 || resp.Header.Get("Server") != "" {
					t.Fatalf("headers changed: %v", resp.Header)
				}
			}
			if bytes.Count(c.sendBuf[len(prefix):], []byte("\r\nContent-Length:")) != 2 {
				t.Fatal("explicit Content-Length duplicated or omitted")
			}
			if _, err := reader.ReadByte(); err != io.EOF {
				t.Fatalf("unexpected bytes after the pipeline: %v", err)
			}
		})
	}
}

func TestLocalWingInlineQueuedStaticResponse(t *testing.T) {
	static := []byte("HTTP/1.1 202 Accepted\r\nContent-Length: 6\r\nX-Static: yes\r\n\r\nstatic")
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		w.(*wingResponse).SetStaticResponse(static)
	})
	w, c := localWingQueuedWorker(t, handler, nil, 256)
	prefix := []byte("earlier queued response")
	c.sendBuf = append(c.sendBuf, prefix...)
	c.readN = len(localWingRequest)
	w.tryParse(c)
	want := append(bytes.Clone(prefix), static...)
	if !bytes.Equal(c.sendBuf, want) {
		t.Fatalf("queued static response=%q, want %q", c.sendBuf, want)
	}
	if c.readN != 0 || c.sendN != 0 || !c.keepAlive {
		t.Fatalf("readN=%d sendN=%d keepAlive=%v", c.readN, c.sendN, c.keepAlive)
	}
}
