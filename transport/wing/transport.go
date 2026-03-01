package wing

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/go-kruda/kruda/transport"
)

// Compile-time interface assertion.
var _ transport.Transport = (*Transport)(nil)

// Config holds Wing transport settings.
type Config struct {
	Workers     int    // worker count (0 = NumCPU)
	RingSize    uint32 // engine ring/event capacity (0 = 4096)
	ReadBufSize int    // per-conn read buffer (0 = 8192)
}

func (c *Config) defaults() {
	if c.Workers <= 0 {
		c.Workers = runtime.NumCPU()
	}
	if c.RingSize == 0 {
		c.RingSize = 4096
	}
	if c.ReadBufSize <= 0 {
		c.ReadBufSize = 8192
	}
}

// Transport implements transport.Transport using platform-native async I/O.
// Linux: io_uring, macOS: kqueue, Windows: IOCP.
type Transport struct {
	config   Config
	workers  []*worker
	shutdown atomic.Bool
	wg       sync.WaitGroup
	ready    chan struct{} // closed when workers are initialized
}

// New creates a new Wing transport.
func New(cfg Config) *Transport {
	cfg.defaults()
	return &Transport{config: cfg, ready: make(chan struct{})}
}

// ListenAndServe creates listen sockets and starts workers.
func (t *Transport) ListenAndServe(addr string, handler transport.Handler) error {
	t.workers = make([]*worker, t.config.Workers)

	for i := range t.workers {
		fd, err := createListenFd(addr)
		if err != nil {
			t.cleanupWorkers(i)
			return fmt.Errorf("wing: listen: %w", err)
		}
		w, err := newWorker(i, fd, t.config, handler)
		if err != nil {
			closeFd(fd)
			t.cleanupWorkers(i)
			return fmt.Errorf("wing: worker %d: %w", i, err)
		}
		t.workers[i] = w
	}

	close(t.ready)

	for _, w := range t.workers {
		t.wg.Add(1)
		go func(w *worker) {
			defer t.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					w.cleanup()
				}
			}()
			w.run(&t.shutdown)
		}(w)
	}

	t.wg.Wait()
	return nil
}

// Serve starts on an existing listener. Wing manages its own sockets,
// so we extract the address and create new listen fds.
func (t *Transport) Serve(ln net.Listener, handler transport.Handler) error {
	addr := ln.Addr().String()
	_ = ln.Close()
	return t.ListenAndServe(addr, handler)
}

// Shutdown signals all workers to stop.
func (t *Transport) Shutdown(_ context.Context) error {
	<-t.ready
	t.shutdown.Store(true)
	for _, w := range t.workers {
		if w != nil {
			w.wake()
		}
	}
	t.wg.Wait()
	return nil
}

func (t *Transport) cleanupWorkers(upTo int) {
	for j := 0; j < upTo; j++ {
		if t.workers[j] != nil {
			t.workers[j].close()
		}
	}
}

// ----------------------------- worker -----------------------------

// maxEventsPerWait is the maximum events drained per Wait call.
const maxEventsPerWait = 128

type worker struct {
	id       int
	listenFd int
	eng      engine
	handler  transport.Handler
	config   Config
	conns    map[int32]*conn
	events   [maxEventsPerWait]event // reused per Wait call
	pipeR    int
	pipeW    int
	pipeBuf  [8]byte
	pending  chan *pendingResp
}

type conn struct {
	fd        int32
	readBuf   []byte
	readN     int
	sendBuf   []byte // held until send completes
	keepAlive bool
}

type pendingResp struct {
	fd        int32
	data      []byte
	keepAlive bool
}

func newWorker(id, listenFd int, cfg Config, handler transport.Handler) (*worker, error) {
	eng := newEngine()

	pipeR, pipeW, err := createPipe()
	if err != nil {
		eng.Close()
		return nil, err
	}

	if err := eng.Init(engineConfig{RingSize: cfg.RingSize, PipeW: pipeW}); err != nil {
		if pipeR >= 0 {
			closeFd(pipeR)
		}
		if pipeW >= 0 {
			closeFd(pipeW)
		}
		return nil, err
	}

	return &worker{
		id:       id,
		listenFd: listenFd,
		eng:      eng,
		handler:  handler,
		config:   cfg,
		conns:    make(map[int32]*conn, 1024),
		pipeR:    pipeR,
		pipeW:    pipeW,
		pending:  make(chan *pendingResp, 4096),
	}, nil
}

func (w *worker) run(shutdown *atomic.Bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w.eng.SubmitAccept(w.listenFd)
	w.eng.SubmitPipeRecv(w.pipeR, w.pipeBuf[:])
	w.eng.Flush()

	for !shutdown.Load() {
		n, err := w.eng.Wait(w.events[:])
		if err != nil {
			if shutdown.Load() {
				break
			}
			continue
		}

		for i := 0; i < n; i++ {
			w.processEvent(w.events[i])
		}

		w.eng.Flush()
	}

	w.cleanup()
}

func (w *worker) processEvent(ev event) {
	switch ev.Op {
	case opAccept:
		w.handleAccept(ev.Res)
	case opRecv:
		w.handleRecv(ev.Fd, ev.Res)
	case opSend:
		w.handleSend(ev.Fd, ev.Res)
	case opClose:
		delete(w.conns, ev.Fd)
	case opWake:
		w.handleWake()
	}
}

func (w *worker) handleAccept(res int32) {
	w.eng.SubmitAccept(w.listenFd) // always re-arm

	if res < 0 {
		return
	}
	fd := res
	setTCPNodelay(fd)

	c := &conn{
		fd:      fd,
		readBuf: make([]byte, w.config.ReadBufSize),
	}
	w.conns[fd] = c
	w.eng.SubmitRecv(c.fd, c.readBuf, c.readN)
}

func (w *worker) handleRecv(fd, res int32) {
	c, ok := w.conns[fd]
	if !ok {
		return
	}

	if res <= 0 {
		w.closeConn(fd)
		return
	}

	c.readN += int(res)

	req, ok := parseHTTPRequest(c.readBuf[:c.readN])
	if !ok {
		if c.readN >= len(c.readBuf) {
			w.closeConn(fd) // buffer full, no valid request
			return
		}
		w.eng.SubmitRecv(fd, c.readBuf, c.readN) // need more data
		return
	}

	// OPTIMIZATION: Inline handler — run handler directly in event loop.
	// Eliminates goroutine spawn + channel + pipe syscall per request.
	//
	// SAFETY: recover from handler panics to protect the event loop.
	resp := acquireResponse()
	func() {
		defer func() {
			if r := recover(); r != nil {
				resp.WriteHeader(500)
				resp.Write([]byte("Internal Server Error\n"))
			}
		}()
		w.handler.ServeKruda(resp, req)
	}()

	data := resp.buildZeroCopy()
	ka := req.keepAlive
	releaseResponse(resp)

	c.sendBuf = data
	c.keepAlive = ka
	c.readN = 0
	w.eng.SubmitSend(fd, data)
}

// dispatchAsync falls back to goroutine dispatch (for future slow-handler detection).
func (w *worker) dispatchAsync(fd int32, req *wingRequest) {
	resp := acquireResponse()
	func() {
		defer func() {
			if r := recover(); r != nil {
				resp.WriteHeader(500)
				resp.Write([]byte("Internal Server Error\n"))
			}
		}()
		w.handler.ServeKruda(resp, req)
	}()

	data := resp.buildZeroCopy()
	ka := req.keepAlive
	releaseResponse(resp)

	w.pending <- &pendingResp{fd: fd, data: data, keepAlive: ka}
	w.eng.PostWake() // wake event loop
}

func (w *worker) handleSend(fd, res int32) {
	c, ok := w.conns[fd]
	if !ok {
		return
	}

	if res < 0 {
		w.closeConn(fd)
		return
	}

	sent := int(res)
	if sent < len(c.sendBuf) {
		c.sendBuf = c.sendBuf[sent:]
		w.eng.SubmitSend(fd, c.sendBuf)
		return
	}

	c.sendBuf = nil
	if !c.keepAlive {
		w.closeConn(fd)
		return
	}

	c.readN = 0
	w.eng.SubmitRecv(fd, c.readBuf, 0)
}

func (w *worker) handleWake() {
	w.eng.SubmitPipeRecv(w.pipeR, w.pipeBuf[:]) // re-arm pipe

	for {
		select {
		case pr := <-w.pending:
			c, ok := w.conns[pr.fd]
			if !ok {
				continue
			}
			c.sendBuf = pr.data
			c.keepAlive = pr.keepAlive
			c.readN = 0
			w.eng.SubmitSend(pr.fd, pr.data)
		default:
			return
		}
	}
}

// closeConn eagerly removes the connection from the map and submits a close.
// On Linux (io_uring), close is async and opClose fires later — the extra
// delete is a harmless no-op. On macOS (kqueue) and Windows (IOCP), close is
// synchronous and no opClose event is emitted, so the eager delete prevents
// a map leak.
func (w *worker) closeConn(fd int32) {
	delete(w.conns, fd)
	w.eng.SubmitClose(fd)
}

func (w *worker) wake() {
	w.eng.PostWake()
}

func (w *worker) cleanup() {
	for fd := range w.conns {
		closeFd(int(fd))
	}
	w.eng.Close()
	if w.pipeR >= 0 {
		closeFd(w.pipeR)
	}
	if w.pipeW >= 0 {
		closeFd(w.pipeW)
	}
	closeFd(w.listenFd)
}

func (w *worker) close() { w.cleanup() }
