//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingShutdownPoolWaitingOnTakeoverWriter runs a Pool handler that waits
// for a hijacked writer, which is blocked on a client that does not read.
// Shutdown must cut that writer while it waits for the Pool, or neither ends.
func TestWingShutdownPoolWaitingOnTakeoverWriter(t *testing.T) {
	t.Setenv("KRUDA_WORKERS", "1") // the Pool handler must block the writer's own worker
	writing, writerDone, poolEntered := make(chan struct{}), make(chan struct{}), make(chan struct{})
	app := New(Wing())
	app.Get("/ws", func(c *Ctx) error {
		nc, _, err := c.ResponseWriter().(http.Hijacker).Hijack()
		if err != nil {
			return nil
		}
		defer close(writerDone)
		close(writing)
		big := make([]byte, 64<<10)
		for {
			if _, err := nc.Write(big); err != nil {
				return nil
			}
		}
	}, Hijack)
	app.Get("/pool", func(c *Ctx) error {
		close(poolEntered)
		<-writerDone // ignores its context, as a hub broadcasting to sockets may
		return c.Text("ok")
	}, Arrow)
	app.Compile()
	addr, _ := startWingHijackServer(t, app)
	ws := dialRawHijack(t, addr)
	defer ws.Close()
	if tc, ok := ws.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(1024)
	}
	select {
	case <-writing:
	case <-time.After(3 * time.Second):
		t.Fatal("writer never ran")
	}
	pool, _ := framingDial(t, addr)
	if _, err := pool.Write([]byte(framingGet("/pool"))); err != nil {
		t.Fatal(err)
	}
	select {
	case <-poolEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("Pool handler never ran")
	}
	time.Sleep(100 * time.Millisecond) // the writer fills the socket and blocks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := app.transport.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown after %v: %v", time.Since(start), err)
	}
	// Sooner than the grace means the writer's socket was cut before the Pool
	// drained, so the deadlock was never set up.
	if took := time.Since(start); took < shutdownTakeoverGrace*8/10 || took > 3*time.Second {
		t.Fatalf("Shutdown took %v, want about %v", took, shutdownTakeoverGrace)
	}
}

// TestWingFramedTakeoverCloseFreesSlot closes a Takeover connection after a
// framed response, with nothing unread and a client that keeps its write side
// open. That close needs no linger, so with one connection allowed the next
// client is served at once.
func TestWingFramedTakeoverCloseFreesSlot(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Connection", "close")
		_, _ = w.Write([]byte("ok"))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, MaxConns: 1, DefaultPreset: Spear}, handler)
	defer stop()
	var conn net.Conn
	var br *bufio.Reader
	for attempt := 0; conn == nil; attempt++ { // the start-up probe may still hold the slot
		if attempt == 20 {
			t.Fatal("never admitted")
		}
		c, r := framingDial(t, addr)
		if _, err := io.WriteString(c, framingGet("/")); err != nil {
			t.Fatal(err)
		}
		if res, err := http.ReadResponse(r, nil); err == nil {
			_, _ = io.ReadAll(res.Body)
			conn, br = c, r
		} else {
			c.Close()
			time.Sleep(50 * time.Millisecond)
		}
	}
	if _, err := br.ReadByte(); err != io.EOF {
		t.Fatalf("after the response: %v, want EOF", err)
	}
	time.Sleep(100 * time.Millisecond) // well inside lingerDelay; conn stays open
	next, nbr := framingDial(t, addr)
	if _, err := io.WriteString(next, framingGet("/")); err != nil {
		t.Fatal(err)
	}
	if _, err := http.ReadResponse(nbr, nil); err != nil {
		t.Fatalf("next client not served while the first kept its write side open: %v", err)
	}
}
