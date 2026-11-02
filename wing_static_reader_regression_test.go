//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWingFileStreamReadDoesNotBlockWorkerEventLoop(t *testing.T) {
	for _, mode := range []DispatchMode{Inline, Pool, Spawn, Takeover} {
		t.Run(mode.String(), func(t *testing.T) { testWingFileStreamReadDoesNotBlockWorkerEventLoop(t, mode) })
	}
}

func testWingFileStreamReadDoesNotBlockWorkerEventLoop(t *testing.T, mode DispatchMode) {
	t.Helper()
	gate := make(chan struct{})
	entered := make(chan struct{})
	var releaseOnce sync.Once
	files := &blockingFS{entered: entered, gate: gate}
	app := New(WithTransport(NewWingTransport(WingConfig{
		Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow.With(Dispatch(mode)),
	})))
	app.StaticFS("/assets", files)
	app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
	app.Get("/ping", func(c *Ctx) error { return c.Text("pong") })
	base := "http://" + startStaticStreamApp(t, app)
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	staticDone := make(chan error, 1)
	go func() {
		resp, err := client.Get(base + "/assets/asset.bin")
		if resp != nil {
			resp.Body.Close()
		}
		staticDone <- err
	}()
	defer releaseOnce.Do(func() { close(gate) })
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Wing did not start reading the static source")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	pingReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/ping", nil)
	pingResp, pingErr := client.Do(pingReq)
	if pingErr != nil {
		t.Fatalf("ping blocked by file read: %v", pingErr)
	}
	body, err := io.ReadAll(pingResp.Body)
	pingResp.Body.Close()
	if err != nil || string(body) != "pong" {
		t.Fatalf("ping body=%q err=%v", body, err)
	}
	releaseOnce.Do(func() { close(gate) })
	select {
	case err := <-staticDone:
		if err != nil {
			t.Fatalf("static request failed after releasing its reader: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("static request did not finish after releasing its reader")
	}

}

type blockingFS struct {
	entered chan struct{}
	gate    chan struct{}
	once    sync.Once
}

func (f *blockingFS) Open(string) (fs.File, error) { return &blockingFile{owner: f}, nil }

type blockingFile struct{ owner *blockingFS }

func (f *blockingFile) Stat() (fs.FileInfo, error) { return blockingInfo{}, nil }
func (f *blockingFile) Close() error               { return nil }
func (f *blockingFile) Read(p []byte) (int, error) {
	f.owner.once.Do(func() { close(f.owner.entered) })
	<-f.owner.gate
	p[0] = 'x'
	return 1, nil
}

type blockingInfo struct{}

func (blockingInfo) Name() string       { return "asset.bin" }
func (blockingInfo) Size() int64        { return 1 }
func (blockingInfo) Mode() fs.FileMode  { return 0444 }
func (blockingInfo) ModTime() time.Time { return time.Time{} }
func (blockingInfo) IsDir() bool        { return false }
func (blockingInfo) Sys() any           { return nil }

type panicStaticFS struct{}
type panicStaticFile struct{}

func (panicStaticFS) Open(string) (fs.File, error) { return panicStaticFile{}, nil }
func (panicStaticFile) Stat() (fs.FileInfo, error) { return blockingInfo{}, nil }
func (panicStaticFile) Read([]byte) (int, error)   { panic("file read failed") }
func (panicStaticFile) Close() error               { return nil }

func TestWingStaticReaderPanicClosesResponse(t *testing.T) {
	for _, mode := range []DispatchMode{Inline, Pool, Spawn, Takeover} {
		t.Run(mode.String(), func(t *testing.T) {
			app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow.With(Dispatch(mode))})))
			app.StaticFS("/assets", panicStaticFS{})
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			base := "http://" + startStaticStreamApp(t, app)
			client := &http.Client{Timeout: time.Second}
			defer client.CloseIdleConnections()
			resp, err := client.Get(base + "/assets/asset.bin")
			if err == nil {
				_, err = io.ReadAll(resp.Body)
				resp.Body.Close()
			}
			if err == nil {
				t.Fatal("panicking reader produced a complete response")
			}
			resp, err = client.Get(base + "/ready")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil || string(body) != "ready" {
				t.Fatalf("server after reader panic: %q %v", body, err)
			}
		})
	}
}

func TestWingBlockedFileReadDoesNotHoldShutdown(t *testing.T) {
	for _, mode := range []DispatchMode{Inline, Pool, Spawn, Takeover} {
		t.Run(mode.String(), func(t *testing.T) {
			gate := make(chan struct{})
			entered := make(chan struct{})
			defer close(gate)
			app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow.With(Dispatch(mode))})))
			app.StaticFS("/assets", &blockingFS{entered: entered, gate: gate})
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			base := "http://" + startStaticStreamApp(t, app)
			client := &http.Client{Timeout: time.Second}
			defer client.CloseIdleConnections()
			go func() {
				resp, _ := client.Get(base + "/assets/asset.bin")
				if resp != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("read did not start")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := app.Shutdown(ctx); err != nil {
				t.Fatalf("shutdown waited for file reader: %v", err)
			}
		})
	}
}

type slowCloseFS struct {
	entered, gate chan struct{}
	size          int64
}
type slowCloseFile struct{ owner *slowCloseFS }

func (f *slowCloseFS) Open(string) (fs.File, error) { return &slowCloseFile{f}, nil }
func (f *slowCloseFile) Stat() (fs.FileInfo, error) { return staticSizeInfo{size: f.owner.size}, nil }
func (f *slowCloseFile) Read(p []byte) (int, error) { p[0] = 'x'; return 1, nil }
func (f *slowCloseFile) Close() error               { close(f.owner.entered); <-f.owner.gate; panic("close failed") }

func TestWingFileCloseDoesNotBlockOrPanicWorker(t *testing.T) {
	files := &slowCloseFS{entered: make(chan struct{}), gate: make(chan struct{})}
	defer close(files.gate)
	app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow})))
	app.StaticFS("/assets", files)
	app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
	base := "http://" + startStaticStreamApp(t, app)
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	resp, err := client.Get(base + "/assets/asset.bin")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	select {
	case <-files.entered:
	case <-time.After(time.Second):
		t.Fatal("close did not start")
	}
	resp, err = client.Get(base + "/ready")
	if err != nil {
		t.Fatalf("close blocked worker: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestWingPendingFileReadHasWriteTimeout(t *testing.T) {
	w, _ := newTestWorker(0)
	w.writeTimeout = 1
	w.now = 10
	c := newTestConn(12345)
	c.lastActive = 1
	c.fileStream = &fileStream{reader: io.NopCloser(&emptyReader{}), remaining: 1, reading: true}
	w.conns[c.fd] = c
	w.sweepTimeouts(w.now)
	if w.conns[c.fd] != nil {
		t.Fatal("write timeout ignored pending source read")
	}
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

func TestWingBlockedReaderWriteTimeoutWithoutTraffic(t *testing.T) {
	for _, mode := range []DispatchMode{Inline, Pool, Spawn, Takeover} {
		t.Run(mode.String(), func(t *testing.T) {
			gate := make(chan struct{})
			defer close(gate)
			app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow.With(Dispatch(mode)), WriteTimeout: 100 * time.Millisecond})))
			app.StaticFS("/assets", &blockingFS{entered: make(chan struct{}), gate: gate})
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			base := "http://" + startStaticStreamApp(t, app)
			client := &http.Client{Timeout: 2 * time.Second}
			defer client.CloseIdleConnections()
			start := time.Now()
			resp, err := client.Get(base + "/assets/asset.bin")
			if err == nil {
				_, err = io.ReadAll(resp.Body)
				resp.Body.Close()
			}
			if err == nil || time.Since(start) > time.Second {
				t.Fatalf("pending read deadline: elapsed=%v err=%v", time.Since(start), err)
			}
		})
	}
}

type staticSizeInfo struct {
	blockingInfo
	size int64
}

func (s staticSizeInfo) Size() int64 { return s.size }
func TestWingStaticAdmissionIncludesEmptyFilesAndClose(t *testing.T) {
	for _, size := range []int64{0, 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			files := &slowCloseFS{entered: make(chan struct{}), gate: make(chan struct{}), size: size}
			var release sync.Once
			defer release.Do(func() { close(files.gate) })
			app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow})))
			app.StaticFS("/assets", files, WithMaxOpenFiles(1))
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			base := "http://" + startStaticStreamApp(t, app)
			client := &http.Client{Timeout: time.Second}
			defer client.CloseIdleConnections()
			resp, err := client.Get(base + "/assets/asset.bin")
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			select {
			case <-files.entered:
			case <-time.After(time.Second):
				t.Fatal("close not started")
			}
			resp, err = client.Get(base + "/assets/asset.bin")
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 503 {
				t.Fatalf("open limit status=%d", resp.StatusCode)
			}
			release.Do(func() { close(files.gate) })
		})
	}
}

func TestWingSocketBackpressureWriteTimeoutWithoutTraffic(t *testing.T) {
	app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow, WriteTimeout: 100 * time.Millisecond})))
	files := &boundedStaticFS{size: 64 << 20}
	app.StaticFS("/assets", files)
	app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
	addr := startStaticStreamApp(t, app)
	conn, err := net.Dial("tcp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.(*net.TCPConn).SetReadBuffer(4096)
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(1500 * time.Millisecond)
	n, err := io.Copy(io.Discard, resp.Body)
	if err == nil || n >= files.size {
		t.Fatalf("stalled client did not expire: bytes=%d err=%v", n, err)
	}
}

func TestWingSendfileRetainsStaticAdmission(t *testing.T) {
	root := t.TempDir()
	f, err := os.Create(filepath.Join(root, "asset.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(64 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1})))
	app.Static("/assets", root, WithMaxOpenFiles(1))
	app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
	addr := startStaticStreamApp(t, app)
	conn, err := net.Dial("tcp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.(*net.TCPConn).SetReadBuffer(4096)
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	second, err := client.Get("http://" + addr + "/assets/asset.bin")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, second.Body)
	second.Body.Close()
	if second.StatusCode != 503 {
		t.Fatalf("sendfile released admission early: %d", second.StatusCode)
	}
	conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	for {
		next, err := client.Get("http://" + addr + "/assets/asset.bin")
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, next.Body)
		next.Body.Close()
		if next.StatusCode == 200 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("closed sendfile retained admission")
		}
		time.Sleep(time.Millisecond * 10)
	}
}
