//go:build linux || darwin

package kruda

import (
	"github.com/go-kruda/kruda/transport"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestWingFileStreamOwnershipCleanup(t *testing.T) {
	for _, where := range []string{"response", "missing connection", "close pending", "send failure", "connection", "cleanup"} {
		t.Run(where, func(t *testing.T) {
			files := &boundedStaticFS{size: 1024}
			file, err := files.Open("asset.bin")
			if err != nil {
				t.Fatal(err)
			}
			stream := &fileStream{reader: file, remaining: files.size}
			w, _ := newTestWorker(0)
			c := newTestConn(12345)
			w.conns[c.fd] = c
			switch where {
			case "response":
				r := acquireResponse()
				r.fileStream = stream
				releaseResponse(r)
			case "missing connection":
				w.handleDone(doneMsg{fd: 42, stream: stream})
			case "close pending":
				c.pending = 1
				c.closePending = true
				w.handleDone(doneMsg{fd: c.fd, stream: stream})
			case "send failure":
				c.fileStream = stream
				w.failSend(c)
			case "connection":
				c.fileStream = stream
				w.removeConnBookkeeping(c.fd)
			case "cleanup":
				w.doneCh = make(chan doneMsg, 1)
				w.doneCh <- doneMsg{fd: c.fd, stream: stream}
				delete(w.conns, c.fd)
				w.cleanup()
			}
			waitStaticCloses(t, files, 1)
			if n := files.closes.Load(); n != 1 {
				t.Fatalf("file closed %d times, want 1", n)
			}
			stream.close()
			waitStaticCloses(t, files, 1)
			if n := files.closes.Load(); n != 1 {
				t.Fatalf("second cleanup closed file %d times", n)
			}
		})
	}
}

func TestWingFileStreamReplacementClosesPrevious(t *testing.T) {
	files := &boundedStaticFS{size: 1024}
	first, _ := files.Open("asset.bin")
	second, _ := files.Open("asset.bin")
	r := acquireResponse()
	r.SetFileStream(first, files.size)
	r.SetFileStream(second, files.size)
	waitStaticCloses(t, files, 1)
	if n := files.closes.Load(); n != 1 {
		t.Fatalf("replacement closed %d files", n)
	}
	releaseResponse(r)
	waitStaticCloses(t, files, 2)
	if n := files.closes.Load(); n != 2 {
		t.Fatalf("release closed %d files", n)
	}
}

func TestWingFileStreamSerializationDoesNotReadBody(t *testing.T) {
	files := &boundedStaticFS{size: 64 << 20}
	file, _ := files.Open("asset.bin")
	r := acquireResponse()
	r.SetFileStream(file, files.size)
	headers := r.buildZeroCopy()
	if len(headers) > 1024 {
		t.Fatalf("serialized %d bytes before file delivery", len(headers))
	}
	if files.maxRead.Load() != 0 {
		t.Fatal("response serialization read file")
	}
	stream := r.fileStream
	r.fileStream = nil
	releaseResponse(r)
	if files.closes.Load() != 0 {
		t.Fatal("pooled response closed transferred file")
	}
	if _, err := io.CopyN(io.Discard, stream.reader, 1024); err != nil {
		t.Fatal(err)
	}
	stream.close()
}

func TestWingFileStreamBackpressureStopsDispatch(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetsockoptInt(fds[0], syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1024); err != nil {
		t.Fatal(err)
	}
	files := &boundedStaticFS{size: 16 << 20}
	file, _ := files.Open("asset.bin")
	w, eng := newTestWorker(0)
	w.doneCh = make(chan doneMsg, 1)
	c := newTestConn(int32(fds[0]))
	c.keepAlive = true
	c.fileStream = &fileStream{reader: file, remaining: files.size}
	c.readN = copy(c.readBuf, "GET /next HTTP/1.1\r\nHost: test\r\n\r\n")
	w.conns[c.fd] = c
	defer w.closeConn(c.fd)
	w.directSend(c)
	select {
	case msg := <-w.doneCh:
		w.handleDone(msg)
	case <-time.After(time.Second):
		t.Fatal("file read did not complete")
	}
	w.dispatchWG.Wait()
	if c.fileStream == nil || eng.sendArmed == 0 {
		t.Fatal("stream did not stop on socket backpressure")
	}
	if len(c.sendBuf) > 32<<10 || files.maxRead.Load() > 32<<10 {
		t.Fatalf("unbounded pending chunk: buffer=%d read=%d", len(c.sendBuf), files.maxRead.Load())
	}
	pending := c.readN
	w.handleRecv(event{Fd: c.fd})
	if !c.readSuppressed || c.readN != pending {
		t.Fatal("read readiness consumed next request during stream")
	}
	w.tryParse(c)
	if c.readN != pending {
		t.Fatal("next request parsed before streamed body drained")
	}
}

func TestWingSendfileBackpressureStopsStreamDispatch(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "sendfile")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(16 << 20); err != nil {
		file.Close()
		t.Fatal(err)
	}
	fd, ok := duplicateStaticFile(file)
	file.Close()
	if !ok {
		t.Fatal("duplicate source file")
	}
	files := &boundedStaticFS{size: 1024}
	calls := 0
	w, _ := newTestWorker(0)
	w.presets = NewPresetTable(nil, Bolt)
	w.limits = parserLimits{maxHeaderCount: 64, maxHeaderSize: 8192, bodyLimit: 1 << 20}
	w.handler = transport.HandlerFunc(func(writer transport.ResponseWriter, req transport.Request) {
		calls++
		f, _ := files.Open("asset.bin")
		writer.WriteHeader(200)
		writer.(transport.FileStreamer).SetFileStream(f, files.size)
	})
	c := newTestConn(int32(fds[0]))
	c.keepAlive = true
	c.sendFileFd = fd
	c.sendFileSize = 16 << 20
	w.conns[c.fd] = c
	defer w.removeConnBookkeeping(c.fd)
	if _, err := syscall.Write(fds[1], []byte("GET /stream HTTP/1.1\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Fd: c.fd})
	if calls != 0 || !c.readSuppressed {
		t.Fatalf("dispatched %d later stream requests while sendfile pending; suppressed=%v", calls, c.readSuppressed)
	}
}
