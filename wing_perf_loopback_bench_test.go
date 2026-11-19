//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// BenchmarkPerfLoopback measures a whole request on one keep-alive loopback
// connection, server included, for paths whose buffers changed: a large POST
// body that arrives across many reads, a JSON response on Takeover, and a
// streamed (non-sendfile) body.
func BenchmarkPerfLoopback(b *testing.B) {
	if testing.Short() {
		b.Skip("loopback")
	}
	body := bytes.Repeat([]byte("x"), 256<<10)
	post := append([]byte("POST /up HTTP/1.1\r\nHost: x\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
	get := []byte("GET /j HTTP/1.1\r\nHost: x\r\n\r\n")
	stream := bytes.Repeat([]byte("s"), 1<<20)
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		switch r.Path() {
		case "/up":
			got, _ := r.Body()
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(strconv.Itoa(len(got))))
		case "/stream":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.(*wingResponse).SetFileStream(io.NopCloser(bytes.NewReader(stream)), int64(len(stream)))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"Hello, World!","items":[1,2,3,4,5,6,7,8]}`))
		}
	})
	for _, tc := range []struct {
		name   string
		preset Preset
		req    []byte
	}{
		{"post256K-inline", Bolt, post},
		{"post256K-takeover", Spear, post},
		{"json-takeover", Spear, get},
		{"stream1M-inline", Bolt, []byte("GET /stream HTTP/1.1\r\nHost: x\r\n\r\n")},
	} {
		b.Run(tc.name, func(b *testing.B) {
			addr, cleanup := startBenchTransport(b, handler, WingConfig{Workers: 1, RingSize: 64, ReadBufSize: 4096, DefaultPreset: tc.preset})
			defer cleanup()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				b.Fatal(err)
			}
			defer conn.Close()
			br := bufio.NewReaderSize(conn, 64<<10)
			roundTrip := func() {
				if _, err := conn.Write(tc.req); err != nil {
					b.Fatal(err)
				}
				resp, err := http.ReadResponse(br, nil)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := io.Copy(io.Discard, resp.Body); err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != 200 {
					b.Fatal(resp.StatusCode)
				}
			}
			roundTrip()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				roundTrip()
			}
		})
	}
}
