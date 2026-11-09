package kruda

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type boundedStaticFS struct {
	size      int64
	failAfter int64
	maxRead   atomic.Int64
	closes    atomic.Int64
}

func (s *boundedStaticFS) Open(name string) (fs.File, error) {
	if name != "asset.bin" {
		return nil, fs.ErrNotExist
	}
	return &boundedStaticFile{fs: s}, nil
}

type boundedStaticFile struct {
	fs     *boundedStaticFS
	offset int64
}

func (f *boundedStaticFile) Stat() (fs.FileInfo, error) { return staticFileInfo{f.fs.size}, nil }
func (f *boundedStaticFile) Close() error               { f.fs.closes.Add(1); return nil }
func (f *boundedStaticFile) Read(p []byte) (int, error) {
	for old := f.fs.maxRead.Load(); int64(len(p)) > old; old = f.fs.maxRead.Load() {
		if f.fs.maxRead.CompareAndSwap(old, int64(len(p))) {
			break
		}
	}
	if len(p) > 64*1024 {
		return 0, fmt.Errorf("file read requested %d bytes; limit is 65536", len(p))
	}
	if f.fs.failAfter > 0 && f.offset >= f.fs.failAfter {
		return 0, io.ErrUnexpectedEOF
	}
	if f.offset == f.fs.size {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > f.fs.size-f.offset {
		n = int(f.fs.size - f.offset)
	}
	for i := range p[:n] {
		p[i] = byte((f.offset + int64(i)) % 251)
	}
	f.offset += int64(n)
	return n, nil
}

type staticFileInfo struct{ size int64 }

func (s staticFileInfo) Name() string       { return "asset.bin" }
func (s staticFileInfo) Size() int64        { return s.size }
func (s staticFileInfo) Mode() fs.FileMode  { return 0444 }
func (s staticFileInfo) ModTime() time.Time { return time.Time{} }
func (s staticFileInfo) IsDir() bool        { return false }
func (s staticFileInfo) Sys() any           { return nil }

func TestStaticBoundedReads(t *testing.T) {
	files := &boundedStaticFS{size: 4 << 20}
	app := New(NetHTTP())
	app.StaticFS("/assets", files)
	app.Compile()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/asset.bin", nil))
	if rec.Code != 200 || int64(rec.Body.Len()) != files.size {
		t.Fatalf("status=%d body=%d max read=%d; want status=200 body=%d", rec.Code, rec.Body.Len(), files.maxRead.Load(), files.size)
	}
	if files.maxRead.Load() > 64*1024 {
		t.Fatalf("max file read=%d", files.maxRead.Load())
	}
	if files.closes.Load() != 1 {
		t.Fatalf("file closes=%d, want 1", files.closes.Load())
	}
}

func TestStaticStreamTLS(t *testing.T) {
	files := &boundedStaticFS{size: 4 << 20}
	app := New(NetHTTP())
	app.StaticFS("/assets", files)
	app.Compile()
	server := httptest.NewTLSServer(app)
	defer server.Close()
	client := server.Client()
	defer client.CloseIdleConnections()
	resp, err := client.Get(server.URL + "/assets/asset.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK || n != files.size || err != nil {
		t.Fatalf("status=%d bytes=%d error=%v", resp.StatusCode, n, err)
	}
	if files.maxRead.Load() > 64<<10 || files.closes.Load() != 1 {
		t.Fatalf("max read=%d closes=%d", files.maxRead.Load(), files.closes.Load())
	}
}

func TestStaticPreservesAlreadySentResponse(t *testing.T) {
	files := &boundedStaticFS{size: 1024}
	app := New(NetHTTP())
	app.BeforeHandle(func(c *Ctx) error { return c.Text("already") })
	app.StaticFS("/assets", files)
	app.Compile()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/asset.bin", nil))
	if rec.Body.String() != "already" {
		t.Fatalf("static replaced an existing response: body length=%d", rec.Body.Len())
	}
}
