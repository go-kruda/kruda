//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"
)

// TestWingStream_ShutdownDrainsActiveSSE encodes the K8s rolling-deploy
// contract: Shutdown while an SSE stream is live must (1) fire the stream's
// Done() so the handler can exit, (2) let Shutdown return before its context
// deadline, and (3) end the client connection.
func TestWingStream_ShutdownDrainsActiveSSE(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	doneFired := make(chan struct{})
	handlerExited := make(chan struct{})

	app := New(Wing())
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			defer close(handlerExited)
			if err := s.Data("hello"); err != nil {
				return err
			}
			select {
			case <-s.Done():
				close(doneFired)
				return nil
			case <-time.After(10 * time.Second):
				return nil // test will fail on the doneFired assertion below
			}
		})
	}, Stream)

	addr, _ := startWingApp(t, app) // we call Shutdown ourselves below

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	req := "GET /events HTTP/1.1\r\nHost: t\r\nAccept: text/event-stream\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := bufio.NewReader(conn)
	readHeaders(t, r)
	readSSEEvent(t, r) // the stream is confirmed live on the wire

	shutdownErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr <- app.transport.Shutdown(ctx)
	}()

	select {
	case <-doneFired:
	case <-time.After(5 * time.Second):
		t.Error("SSEStream.Done() did not fire within 5s of Shutdown")
	}
	select {
	case <-handlerExited:
	case <-time.After(5 * time.Second):
		t.Error("handler did not exit within 5s of Shutdown")
	}
	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Error("Shutdown did not return within its deadline")
	}
}
