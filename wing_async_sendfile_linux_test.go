//go:build linux

package kruda

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func TestDirectSendRetainsConnectionOnSendfileErrorWhileAsyncHandlerOwnsFD(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])

	file, err := os.CreateTemp(t.TempDir(), "sendfile")
	if err != nil {
		t.Fatal(err)
	}
	fileFd := int32(file.Fd())
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	w, eng := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	c.pending = 1
	c.sendFileFd = fileFd
	c.sendFileSize = 1
	w.conns[c.fd] = c

	w.directSend(c)

	if len(eng.closedFds) != 0 {
		t.Fatal("connection fd closed while async handler still owned it")
	}
	if w.conns[c.fd] == nil {
		t.Fatal("connection bookkeeping removed before async completion")
	}
}

func TestDirectSendClosesOwnedSendfileDescriptorOnSuccess(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "success.bin")
	if err := os.WriteFile(path, []byte("sendfile success"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileFD, err := syscall.Open(path, syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}

	w, _ := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	c.keepAlive = true
	c.sendFileFd = int32(fileFD)
	c.sendFileSize = int64(len("sendfile success"))
	w.conns[c.fd] = c

	w.directSend(c)

	if c.sendFileFd != 0 {
		t.Fatalf("sendFileFd = %d, want released", c.sendFileFd)
	}
	assertDescriptorClosed(t, fileFD)
}

func TestFailSendClosesOwnedSendfileDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failure.bin")
	if err := os.WriteFile(path, []byte("sendfile failure"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileFD, err := syscall.Open(path, syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}

	w, _ := newTestWorker(0)
	c := newTestConn(123)
	c.pending = 1
	c.sendFileFd = int32(fileFD)
	c.sendFileSize = int64(len("sendfile failure"))
	w.conns[c.fd] = c

	w.failSend(c)

	if c.sendFileFd != 0 {
		t.Fatalf("sendFileFd = %d, want released", c.sendFileFd)
	}
	assertDescriptorClosed(t, fileFD)
}

func TestConnectionCleanupClosesOwnedSendfileDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disconnect.bin")
	if err := os.WriteFile(path, []byte("disconnect"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileFD, err := syscall.Open(path, syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}

	w, _ := newTestWorker(0)
	c := newTestConn(123)
	c.sendFileFd = int32(fileFD)
	c.sendFileSize = int64(len("disconnect"))
	w.conns[c.fd] = c

	w.removeConnBookkeeping(c.fd)

	if c.sendFileFd != 0 {
		t.Fatalf("sendFileFd = %d, want released", c.sendFileFd)
	}
	assertDescriptorClosed(t, fileFD)
}

func assertDescriptorClosed(t *testing.T, fd int) {
	t.Helper()
	var one [1]byte
	if _, err := syscall.Read(fd, one[:]); err != syscall.EBADF {
		t.Fatalf("read fd %d after release = %v, want EBADF", fd, err)
	}
}

func TestWingAsyncPipelinePreservesSendfileResponseOrder(t *testing.T) {
	presets := []struct {
		name   string
		preset Preset
	}{
		{name: "Pool", preset: Arrow},
		{name: "Spawn", preset: Preset{Dispatch: Spawn}},
		{name: "Takeover", preset: DB},
	}
	fileBody := bytes.Repeat([]byte("f"), 8<<20)
	filePath := filepath.Join(t.TempDir(), "response.bin")
	if err := os.WriteFile(filePath, fileBody, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range presets {
		t.Run(tt.name, func(t *testing.T) {
			app := New(Wing())
			app.Get("/file", func(c *Ctx) error {
				fileSender, ok := c.ResponseWriter().(transport.FileSender)
				if !ok {
					return errors.New("Wing response does not support sendfile")
				}
				fd, err := syscall.Open(filePath, syscall.O_RDONLY, 0)
				if err != nil {
					return err
				}
				fileSender.SetSendFile(int32(fd), int64(len(fileBody)))
				return nil
			}, Bolt)
			app.Get("/async", func(c *Ctx) error { return c.Text("async") }, tt.preset)

			addr, stop := startWingApp(t, app)
			defer stop()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
				t.Fatal(err)
			}
			pipeline := "GET /file HTTP/1.1\r\nHost: test\r\n\r\n" +
				"GET /async HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"
			if _, err := io.WriteString(conn, pipeline); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)

			reader := bufio.NewReader(conn)
			resp, err := http.ReadResponse(reader, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("read sendfile body (%d/%d bytes): %v", len(body), len(fileBody), err)
			}
			if !bytes.Equal(body, fileBody) {
				t.Fatal("async response was written before the preceding sendfile body completed")
			}
			assertWingPipelineResponse(t, reader, "async")
		})
	}
}
