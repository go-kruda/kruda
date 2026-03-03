//go:build linux || darwin

package wing

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// mockEngine implements engine for unit testing worker logic.
type mockEngine struct {
	acceptArmed int
	recvArmed   int
	sendArmed   int
	closedFds   []int32
	flushed     int
}

func (m *mockEngine) Init(_ engineConfig) error           { return nil }
func (m *mockEngine) SubmitAccept(_ int)                  { m.acceptArmed++ }
func (m *mockEngine) SubmitRecv(_ int32, _ []byte, _ int) { m.recvArmed++ }
func (m *mockEngine) SubmitSend(_ int32, _ []byte)        { m.sendArmed++ }
func (m *mockEngine) SubmitClose(fd int32)                { m.closedFds = append(m.closedFds, fd) }
func (m *mockEngine) SubmitPipeRecv(_ int, _ []byte)      {}
func (m *mockEngine) RegisterConn(_ int32, _ unsafe.Pointer) {}
func (m *mockEngine) PostWake()                           {}
func (m *mockEngine) Wait(_ []event) (int, error)         { return 0, nil }
func (m *mockEngine) WaitNonBlock(_ []event) (int, error) { return 0, nil }
func (m *mockEngine) Flush() error                        { return nil }
func (m *mockEngine) Close()                              {}

func newTestWorker(maxConns int) (*worker, *mockEngine) {
	eng := &mockEngine{}
	w := &worker{
		id:       0,
		listenFd: 100,
		eng:      eng,
		config:   Config{ReadBufSize: 4096},
		maxConns: maxConns,
		conns:    make(map[int32]*conn, 16),
	}
	return w, eng
}

func newTestConn(fd int32) *conn {
	return &conn{fd: fd, readBuf: make([]byte, 4096)}
}

func TestHandleAccept_ConnectionLimitReached(t *testing.T) {
	w, eng := newTestWorker(2)
	w.conns[10] = newTestConn(10)
	w.conns[11] = newTestConn(11)

	w.handleAccept(event{Op: opAccept, Res: 12, Flags: cqeFMore})

	if _, has := w.conns[12]; has {
		t.Error("fd 12 should NOT be in conns map after limit reached")
	}
	if len(w.conns) != 2 {
		t.Errorf("conns count = %d, want 2", len(w.conns))
	}
	// v1.5: no duplicate SubmitAccept when cqeFMore is set and at limit
	if eng.acceptArmed != 0 {
		t.Errorf("acceptArmed = %d, want 0 (cqeFMore set, no re-arm needed)", eng.acceptArmed)
	}
	if eng.recvArmed != 0 {
		t.Errorf("recvArmed = %d, want 0", eng.recvArmed)
	}
}

func TestHandleAccept_ConnectionLimitNotReached(t *testing.T) {
	w, eng := newTestWorker(3)
	w.conns[10] = newTestConn(10)

	w.handleAccept(event{Op: opAccept, Res: 11, Flags: cqeFMore})

	if _, has := w.conns[11]; !has {
		t.Error("fd 11 should be in conns map")
	}
	if len(w.conns) != 2 {
		t.Errorf("conns count = %d, want 2", len(w.conns))
	}
	if eng.recvArmed != 1 {
		t.Errorf("recvArmed = %d, want 1", eng.recvArmed)
	}
}

func TestHandleAccept_UnlimitedConnections(t *testing.T) {
	w, eng := newTestWorker(0)
	for i := int32(0); i < 100; i++ {
		w.conns[i] = newTestConn(i)
	}

	w.handleAccept(event{Op: opAccept, Res: 200, Flags: cqeFMore})

	if _, has := w.conns[200]; !has {
		t.Error("fd 200 should be in conns map (unlimited mode)")
	}
	if eng.recvArmed != 1 {
		t.Errorf("recvArmed = %d, want 1", eng.recvArmed)
	}
}

func TestHandleAccept_NegativeResult(t *testing.T) {
	w, eng := newTestWorker(10)

	w.handleAccept(event{Op: opAccept, Res: -1, Flags: cqeFMore})

	if len(w.conns) != 0 {
		t.Errorf("conns count = %d, want 0", len(w.conns))
	}
	if eng.recvArmed != 0 {
		t.Errorf("recvArmed = %d, want 0", eng.recvArmed)
	}
	if eng.acceptArmed != 1 {
		t.Errorf("acceptArmed = %d, want 1", eng.acceptArmed)
	}
}

func TestHandleAccept_AtExactLimit(t *testing.T) {
	w, _ := newTestWorker(1)
	w.conns[10] = newTestConn(10)

	w.handleAccept(event{Op: opAccept, Res: 11, Flags: cqeFMore})

	if _, has := w.conns[11]; has {
		t.Error("fd 11 should NOT be in conns (at exact limit)")
	}
	if len(w.conns) != 1 {
		t.Errorf("conns count = %d, want 1", len(w.conns))
	}
}

func TestHandleRecv_CloseOnEOF(t *testing.T) {
	w, eng := newTestWorker(10)
	w.conns[5] = newTestConn(5)

	w.handleRecv(event{Op: opRecv, Fd: 5, Res: 0})

	if _, has := w.conns[5]; has {
		t.Error("fd 5 should be removed on EOF")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 5 {
		t.Errorf("closedFds = %v, want [5]", eng.closedFds)
	}
}

func TestHandleRecv_CloseOnNegativeResult(t *testing.T) {
	w, eng := newTestWorker(10)
	w.conns[7] = newTestConn(7)

	w.handleRecv(event{Op: opRecv, Fd: 7, Res: -1})

	if _, has := w.conns[7]; has {
		t.Error("fd 7 should be removed on read error")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 7 {
		t.Errorf("closedFds = %v, want [7]", eng.closedFds)
	}
}

func TestHandleSend_CloseOnError(t *testing.T) {
	w, eng := newTestWorker(10)
	c := newTestConn(9)
	c.sendBuf = []byte("data")
	w.conns[9] = c

	w.handleSend(event{Op: opSend, Fd: 9, Res: -1})

	if _, has := w.conns[9]; has {
		t.Error("fd 9 should be removed on send error")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 9 {
		t.Errorf("closedFds = %v, want [9]", eng.closedFds)
	}
}

func TestHandleSend_CloseOnConnectionClose(t *testing.T) {
	w, eng := newTestWorker(10)
	c := newTestConn(3)
	c.sendBuf = []byte("OK")
	c.keepAlive = false
	w.conns[3] = c

	w.handleSend(event{Op: opSend, Fd: 3, Res: 2})

	if _, has := w.conns[3]; has {
		t.Error("fd 3 should be removed when keepAlive=false after send")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 3 {
		t.Errorf("closedFds = %v, want [3]", eng.closedFds)
	}
}

func TestHandleRecv_PanicRecoveryClosesFd(t *testing.T) {
	// v1.5: panic recovery is in handlerWorker goroutine, not ioLoop.
	// ioLoop itself won't panic on recv — this is covered by integration tests.
	t.Skip("v1.5: panic recovery in handlerWorker — see integration tests")
}

func TestWorkerMaxConnsFromConfig(t *testing.T) {
	cfg := Config{Workers: 1, MaxConnsPerWorker: 5000}
	cfg.defaults()
	if cfg.MaxConnsPerWorker != 5000 {
		t.Errorf("MaxConnsPerWorker = %d, want 5000", cfg.MaxConnsPerWorker)
	}
	cfg2 := Config{}
	cfg2.defaults()
	if cfg2.MaxConnsPerWorker != 0 {
		t.Errorf("default MaxConnsPerWorker = %d, want 0 (unlimited)", cfg2.MaxConnsPerWorker)
	}
}

func TestTransportConnectionLimit(t *testing.T) {
	skipIfNoEngine(t)

	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	tr := New(Config{
		Workers:           1,
		RingSize:          64,
		ReadBufSize:       4096,
		MaxConnsPerWorker: 2,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tr.ListenAndServe(addr, &echoHandler{})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tr.Shutdown(ctx)
		wg.Wait()
	}()

	conns := make([]net.Conn, 0, 2)
	for i := 0; i < 2; i++ {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			t.Fatalf("connection %d failed: %v", i, err)
		}
		conns = append(conns, c)
	}
	time.Sleep(100 * time.Millisecond)

	c3, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Logf("3rd connection refused (expected): %v", err)
	} else {
		c3.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1)
		_, readErr := c3.Read(buf)
		if readErr == nil {
			t.Error("expected 3rd connection to be closed by server")
		}
		c3.Close()
	}

	for _, c := range conns {
		c.Close()
	}
}
