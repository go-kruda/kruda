//go:build linux || darwin

package kruda

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
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

// TestWingShutdownCancelsPoolHandlerContext runs a Pool handler that waits on
// its request context. Shutdown must cancel that context instead of waiting
// for the handler to give up on its own.
func TestWingShutdownCancelsPoolHandlerContext(t *testing.T) {
	entered := make(chan struct{})
	cancelled := make(chan bool, 1)
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
			cancelled <- true
		case <-time.After(5 * time.Second):
			cancelled <- false
		}
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow}, handler)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler never ran")
	}
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown waited for a Pool handler blocked on its context")
	}
	if !<-cancelled {
		t.Fatal("Pool handler context was not cancelled by Shutdown")
	}
}

// TestWingShutdownDeliversFinishingPoolResponse runs a Pool handler that is
// still working when Shutdown starts and ignores its context. Its response
// must still reach the client instead of hitting a socket already shut down.
func TestWingShutdownDeliversFinishingPoolResponse(t *testing.T) {
	entered := make(chan struct{})
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		close(entered)
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("late"))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Arrow}, handler)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler never ran")
	}
	go stop()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got, _ := io.ReadAll(conn)
	if !strings.Contains(string(got), "late") {
		t.Fatalf("response lost during shutdown, got %q", got)
	}
}

// TestWingShutdownKeepsFinishedTakeoverFD finishes a Takeover while the Pool is
// still draining. Its fd must stay open until Shutdown has shut every socket:
// once closed, the number can belong to an unrelated socket.
func TestWingShutdownKeepsFinishedTakeoverFD(t *testing.T) {
	socks, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(socks[1])
	file := os.NewFile(uintptr(socks[0]), "takeover")
	w, _ := newTestWorker(0)
	w.evfd, w.listenFd = -1, -1
	w.doneCh = make(chan doneMsg, 1)
	c := newTestConn(int32(socks[0]))
	c.pending, c.takenOver = 1, true
	w.conns[c.fd] = c

	openDuringDrain := make(chan bool, 1)
	w.pool = &workerPool{jobs: make(chan handlerJob)}
	w.pool.wg.Add(1)
	go func() {
		defer w.pool.wg.Done()
		for range w.pool.jobs {
		}
		w.doneCh <- doneMsg{fd: c.fd, file: file}
		time.Sleep(50 * time.Millisecond) // the drain has taken the completion
		_, err := file.Stat()
		openDuringDrain <- err == nil
	}()
	// A Spawn handler that never returns must not keep the File open: only the
	// raw shutdown pass needs it.
	closedBeforeDispatch := make(chan bool, 1)
	w.dispatchWG.Add(1)
	go func() {
		defer w.dispatchWG.Done()
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			if _, err := file.Stat(); err != nil {
				closedBeforeDispatch <- true
				return
			}
		}
		closedBeforeDispatch <- false
	}()
	w.cleanup()
	if !<-openDuringDrain {
		t.Fatal("Takeover fd closed while the Pool was draining")
	}
	if !<-closedBeforeDispatch {
		t.Fatal("Takeover fd kept open while a dispatched handler ran")
	}
}
