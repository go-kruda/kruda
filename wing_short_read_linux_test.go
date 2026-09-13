//go:build linux

package kruda

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func newShortReadWorker(t *testing.T, readSize int, handler transport.Handler) (*worker, *conn, net.Conn, *mockEngine) {
	t.Helper()
	server, client := newSocketpairFiles(t)
	t.Cleanup(func() { server.Close() })
	fd := int32(server.Fd())
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		client.Close()
		t.Fatal(err)
	}
	peer, err := net.FileConn(client)
	client.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.Close() })
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	w, eng := newTestWorker(0)
	w.config.ReadBufSize = readSize
	w.handler = handler
	w.presets = NewPresetTable(nil, Bolt)
	c := newTestConn(fd)
	c.readBuf = make([]byte, readSize)
	c.ctx = context.Background()
	w.conns[fd] = c
	return w, c, peer, eng
}

func queueShortReadRequest(t *testing.T, peer net.Conn, request string) {
	t.Helper()
	if n, err := io.WriteString(peer, request); err != nil || n != len(request) {
		t.Fatalf("write request: %d/%d bytes, %v", n, len(request), err)
	}
}

type shortReadEOFEngine struct {
	*mockEngine
	closeErr error
}

func (e *shortReadEOFEngine) SubmitClose(fd int32) {
	e.mockEngine.SubmitClose(fd)
	// Expose closure to the peer while leaving descriptor ownership with test cleanup.
	e.closeErr = syscall.Shutdown(int(fd), syscall.SHUT_RDWR)
}

func TestWingShortReadBaselinePipelineBeyondReadBuffer(t *testing.T) {
	const readSize = 512
	var paths []string
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		paths = append(paths, strings.Clone(r.Path()))
		w.Write([]byte(r.Path()))
	})
	w, c, peer, _ := newShortReadWorker(t, readSize, handler)
	var pipeline strings.Builder
	var want []string
	for i := 0; i < 32; i++ {
		path := fmt.Sprintf("/%02d", i)
		want = append(want, path)
		fmt.Fprintf(&pipeline, "GET %s HTTP/1.1\r\nHost: h\r\n\r\n", path)
	}
	requestSize := len(pipeline.String()) / len(want)
	if pipeline.Len() <= readSize || readSize%requestSize == 0 {
		t.Fatal("fixture must leave a partial header at the end of a full read")
	}
	queueShortReadRequest(t, peer, pipeline.String())
	queued := make([]byte, pipeline.Len())
	if n, _, err := syscall.Recvfrom(int(c.fd), queued, syscall.MSG_PEEK|syscall.MSG_DONTWAIT); err != nil || n != len(queued) {
		t.Fatalf("queued bytes = %d, %v; want %d before the first read", n, err, len(queued))
	}

	// Only one readiness event: queued kernel bytes must not need another client write.
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("dispatched %d/%d requests after one readiness event; buffered bytes=%d", len(paths), len(want), c.readN)
	}
	reader := bufio.NewReader(peer)
	for _, path := range want {
		assertWingPipelineResponse(t, reader, path)
	}
}

func TestWingShortReadExactBufferDrainsQueuedRequest(t *testing.T) {
	const readSize = 512
	var paths []string
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		paths = append(paths, strings.Clone(r.Path()))
		w.Write([]byte(r.Path()))
	})
	w, c, peer, _ := newShortReadWorker(t, readSize, handler)
	prefix := "GET /first HTTP/1.1\r\nHost: h\r\nX-Pad: "
	first := prefix + strings.Repeat("x", readSize-len(prefix)-4) + "\r\n\r\n"
	queueShortReadRequest(t, peer, first+"GET /next HTTP/1.1\r\nHost: h\r\n\r\n")
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if strings.Join(paths, ",") != "/first,/next" {
		t.Fatalf("full read left a queued request undispatched: %v", paths)
	}
	reader := bufio.NewReader(peer)
	assertWingPipelineResponse(t, reader, "/first")
	assertWingPipelineResponse(t, reader, "/next")
}

func TestWingShortReadFullBufferWaitsForHeaderRemainder(t *testing.T) {
	const readSize = 512
	handled := 0
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		handled++
		w.Write([]byte(r.Path()))
	})
	w, c, peer, eng := newShortReadWorker(t, readSize, handler)
	request := "GET /ok HTTP/1.1\r\nHost: h\r\n\r\n"
	completeRequests := readSize / len(request)
	pipeline := strings.Repeat(request, completeRequests+1)
	if readSize%len(request) == 0 {
		t.Fatal("fixture must end the full read inside a request header")
	}
	queueShortReadRequest(t, peer, pipeline[:readSize])
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if len(eng.closedFds) != 0 || w.conns[c.fd] != c {
		t.Fatal("connection closed when the header continuation was not yet available")
	}
	if handled != completeRequests {
		t.Fatalf("handled %d requests before continuation, want %d", handled, completeRequests)
	}
	var one [1]byte
	if _, _, err := syscall.Recvfrom(int(c.fd), one[:], syscall.MSG_PEEK|syscall.MSG_DONTWAIT); err != syscall.EAGAIN && err != syscall.EWOULDBLOCK {
		t.Fatalf("kernel receive queue must be empty before continuation: %v", err)
	}
	reader := bufio.NewReader(peer)
	for i := 0; i < completeRequests; i++ {
		assertWingPipelineResponse(t, reader, "/ok")
	}

	queueShortReadRequest(t, peer, pipeline[readSize:])
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if handled != completeRequests+1 {
		t.Fatalf("handled %d requests after continuation, want %d", handled, completeRequests+1)
	}
	assertWingPipelineResponse(t, reader, "/ok")
}

func TestWingShortReadEOFDeliversQueuedResponses(t *testing.T) {
	for _, tt := range []struct {
		name        string
		bodySize    int
		partialSend bool
	}{{"SmallResponses", 64, false}, {"PartialWrites", 4096, true}} {
		t.Run(tt.name, func(t *testing.T) {
			const readSize = 512
			body := strings.Repeat("x", tt.bodySize)
			handled := 0
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				handled++
				w.Write([]byte(body))
			})
			w, c, peer, eng := newShortReadWorker(t, readSize, handler)
			closing := &shortReadEOFEngine{mockEngine: eng}
			w.eng = closing
			if tt.partialSend {
				if err := syscall.SetsockoptInt(int(c.fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
					t.Fatal(err)
				}
			}
			request := "GET /ok HTTP/1.1\r\nHost: h\r\n\r\n"
			completeRequests := readSize / len(request)
			pipeline := strings.Repeat(request, completeRequests+1)
			if readSize%len(request) == 0 {
				t.Fatal("fixture must end the full read inside a request header")
			}
			queueShortReadRequest(t, peer, pipeline[:readSize])
			if err := peer.(*net.UnixConn).CloseWrite(); err != nil {
				t.Fatal(err)
			}
			w.handleRecv(event{Op: opRecv, Fd: c.fd})
			if handled != completeRequests {
				t.Fatalf("handled %d requests before EOF, want %d", handled, completeRequests)
			}
			if tt.partialSend && (eng.sendArmed == 0 || c.sendN == 0 || c.sendN >= len(c.sendBuf)) {
				t.Fatal("queued large responses were not retained as a real partial write")
			}
			var received bytes.Buffer
			var chunk [16 << 10]byte
			for i := 0; i < 128 && c.sendN < len(c.sendBuf); i++ {
				n, err := peer.Read(chunk[:])
				if err != nil {
					t.Fatalf("read before queued responses completed: %v", err)
				}
				received.Write(chunk[:n])
				w.handleSend(event{Op: opSend, Fd: c.fd})
			}
			if c.sendN < len(c.sendBuf) {
				t.Fatal("queued responses remained after writable progress")
			}
			reader := bufio.NewReader(io.MultiReader(&received, peer))
			for i := 0; i < completeRequests; i++ {
				status, got, err := readHTTPResponse(reader)
				if err != nil || status != "HTTP/1.1 200 OK" || got != body {
					t.Fatalf("response %d before EOF = (%q, %d bytes, %v), want 200 and %d bytes", i+1, status, len(got), err, len(body))
				}
			}
			if len(eng.closedFds) != 1 || w.conns[c.fd] != nil || closing.closeErr != nil {
				t.Fatalf("completed responses did not close cleanly: closed=%v, retained=%t, error=%v", eng.closedFds, w.conns[c.fd] != nil, closing.closeErr)
			}
			if _, err := reader.ReadByte(); err != io.EOF {
				t.Fatalf("after %d complete responses got %v, want EOF", completeRequests, err)
			}
		})
	}
}

func TestWingShortReadSequentialKeepAlive(t *testing.T) {
	app := New(Wing())
	app.Get("/:id", func(c *Ctx) error { return c.Text(c.Param("id")) }, Bolt)
	addr, stop := startWingApp(t, app)
	defer stop()
	peer, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(peer)
	for i := 0; i < 8; i++ {
		queueShortReadRequest(t, peer, fmt.Sprintf("GET /%d HTTP/1.1\r\nHost: h\r\n\r\n", i))
		assertWingPipelineResponse(t, reader, fmt.Sprint(i))
	}
}

func TestWingShortReadPartialWriteWithReadableRequest(t *testing.T) {
	large := strings.Repeat("x", 128<<10)
	var paths []string
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		paths = append(paths, strings.Clone(r.Path()))
		if r.Path() == "/large" {
			w.Write([]byte(large))
		} else {
			w.Write([]byte("next"))
		}
	})
	w, c, peer, eng := newShortReadWorker(t, 512, handler)
	if err := syscall.SetsockoptInt(int(c.fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
		t.Fatal(err)
	}
	queueShortReadRequest(t, peer, "GET /large HTTP/1.1\r\nHost: h\r\n\r\n")
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if c.sendN == 0 || c.sendN >= len(c.sendBuf) || eng.sendArmed == 0 {
		t.Fatal("fixture did not produce a partial write awaiting EPOLLOUT")
	}
	queueShortReadRequest(t, peer, "GET /next HTTP/1.1\r\nHost: h\r\n\r\n")
	var received bytes.Buffer
	var chunk [16 << 10]byte
	for i := 0; i < 256 && (len(paths) < 2 || c.sendN < len(c.sendBuf)); i++ {
		n, err := peer.Read(chunk[:])
		if err != nil {
			t.Fatalf("drain partial response: %v", err)
		}
		received.Write(chunk[:n])
		// The engine gives EPOLLOUT priority when input and output are both ready.
		w.handleSend(event{Op: opSend, Fd: c.fd})
	}
	if strings.Join(paths, ",") != "/large,/next" || c.sendN < len(c.sendBuf) {
		t.Fatalf("response drain failed to serve queued input: paths=%v, pending=%d", paths, len(c.sendBuf)-c.sendN)
	}
	reader := bufio.NewReader(io.MultiReader(&received, peer))
	status, body, err := readHTTPResponse(reader)
	if err != nil || status != "HTTP/1.1 200 OK" || body != large {
		t.Fatalf("partial first response = (%q, %d bytes, %v), want 200 and %d bytes", status, len(body), err, len(large))
	}
	assertWingPipelineResponse(t, reader, "next")
}

func TestWingShortReadAsyncSuppressedReadiness(t *testing.T) {
	for _, tt := range []struct {
		name   string
		preset Preset
	}{{"Pool", Arrow}, {"Spawn", Preset{Dispatch: Spawn}}} {
		t.Run(tt.name, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			suppressed := make(chan struct{}, 1)
			testSuppressedRecvHook = func(int32) {
				select {
				case suppressed <- struct{}{}:
				default:
				}
			}
			t.Cleanup(func() { testSuppressedRecvHook = nil })
			app := New(Wing())
			app.Get("/inline", func(c *Ctx) error { return c.Text("inline") }, Bolt)
			app.Get("/async", func(c *Ctx) error {
				close(started)
				<-release
				return c.Text("async")
			}, tt.preset)
			app.Get("/next", func(c *Ctx) error { return c.Text("next") }, Bolt)
			addr, stop := startWingApp(t, app)
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
				stop()
			}()
			peer, err := net.DialTimeout("tcp", addr, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			queueShortReadRequest(t, peer, "GET /inline HTTP/1.1\r\nHost: h\r\n\r\nGET /async HTTP/1.1\r\nHost: h\r\n\r\n")
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("async handler did not start")
			}
			reader := bufio.NewReader(peer)
			assertWingPipelineResponse(t, reader, "inline")
			queueShortReadRequest(t, peer, "GET /next HTTP/1.1\r\nHost: h\r\nConnection: close\r\n\r\n")
			select {
			case <-suppressed:
			case <-time.After(time.Second):
				t.Fatal("EPOLLIN was not observed while the async handler was pending")
			}
			close(release)
			assertWingPipelineResponse(t, reader, "async")
			assertWingPipelineResponse(t, reader, "next")
		})
	}
}

func TestWingShortReadBodyPipelineAfterKeepAlive(t *testing.T) {
	cfg := WingConfig{Workers: 1, ReadBufSize: 512, BodyLimit: 4096}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()
	peer, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(peer)
	queueShortReadRequest(t, peer, "GET /warm HTTP/1.1\r\nHost: h\r\n\r\n")
	assertWingPipelineResponse(t, reader, "GET /warm len=0")
	body := strings.Repeat("b", 1024)
	queueShortReadRequest(t, peer, fmt.Sprintf("POST /body HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n%sGET /done HT", len(body), body))
	assertWingPipelineResponse(t, reader, "POST /body len=1024")
	queueShortReadRequest(t, peer, "TP/1.1\r\nHost: h\r\nConnection: close\r\n\r\n")
	assertWingPipelineResponse(t, reader, "GET /done len=0")
}
