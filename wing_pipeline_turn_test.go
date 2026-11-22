//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func paddedGet(size int) string {
	head := "GET / HTTP/1.1\r\nHost: x\r\nX-Pad: "
	return head + strings.Repeat("a", size-len(head)-4) + "\r\n\r\n"
}

// socketWorker returns a worker that owns one end of a socketpair.
func socketWorker(t *testing.T, readSize int, h transport.Handler) (*worker, *conn, net.Conn, *mockEngine) {
	t.Helper()
	server, client := newSocketpairFiles(t)
	t.Cleanup(func() { server.Close() })
	fd := int(server.Fd())
	for _, opt := range []int{syscall.SO_SNDBUF, syscall.SO_RCVBUF} {
		for _, s := range []int{fd, int(client.Fd())} {
			if err := syscall.SetsockoptInt(s, syscall.SOL_SOCKET, opt, 1<<20); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := syscall.SetNonblock(fd, true); err != nil {
		t.Fatal(err)
	}
	peer, err := net.FileConn(client)
	client.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.Close() })
	if err := peer.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	w, eng := newTestWorker(0)
	w.config.ReadBufSize = readSize
	w.handler = h
	w.presets = NewPresetTable(nil, Bolt)
	c := newTestConn(int32(fd))
	c.readBuf = make([]byte, readSize)
	c.ctx = context.Background()
	w.conns[c.fd] = c
	return w, c, peer, eng
}

// TestWingPipelineTurnIsBounded keeps the socket readable and the client
// reading. One readiness event must neither nest without end nor keep the loop.
func TestWingPipelineTurnIsBounded(t *testing.T) {
	for _, size := range []int{
		256, // ends on a read boundary: the read after a completed send
		300, // leaves part of a header: the refill, which only Linux does
	} {
		var peer net.Conn
		pcs := make([]uintptr, 4096)
		calls, deepest := 0, 0
		h := transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
			for t0 := time.Now(); time.Since(t0) < 10*time.Microsecond; {
			}
			deepest = max(deepest, runtime.Callers(0, pcs))
			if calls++; calls == 2000 {
				peer.Close() // it would never return
			}
			_, _ = rw.Write([]byte("ok"))
		})
		w, c, p, eng := socketWorker(t, 512, h)
		peer = p
		batch := strings.Repeat(paddedGet(size), 1000)
		go func() {
			for {
				if _, err := io.WriteString(p, batch); err != nil {
					return
				}
			}
		}()
		go func() { _, _ = io.CopyBuffer(io.Discard, p, make([]byte, 1<<20)) }()
		time.Sleep(50 * time.Millisecond)
		w.handleRecv(event{Op: opRecv, Fd: c.fd})
		if deepest > 200 || calls >= 2000 {
			t.Fatalf("size %d: one event ran %d requests %d frames deep", size, calls, deepest)
		}
		if size == 256 {
			if eng.sendArmed == 0 {
				t.Fatalf("size %d: turn ended after %d requests with no wakeup", size, calls)
			}
			before := calls
			w.handleSend(event{Op: opSend, Fd: c.fd})
			if calls == before {
				t.Fatalf("size %d: the wakeup did not resume parsing", size)
			}
		}
		p.Close()
	}
}

// TestWingPipelineFloodSharesWorker floods one connection of a single worker
// and expects another connection to be served meanwhile.
func TestWingPipelineFloodSharesWorker(t *testing.T) {
	h := transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
		for t0 := time.Now(); time.Since(t0) < 20*time.Microsecond; {
		}
		_, _ = rw.Write([]byte("ok"))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, DefaultPreset: Bolt, ReadBufSize: 512}, h)
	defer stop()
	flood, _ := framingDial(t, addr)
	defer flood.Close()
	batch := strings.Repeat(paddedGet(256), 1000)
	go func() {
		for {
			if _, err := io.WriteString(flood, batch); err != nil {
				return
			}
		}
	}()
	go func() { _, _ = io.CopyBuffer(io.Discard, flood, make([]byte, 1<<20)) }()
	time.Sleep(300 * time.Millisecond)

	other, br := framingDial(t, addr)
	defer other.Close()
	_ = other.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.WriteString(other, framingGet("/other")); err != nil {
		t.Fatal(err)
	}
	if _, err := http.ReadResponse(br, nil); err != nil {
		flood.Close() // lets the worker stop
		t.Fatalf("starved by a pipelining connection: %v", err)
	}
}

// TestWingPipelineSurvivesYield pipelines more than one turn of requests and
// sends nothing after them.
func TestWingPipelineSurvivesYield(t *testing.T) {
	for _, size := range []int{256, 300} {
		h := transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
			_, _ = rw.Write([]byte("ok"))
		})
		addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, DefaultPreset: Bolt, ReadBufSize: 512}, h)
		defer stop()
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(10 * time.Second))
		const n = 5000
		go func() { _, _ = io.WriteString(c, strings.Repeat(paddedGet(size), n)) }()
		br := bufio.NewReader(c)
		for i := range n {
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatalf("size %d: response %d of %d: %v", size, i, n, err)
			}
			if _, err := io.Copy(io.Discard, res.Body); err != nil {
				t.Fatalf("size %d: response %d: %v", size, i, err)
			}
		}
	}
}
