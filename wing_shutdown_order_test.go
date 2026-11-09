//go:build linux || darwin

package kruda

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestWingShutdownRightAfterStartWaitsForWorkers calls Shutdown as soon as the
// transport reports ready. Shutdown must not return while a worker still holds
// its listening socket.
func TestWingShutdownRightAfterStartWaitsForWorkers(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	tr := NewWingTransport(WingConfig{Workers: 4})
	app := New(WithTransport(tr))
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })
	if err := app.compile(); err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- tr.ListenAndServe(addr, app) }()
	<-tr.ready

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tr.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
		c.Close()
		t.Error("listener still accepting after Shutdown returned")
	}
	select {
	case <-served:
	case <-time.After(2 * time.Second):
		t.Fatal("ListenAndServe did not return after Shutdown")
	}
}
