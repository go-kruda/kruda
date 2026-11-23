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

// TestWingShutdownClosesFinishedTakeoverFD finishes a Takeover while a
// dispatched handler never returns. Shutdown must still close its File.
func TestWingShutdownClosesFinishedTakeoverFD(t *testing.T) {
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

	closedWhileRunning := make(chan bool, 1)
	w.dispatchWG.Add(1)
	go func() {
		defer w.dispatchWG.Done()
		w.doneCh <- doneMsg{fd: c.fd, file: file}
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			if _, err := file.Stat(); err != nil {
				closedWhileRunning <- true
				return
			}
		}
		closedWhileRunning <- false
	}()
	w.cleanup()
	if !<-closedWhileRunning {
		t.Fatal("Takeover fd kept open while a dispatched handler ran")
	}
}

// TestWingShutdownNeverShutsARecycledFD finishes a Takeover while the Pool is
// still draining. Its fd number may then belong to an unrelated socket, which
// Shutdown must not touch.
func TestWingShutdownNeverShutsARecycledFD(t *testing.T) {
	socks, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(socks[1])
	other, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(other[0])
	defer syscall.Close(other[1])

	fd := socks[0]
	file := os.NewFile(uintptr(fd), "takeover")
	w, _ := newTestWorker(0)
	w.evfd, w.listenFd = -1, -1
	w.doneCh = make(chan doneMsg, 1)
	c := newTestConn(int32(fd))
	c.pending, c.takenOver = 1, true
	w.conns[c.fd] = c

	// The Pool finishes only after the Takeover's fd was closed and reused.
	recycled, closedDuringDrain := -1, false
	w.pool = &workerPool{jobs: make(chan handlerJob)}
	w.pool.wg.Add(1)
	go func() {
		defer w.pool.wg.Done()
		for range w.pool.jobs {
		}
		w.doneCh <- doneMsg{fd: c.fd, file: file}
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			if _, err := file.Stat(); err != nil {
				closedDuringDrain = true
				n, _, errno := syscall.RawSyscall(syscall.SYS_FCNTL, uintptr(other[0]), syscall.F_DUPFD_CLOEXEC, uintptr(fd))
				if errno == 0 {
					recycled = int(n)
				}
				return
			}
		}
	}()
	w.cleanup()
	if recycled >= 0 {
		defer syscall.Close(recycled)
	}
	if !closedDuringDrain {
		t.Fatal("finished Takeover File kept open while the Pool drained")
	}
	if recycled != fd {
		t.Skipf("setup: fd %d was closed but the duplicate landed on %d", fd, recycled)
	}
	if _, err := syscall.Write(recycled, []byte("x")); err != nil {
		t.Fatalf("Shutdown shut down an unrelated socket on the recycled fd: %v", err)
	}
}

// TestWingRepeatedShutdownWakesOnce calls Shutdown twice. Only the first may
// wake the workers: a stopped worker has closed its wake fd.
func TestWingRepeatedShutdownWakesOnce(t *testing.T) {
	w, eng := newTestWorker(0)
	tr := &Transport{workers: []*worker{w}, ready: make(chan struct{})}
	close(tr.ready)
	for range 2 {
		if err := tr.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if eng.wakes != 1 {
		t.Fatalf("workers woken %d times", eng.wakes)
	}
}

type notifyCloseEngine struct {
	*mockEngine
	closed chan int32
}

func (e notifyCloseEngine) SubmitClose(fd int32) { e.closed <- fd }

// TestWingShutdownClosesLingering stops a worker with a connection lingering
// after its final response, while a dispatched handler still runs. The stopped
// loop never sweeps, so Shutdown must close that connection first.
func TestWingShutdownClosesLingering(t *testing.T) {
	w, eng := newTestWorker(0)
	w.evfd, w.listenFd = -1, -1
	w.doneCh = make(chan doneMsg, 1)
	closed := make(chan int32, 1)
	w.eng = notifyCloseEngine{eng, closed}
	c := newTestConn(-1)
	c.lingerUntil = time.Now().Add(time.Hour).UnixNano()
	w.lingering = 1
	w.conns[c.fd] = c
	closedWhileRunning := make(chan bool, 1)
	w.dispatchWG.Add(1)
	go func() {
		defer w.dispatchWG.Done()
		select {
		case <-closed:
			closedWhileRunning <- true
		case <-time.After(time.Second):
			closedWhileRunning <- false
		}
	}()
	w.cleanup()
	if !<-closedWhileRunning {
		t.Fatal("lingering connection kept open while Shutdown waited")
	}
}

// TestWingShutdownSkipsStoppedWorker calls Shutdown for a worker that has
// already stopped and closed its wake fd. Waking it would write to whatever
// reused that number.
func TestWingShutdownSkipsStoppedWorker(t *testing.T) {
	w, eng := newTestWorker(0)
	w.evfd, w.listenFd = -1, -1
	w.doneCh = make(chan doneMsg, 1)
	w.cleanup()
	tr := &Transport{workers: []*worker{w}, ready: make(chan struct{})}
	close(tr.ready)
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if eng.wakes != 0 {
		t.Fatalf("stopped worker woken %d times", eng.wakes)
	}
}
