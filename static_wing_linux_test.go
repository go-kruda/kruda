//go:build linux

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestAppStaticWingRetainsFileThroughSendfile(t *testing.T) {
	dir := t.TempDir()
	want := bytes.Repeat([]byte("static-file-body\n"), 128*1024)
	if err := os.WriteFile(filepath.Join(dir, "asset.bin"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	decoyPath := filepath.Join(dir, "decoy.bin")
	if err := os.WriteFile(decoyPath, bytes.Repeat([]byte("decoy\n"), 1024), 0o600); err != nil {
		t.Fatal(err)
	}

	app := New(Wing())
	app.Static("/assets", dir)
	addr, stop := startWingApp(t, app)
	defer stop()

	stopChurn := make(chan struct{})
	var churnWG sync.WaitGroup
	churnWG.Add(1)
	go func() {
		defer churnWG.Done()
		for {
			select {
			case <-stopChurn:
				return
			default:
				f, err := os.Open(decoyPath)
				if err == nil {
					runtime.Gosched()
					_ = f.Close()
				}
			}
		}
	}()
	defer func() {
		close(stopChurn)
		churnWG.Wait()
	}()

	for i := 0; i < 8; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		if _, err := io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			conn.Close()
			t.Fatal(err)
		}
		got, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		conn.Close()
		if err != nil {
			t.Fatalf("request %d: read static body: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("request %d: body length = %d, want %d", i, len(got), len(want))
		}
	}
}

func TestAppStaticWingFallsBackOutsideInlineDispatch(t *testing.T) {
	presets := []struct {
		name   string
		preset Preset
	}{
		{name: "Pool", preset: Arrow},
		{name: "Spawn", preset: Preset{Dispatch: Spawn}},
		{name: "Takeover", preset: DB},
	}

	for _, tt := range presets {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			want := []byte("buffered static response")
			if err := os.WriteFile(filepath.Join(dir, "asset.txt"), want, 0o600); err != nil {
				t.Fatal(err)
			}
			tr := NewWingTransport(WingConfig{Workers: 1, DefaultPreset: tt.preset})
			app := New(WithTransport(tr))
			app.Static("/assets", dir)
			addr, stop := startWingApp(t, app)
			defer stop()

			resp, err := http.Get("http://" + addr + "/assets/asset.txt")
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("body = %q, want %q", got, want)
			}
		})
	}
}

func TestDuplicateStaticFileSurvivesSourceDescriptorReuse(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.txt")
	decoyPath := filepath.Join(dir, "decoy.txt")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decoyPath, []byte("decoy"), 0o600); err != nil {
		t.Fatal(err)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	sourceFD := int(source.Fd())
	ownedFD, ok := duplicateStaticFile(source)
	if !ok {
		source.Close()
		t.Fatal("duplicateStaticFile failed")
	}
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(ownedFD), uintptr(syscall.F_GETFD), 0)
	if errno != 0 {
		source.Close()
		syscall.Close(int(ownedFD))
		t.Fatal(errno)
	}
	if flags&syscall.FD_CLOEXEC == 0 {
		source.Close()
		syscall.Close(int(ownedFD))
		t.Fatal("duplicated static file descriptor is not close-on-exec")
	}
	owned := os.NewFile(uintptr(ownedFD), "owned-static")
	if err := source.Close(); err != nil {
		owned.Close()
		t.Fatal(err)
	}

	decoy, err := os.Open(decoyPath)
	if err != nil {
		owned.Close()
		t.Fatal(err)
	}
	defer decoy.Close()
	if int(decoy.Fd()) != sourceFD {
		if err := syscall.Dup3(int(decoy.Fd()), sourceFD, 0); err != nil {
			owned.Close()
			t.Fatal(err)
		}
		defer syscall.Close(sourceFD)
	}

	got, err := io.ReadAll(owned)
	if err != nil {
		owned.Close()
		t.Fatal(err)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if string(got) != "source" {
		t.Fatalf("owned descriptor read %q after source fd reuse", got)
	}
	var one [1]byte
	if _, err := syscall.Read(int(ownedFD), one[:]); err != syscall.EBADF {
		t.Fatalf("read after owned close = %v, want EBADF", err)
	}
}
