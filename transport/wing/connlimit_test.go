//go:build linux || darwin

package wing

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
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
func (m *mockEngine) PostWake()                           {}
func (m *mockEngine) Wait(_ []event) (int, error)         { return 0, nil }
func (m *mockEngine) Flush() error                        { return nil }
func (m *mockEngine) Close()                              {}

// newTestWorker creates a worker with a mock engine for unit testing.
func newTestWorker(maxConns int) (*worker, *mockEngine) {
	eng := &mockEngine{}
	w := &worker{
		id:       0,
		listenFd: 100,
		eng:      eng,
		config:   Config{ReadBufSize: 4096},
		maxConns: maxConns,
		conns:    make(map[int32]*conn, 16),
		pending:  make(chan *pendingResp, 64),
	}
	return w, eng
}

// TestHandleAccept_ConnectionLimitReached verifies that new connections are
// rejected (fd closed immediately) when the worker is at its connection limit.
func TestHandleAccept_ConnectionLimitReached(t *testing.T) {
	w, eng := newTestWorker(2) // limit: 2 connections

	// Fill to capacity.
	w.conns[10] = &conn{fd: 10}
	w.conns[11] = &conn{fd: 11}

	// Accept a new fd=12 — should be rejected.
	w.handleAccept(12)

	if _, ok := w.conns[12]; ok {
		t.Error("fd 12 should NOT be in conns map after limit reached")
	}
	if len(w.conns) != 2 {
		t.Errorf("conns count = %d, want 2", len(w.conns))
	}
	// Engine should have re-armed accept.
	if eng.acceptArmed != 1 {
		t.Errorf("acceptArmed = %d, want 1", eng.acceptArmed)
	}
	// No recv should be armed for the rejected fd.
	if eng.recvArmed != 0 {
		t.Errorf("recvArmed = %d, want 0", eng.recvArmed)
	}
}

// TestHandleAccept_ConnectionLimitNotReached verifies that connections are
// accepted normally when below the limit.
func TestHandleAccept_ConnectionLimitNotReached(t *testing.T) {
	w, eng := newTestWorker(3) // limit: 3

	w.conns[10] = &conn{fd: 10}

	// Accept fd=11 — should succeed (1 < 3).
	w.handleAccept(11)

	if _, ok := w.conns[11]; !ok {
		t.Error("fd 11 should be in conns map")
	}
	if len(w.conns) != 2 {
		t.Errorf("conns count = %d, want 2", len(w.conns))
	}
	if eng.recvArmed != 1 {
		t.Errorf("recvArmed = %d, want 1 (recv should be armed for new conn)", eng.recvArmed)
	}
}

// TestHandleAccept_UnlimitedConnections verifies that maxConns=0 means no limit.
func TestHandleAccept_UnlimitedConnections(t *testing.T) {
	w, eng := newTestWorker(0) // unlimited

	// Add many connections.
	for i := int32(0); i < 100; i++ {
		w.conns[i] = &conn{fd: i}
	}

	// Accept fd=200 — should succeed even with 100 existing conns.
	w.handleAccept(200)

	if _, ok := w.conns[200]; !ok {
		t.Error("fd 200 should be in conns map (unlimited mode)")
	}
	if eng.recvArmed != 1 {
		t.Errorf("recvArmed = %d, want 1", eng.recvArmed)
	}
}

// TestHandleAccept_NegativeResult verifies that negative accept results
// don't create connection entries (R3.5).
func TestHandleAccept_NegativeResult(t *testing.T) {
	w, eng := newTestWorker(10)

	w.handleAccept(-1) // accept error

	if len(w.conns) != 0 {
		t.Errorf("conns count = %d, want 0 (no conn on accept error)", len(w.conns))
	}
	if eng.recvArmed != 0 {
		t.Errorf("recvArmed = %d, want 0", eng.recvArmed)
	}
	// Accept should still be re-armed.
	if eng.acceptArmed != 1 {
		t.Errorf("acceptArmed = %d, want 1", eng.acceptArmed)
	}
}

// TestHandleRecv_CloseOnEOF verifies that handleRecv closes the connection
// when res <= 0 (R3.6).
func TestHandleRecv_CloseOnEOF(t *testing.T) {
	w, eng := newTestWorker(10)

	w.conns[5] = &conn{fd: 5, readBuf: make([]byte, 4096)}

	w.handleRecv(5, 0) // EOF

	if _, ok := w.conns[5]; ok {
		t.Error("fd 5 should be removed from conns on EOF")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 5 {
		t.Errorf("closedFds = %v, want [5]", eng.closedFds)
	}
}

// TestHandleRecv_CloseOnNegativeResult verifies fd cleanup on read error.
func TestHandleRecv_CloseOnNegativeResult(t *testing.T) {
	w, eng := newTestWorker(10)

	w.conns[7] = &conn{fd: 7, readBuf: make([]byte, 4096)}

	w.handleRecv(7, -1) // read error

	if _, ok := w.conns[7]; ok {
		t.Error("fd 7 should be removed from conns on read error")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 7 {
		t.Errorf("closedFds = %v, want [7]", eng.closedFds)
	}
}

// TestHandleSend_CloseOnError verifies fd cleanup on send error.
func TestHandleSend_CloseOnError(t *testing.T) {
	w, eng := newTestWorker(10)

	w.conns[9] = &conn{fd: 9, sendBuf: []byte("data")}

	w.handleSend(9, -1) // send error

	if _, ok := w.conns[9]; ok {
		t.Error("fd 9 should be removed from conns on send error")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 9 {
		t.Errorf("closedFds = %v, want [9]", eng.closedFds)
	}
}

// TestHandleSend_CloseOnConnectionClose verifies fd cleanup when keepAlive=false.
func TestHandleSend_CloseOnConnectionClose(t *testing.T) {
	w, eng := newTestWorker(10)

	w.conns[3] = &conn{fd: 3, sendBuf: []byte("OK"), keepAlive: false}

	w.handleSend(3, 2) // sent all 2 bytes, keepAlive=false

	if _, ok := w.conns[3]; ok {
		t.Error("fd 3 should be removed when keepAlive=false after send completes")
	}
	if len(eng.closedFds) != 1 || eng.closedFds[0] != 3 {
		t.Errorf("closedFds = %v, want [3]", eng.closedFds)
	}
}

// panicHandler panics during request handling to test R3.7 recovery.
type panicHandler struct{}

func (h *panicHandler) ServeKruda(_ transport.ResponseWriter, _ transport.Request) {
	panic("test panic")
}

// TestHandleRecv_PanicRecoveryClosesFd verifies that a panic during request
// handling closes the connection fd (R3.7).
func TestHandleRecv_PanicRecoveryClosesFd(t *testing.T) {
	w, eng := newTestWorker(10)
	w.handler = &panicHandler{}

	// Build a valid HTTP request in the read buffer.
	req := []byte("GET / HTTP/1.1\r\nHost: test\r\n\r\n")
	c := &conn{fd: 20, readBuf: make([]byte, 4096)}
	copy(c.readBuf, req)
	w.conns[20] = c

	// handleRecv should not panic — the recovery should catch it.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("handleRecv panicked — recovery failed: %v", r)
			}
		}()
		w.handleRecv(20, int32(len(req)))
	}()

	// The inner recovery catches the handler panic and writes a 500 response.
	// The send should be armed (inner recovery writes 500, then send is submitted).
	if eng.sendArmed != 1 {
		t.Errorf("sendArmed = %d, want 1 (500 response should be sent)", eng.sendArmed)
	}
}

// TestHandleAccept_AtExactLimit verifies behavior when conns == maxConns.
func TestHandleAccept_AtExactLimit(t *testing.T) {
	w, _ := newTestWorker(1)

	w.conns[10] = &conn{fd: 10}

	// At limit (1/1) — new accept should be rejected.
	w.handleAccept(11)

	if _, ok := w.conns[11]; ok {
		t.Error("fd 11 should NOT be in conns (at exact limit)")
	}
	if len(w.conns) != 1 {
		t.Errorf("conns count = %d, want 1", len(w.conns))
	}
}

// TestWorkerMaxConnsFromConfig verifies that maxConns is set from Config.
func TestWorkerMaxConnsFromConfig(t *testing.T) {
	cfg := Config{
		Workers:           1,
		MaxConnsPerWorker: 5000,
	}
	cfg.defaults()

	// We can't call newWorker directly (needs real engine), so verify
	// the Config field is properly defined and defaults work.
	if cfg.MaxConnsPerWorker != 5000 {
		t.Errorf("MaxConnsPerWorker = %d, want 5000", cfg.MaxConnsPerWorker)
	}

	// Verify zero means unlimited.
	cfg2 := Config{}
	cfg2.defaults()
	if cfg2.MaxConnsPerWorker != 0 {
		t.Errorf("default MaxConnsPerWorker = %d, want 0 (unlimited)", cfg2.MaxConnsPerWorker)
	}
}

// TestTransportConnectionLimit is an integration test that verifies the
// connection limit works end-to-end with a real transport.
func TestTransportConnectionLimit(t *testing.T) {
	skipIfNoEngine(t)

	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	tr := New(Config{
		Workers:           1,
		RingSize:          64,
		ReadBufSize:       4096,
		MaxConnsPerWorker: 2, // very low limit for testing
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var shutdown atomic.Bool
	go func() {
		defer wg.Done()
		tr.ListenAndServe(addr, &echoHandler{})
	}()

	// Wait for server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	defer func() {
		shutdown.Store(true)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tr.Shutdown(ctx)
		wg.Wait()
	}()

	// Open 2 connections (at limit) — these should work.
	conns := make([]net.Conn, 0, 3)
	for i := 0; i < 2; i++ {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			t.Fatalf("connection %d failed: %v", i, err)
		}
		conns = append(conns, c)
	}

	// Give the server time to accept both connections.
	time.Sleep(100 * time.Millisecond)

	// 3rd connection — should be accepted at TCP level but the server
	// will close it immediately when it processes the accept.
	c3, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		// Connection refused is also acceptable — means the fd was closed.
		t.Logf("3rd connection refused (expected): %v", err)
	} else {
		// The server should close this connection quickly.
		c3.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1)
		_, readErr := c3.Read(buf)
		if readErr == nil {
			t.Error("expected 3rd connection to be closed by server")
		}
		c3.Close()
	}

	// Clean up held connections.
	for _, c := range conns {
		c.Close()
	}
}
