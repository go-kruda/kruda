//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestWingFullPoolAnswers503AndKeepsTheLoopFree(t *testing.T) {
	release := make(chan struct{})
	app := New(WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 1})))
	app.Get("/slow", func(c *Ctx) error {
		<-release
		return c.Text("slow")
	}, Arrow)
	app.Get("/fast", func(c *Ctx) error { return c.Text("fast") })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	go func() { _ = app.Listen(addr) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
	}()
	defer close(release)
	ready := false
	for i := 0; i < 100 && !ready; i++ {
		if resp, err := http.Get("http://" + addr + "/fast"); err == nil {
			_ = resp.Body.Close()
			ready = true
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !ready {
		t.Fatal("server did not start")
	}
	http.DefaultClient.CloseIdleConnections()

	send := func(path string) net.Conn {
		t.Helper()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Write([]byte("GET " + path + " HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	status := func(conn net.Conn) int {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			return 0
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	// One pool goroutine and a queue of two: three /slow requests fill it.
	for i := 0; i < 3; i++ {
		c := send("/slow")
		defer c.Close()
		time.Sleep(30 * time.Millisecond)
	}
	over := send("/slow")
	defer over.Close()
	if got := status(over); got != 503 {
		t.Fatalf("request over a full pool = %d, want 503 (0 means no answer within 1s)", got)
	}
	fast := send("/fast")
	defer fast.Close()
	if got := status(fast); got != 200 {
		t.Fatalf("inline route while the pool is full = %d, want 200 (0 means the loop was blocked)", got)
	}
}
