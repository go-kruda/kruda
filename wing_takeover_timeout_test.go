//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func TestWingTakeoverIdleTimeout(t *testing.T) {
	presets := []struct {
		name   string
		preset Preset
	}{
		{name: "Spear", preset: Spear},
		{name: "DB", preset: DB},
		{name: "Render", preset: Render},
	}

	for _, tt := range presets {
		t.Run(tt.name, func(t *testing.T) {
			cfg := WingConfig{
				Workers:       1,
				DefaultPreset: tt.preset,
				IdleTimeout:   100 * time.Millisecond,
			}
			addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))
			defer stop()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadAll(resp.Body); err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if err := conn.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			var one [1]byte
			if _, err := conn.Read(one[:]); err == nil {
				t.Fatal("idle Takeover connection remained readable")
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				t.Fatal("idle Takeover connection was not closed by IdleTimeout")
			}
		})
	}
}

func TestWingTakeoverReadTimeoutBoundsPartialNextRequest(t *testing.T) {
	cfg := WingConfig{
		Workers:       1,
		DefaultPreset: DB,
		ReadTimeout:   100 * time.Millisecond,
		IdleTimeout:   2 * time.Second,
	}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test"); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := conn.Read(one[:]); err == nil {
		t.Fatal("partial Takeover request unexpectedly produced data")
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("partial Takeover request was not closed by ReadTimeout")
	}
}

func TestWingTakeoverWriteTimeoutReleasesBlockedResponse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	cfg := WingConfig{
		Workers:       1,
		DefaultPreset: DB,
		WriteTimeout:  100 * time.Millisecond,
		MaxConns:      1,
	}
	cfg.defaults()
	tr := NewWingTransport(cfg)
	handlerDone := make(chan struct{})
	go tr.ListenAndServe(addr, transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 8<<20))
		close(handlerDone)
	}))
	defer tr.Shutdown(context.Background())

	var conn net.Conn
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetReadBuffer(1024); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Takeover handler did not finish")
	}

	deadline = time.Now().Add(750 * time.Millisecond)
	for tr.connCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := tr.connCount(); got != 0 {
		t.Fatalf("admitted connections = %d, want 0 after WriteTimeout", got)
	}
}
