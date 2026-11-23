//go:build linux || darwin

package kruda

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestWingShutdownPoolWaitingOnTakeoverWriter runs a Pool handler that waits
// for a hijacked writer, which is blocked on a client that does not read.
// Shutdown must cut that writer while it waits for the Pool, or neither ends.
func TestWingShutdownPoolWaitingOnTakeoverWriter(t *testing.T) {
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
	<-writing
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
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("Shutdown took %v", took)
	}
}
