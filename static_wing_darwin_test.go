//go:build darwin

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppStaticWingDarwinRetainsFileThroughSendfile(t *testing.T) {
	dir := t.TempDir()
	want := bytes.Repeat([]byte("darwin-static\n"), 64*1024)
	if err := os.WriteFile(filepath.Join(dir, "asset.bin"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(Wing())
	app.Static("/assets", dir)
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
	if _, err := io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("body length = %d, want %d", len(got), len(want))
	}
}
