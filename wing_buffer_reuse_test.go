//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func TestParseHTTPRequestOwnedBody(t *testing.T) {
	header := []byte("POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\n")
	body := []byte("hello")
	r, ok := parseHTTPRequestOwnedBody(header, body, noLimits)
	if !ok {
		t.Fatal("valid request rejected")
	}
	got, _ := r.Body()
	if &got[0] != &body[0] {
		t.Fatal("body was copied; the request should own the accumulated buffer")
	}
	releaseRequest(r)

	for name, tc := range map[string]struct {
		header string
		body   string
		limits parserLimits
	}{
		"short body":         {"POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\n", "hell", noLimits},
		"long body":          {"POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\n", "hello!", noLimits},
		"bytes after header": {"POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\nhe", "llo", noLimits},
		"no content-length":  {"POST / HTTP/1.1\r\nHost: x\r\n\r\n", "hello", noLimits},
		"chunked":            {"POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n", "hello", noLimits},
		"over body limit":    {"POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\n", "hello", parserLimits{bodyLimit: 4}},
		"empty body":         {"POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 0\r\n\r\n", "", noLimits},
	} {
		if r, ok := parseHTTPRequestOwnedBody([]byte(tc.header), []byte(tc.body), tc.limits); ok {
			releaseRequest(r)
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestReleaseRequestDropsBody(t *testing.T) {
	r, _, ok := parseHTTPRequest([]byte("POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 3\r\n\r\nabc"), noLimits)
	if !ok {
		t.Fatal("parse")
	}
	releaseRequest(r)
	if r.body != nil {
		t.Fatal("a released request must not keep the previous body")
	}
}

func TestHeadResponseReservesNoBody(t *testing.T) {
	payload := string(bytes.Repeat([]byte("x"), 1<<20))
	r := acquireResponse()
	defer releaseResponse(r)
	r.headRequest = true
	r.SetStringBody(200, "text/plain", payload)
	out := r.appendStringTo(nil)
	if cap(out) > 4096 {
		t.Fatalf("HEAD reserved %d bytes", cap(out))
	}
	if !bytes.Contains(out, []byte("Content-Length: 1048576\r\n")) {
		t.Fatalf("HEAD lost its Content-Length: %q", out)
	}
}

// patterned returns n bytes that differ by position, so a reused buffer that
// leaks bytes from an earlier request or chunk changes the digest.
func patterned(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*31) + seed
	}
	return b
}

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// TestWingBufferReuseKeepAlive sends, on one keep-alive connection per preset,
// large fragmented POST bodies, alternating long and short JSON responses and
// multi-chunk streamed bodies, and checks every byte. It guards the buffers
// that are now reused across requests and chunks.
func TestWingBufferReuseKeepAlive(t *testing.T) {
	streams := map[string][]byte{"a": patterned(100_000, 1), "b": patterned(70_001, 7)}
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		switch r.Path() {
		case "/up":
			got, _ := r.Body()
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(digest(got)))
		case "/stream/a", "/stream/b":
			s := streams[r.Path()[len("/stream/"):]]
			w.Header().Set("Content-Type", "application/octet-stream")
			w.(*wingResponse).SetFileStream(io.NopCloser(bytes.NewReader(s)), int64(len(s)))
		case "/long":
			w.Header().Set("Content-Type", "application/json")
			w.Write(bytes.Repeat([]byte("L"), 5000))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"short":true}`))
		}
	})
	for _, preset := range []struct {
		name string
		p    Preset
	}{{"inline", Bolt}, {"takeover", Spear}} {
		t.Run(preset.name, func(t *testing.T) {
			addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, ReadBufSize: 4096, DefaultPreset: preset.p}, handler)
			defer stop()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
			br := bufio.NewReader(conn)
			do := func(req []byte) []byte {
				t.Helper()
				if _, err := conn.Write(req); err != nil {
					t.Fatal(err)
				}
				resp, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				b, err := io.ReadAll(resp.Body)
				if err != nil || resp.StatusCode != 200 {
					t.Fatalf("status=%d err=%v", resp.StatusCode, err)
				}
				return b
			}
			for i := range 3 {
				for _, size := range []int{300_000, 9_000} {
					body := patterned(size, byte(i))
					req := append([]byte("POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: "+strconv.Itoa(size)+"\r\n\r\n"), body...)
					if got := string(do(req)); got != digest(body) {
						t.Fatalf("POST %d round %d: body corrupted", size, i)
					}
				}
				if got := do([]byte("GET /long HTTP/1.1\r\nHost: x\r\n\r\n")); !bytes.Equal(got, bytes.Repeat([]byte("L"), 5000)) {
					t.Fatal("long response corrupted")
				}
				if got := string(do([]byte("GET /short HTTP/1.1\r\nHost: x\r\n\r\n"))); got != `{"short":true}` {
					t.Fatalf("short response after long = %q", got)
				}
				for _, name := range []string{"a", "b"} {
					if got := do([]byte("GET /stream/" + name + " HTTP/1.1\r\nHost: x\r\n\r\n")); !bytes.Equal(got, streams[name]) {
						t.Fatalf("stream %s round %d corrupted", name, i)
					}
				}
			}
		})
	}
}

// TestWingOwnedBodyPipelinedSurplus sends a fragmented POST with the next
// request in the same write, so the bytes after the body must be parsed as
// the following request rather than lost or kept in the body.
func TestWingOwnedBodyPipelinedSurplus(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		got, _ := r.Body()
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Path() + ":" + digest(got)))
	})
	for _, preset := range []struct {
		name string
		p    Preset
	}{{"inline", Bolt}, {"takeover", Spear}} {
		t.Run(preset.name, func(t *testing.T) {
			addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, ReadBufSize: 4096, DefaultPreset: preset.p}, handler)
			defer stop()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			body := patterned(50_000, 3)
			wire := append([]byte("POST /first HTTP/1.1\r\nHost: x\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
			wire = append(wire, "GET /second HTTP/1.1\r\nHost: x\r\n\r\n"...)
			if _, err := conn.Write(wire); err != nil {
				t.Fatal(err)
			}
			br := bufio.NewReader(conn)
			for _, want := range []string{"/first:" + digest(body), "/second:" + digest(nil)} {
				resp, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatal(err)
				}
				got, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if string(got) != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			}
		})
	}
}
