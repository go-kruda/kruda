//go:build linux || darwin

package kruda

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-kruda/kruda/transport"
)

var _ transport.Transport = (*Transport)(nil)

// needsPool returns true if any route uses Pool dispatch (requires pre-allocated goroutine pool).
// Spawn dispatch uses ad-hoc goroutines and does NOT require a pool.
func (c *WingConfig) needsPool() bool {
	d := c.DefaultFeather.Dispatch
	if d == 0 {
		d = Inline
	}
	if d == Pool {
		return true
	}
	for _, f := range c.Feathers {
		fd := f.Dispatch
		if fd == 0 {
			fd = Inline
		}
		if fd == Pool {
			return true
		}
	}
	return false
}

// needsAsync returns true if any route uses Pool or Spawn dispatch.
// When true, doneCh and pipe wake are needed but LockOSThread is still safe.
func (c *WingConfig) needsAsync() bool {
	d := c.DefaultFeather.Dispatch
	if d == 0 {
		d = Inline
	}
	if d == Pool || d == Spawn || d == Takeover {
		return true
	}
	for _, f := range c.Feathers {
		fd := f.Dispatch
		if fd == 0 {
			fd = Inline
		}
		if fd == Pool || fd == Spawn || fd == Takeover {
			return true
		}
	}
	return false
}

// Transport is the Wing async I/O transport. It pins one worker per CPU
// (configurable via WingConfig.Workers), each running its own epoll/kqueue
// loop on a dedicated OS thread. Implements transport.Transport and
// transport.FeatherConfigurator.
type Transport struct {
	config   WingConfig
	workers  []*worker
	shutdown atomic.Bool
	wg       sync.WaitGroup
	ready    chan struct{}
}

// NewWingTransport builds a Wing transport from cfg, applying defaults to any
// zero fields. The transport does not bind a listener until ListenAndServe is
// called.
func NewWingTransport(cfg WingConfig) *Transport {
	cfg.defaults()
	return &Transport{config: cfg, ready: make(chan struct{})}
}

// ListenAndServe binds one SO_REUSEPORT listener per worker on addr, starts
// the workers (each pinned to its own OS thread), and blocks until Shutdown
// is called or all workers exit. Returns an error if any listener or worker
// fails to initialize; partially started workers are torn down before
// returning.
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
			w.run(&t.shutdown)
		}(w)
	}
	t.wg.Wait()
	return nil
}

// Serve adapts a stdlib net.Listener to Wing by extracting its address,
// closing the supplied listener, and re-binding via ListenAndServe so each
// worker can own a SO_REUSEPORT socket. Use ListenAndServe directly when you
// don't already have a *net.Listener.
func (t *Transport) Serve(ln net.Listener, handler transport.Handler) error {
	addr := ln.Addr().String()
	_ = ln.Close()
	return t.ListenAndServe(addr, handler)
}

// SetRouteFeather implements transport.FeatherConfigurator. The feather
// argument is typed as `any` at the interface boundary; route registration
// passes a `*Feather` (so a missing hint is nil); we accept either form for
// backward compatibility with anything still passing `Feather` by value.
func (t *Transport) SetRouteFeather(method, path string, feather any) {
	var f Feather
	switch v := feather.(type) {
	case *Feather:
		if v == nil {
			return
		}
		f = *v
	case Feather:
		f = v
	default:
		return
	}
	if t.config.Feathers == nil {
		t.config.Feathers = make(map[string]Feather)
	}
	t.config.Feathers[method+" "+path] = f
}

// Shutdown signals every worker to stop accepting new connections and waits
// for the in-flight ioLoops to drain. Returns nil if the transport was never
// started, or ctx.Err() if the context expires before workers exit.
func (t *Transport) Shutdown(ctx context.Context) error {
	select {
	case <-t.ready:
	default:
		return nil // never started
	}
	t.shutdown.Store(true)
	for _, w := range t.workers {
		if w != nil {
			w.wake()
		}
	}
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *Transport) cleanupWorkers(upTo int) {
	for j := 0; j < upTo; j++ {
		if t.workers[j] != nil {
			t.workers[j].close()
		}
	}
}

// ----------------------------- worker -----------------------------

const maxEventsPerWait = 128

type parserLimits struct {
	maxHeaderCount int
	maxHeaderSize  int
}

type conn struct {
	fd           int32
	readBuf      []byte
	readN        int
	sendBuf      []byte
	sendN        int
	keepAlive    bool
	pending      int // in-flight handler goroutines
	remoteAddr   string
	lastActive   int64 // unix nano — updated on accept + each recv
	readDeadline int64 // unix nano — set when first byte arrives, cleared on full request
	ctx          context.Context
	cancel       context.CancelFunc
	sendFileFd   int32 // sendfile: source fd (0 = none)
	sendFileSize int64 // sendfile: remaining bytes
}

var resp503 = []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")

type doneMsg struct {
	fd        int32
	data      []byte
	keepAlive bool
}

type worker struct {
	id           int
	listenFd     int
	eng          engine
	handler      transport.Handler
	config       WingConfig
	limits       parserLimits
	maxConns     int
	conns        map[int32]*conn
	events       [maxEventsPerWait]event
	evfd         int // eventfd for wake signaling
	doneCh       chan doneMsg
	pool         *workerPool
	feathers     FeatherTable
	shutdown     *atomic.Bool
	hasTimeout   bool
	readTimeout  int64 // nanoseconds (0 = disabled)
	writeTimeout int64
	idleTimeout  int64
	sweepAt      int64 // next sweep unix nano
	// dispatchWG tracks Spawn and Takeover goroutines so cleanup() can wait
	// for in-flight RawSyscall(SYS_WRITE) / blocking syscall.Write calls to
	// finish before closing fds. Pool goroutines are tracked separately by
	// pool.wg. Without this barrier, a dispatch goroutine could write to an
	// fd the kernel has already recycled.
	dispatchWG sync.WaitGroup
}

type handlerJob struct {
	req          *wingRequest
	fd           int32
	keepAlive    bool
	responseMode responseMode
}

// workerPool is a fixed-size goroutine pool per worker.
type workerPool struct {
	jobs chan handlerJob
	done chan doneMsg
	wake func()
	// wg tracks the pool goroutines so cleanup() can wait for in-flight
	// RawSyscall(SYS_WRITE) calls to finish before closing fds. Without this
	// barrier, a pool goroutine could write to an fd that the kernel has
	// already recycled for a new connection, leaking response bytes to the
	// wrong client.
	wg sync.WaitGroup
}

func newWorkerPool(size int, h transport.Handler, done chan doneMsg, wake func()) *workerPool {
	p := &workerPool{
		jobs: make(chan handlerJob, size*2),
		done: done,
		wake: wake,
	}
	p.wg.Add(size)
	for i := 0; i < size; i++ {
		go p.loop(h)
	}
	return p
}

func (p *workerPool) loop(h transport.Handler) {
	defer p.wg.Done()
	for job := range p.jobs {
		resp := acquireResponse()
		resp.responseMode = job.responseMode
		func() {
			defer func() {
				if r := recover(); r != nil {
					resp.WriteHeader(500)
					resp.Write([]byte("Internal Server Error\n"))
				}
			}()
			h.ServeKruda(resp, job.req)
		}()
		data := resp.buildZeroCopy()
		releaseResponse(resp)
		releaseRequest(job.req)

		// Direct write from pool goroutine — skip doneCh data copy + SubmitSend round-trip.
		if len(data) == 0 {
			p.done <- doneMsg{fd: job.fd, keepAlive: job.keepAlive}
			p.wake()
			continue
		}
		var remaining []byte
		n, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(job.fd), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
		if e == 0 && int(n) == len(data) {
			// Full write succeeded — signal ioLoop to re-arm EPOLLIN only.
			p.done <- doneMsg{fd: job.fd, keepAlive: job.keepAlive}
		} else {
			// Partial or failed write — fall back to ioLoop for remainder.
			written := 0
			if e == 0 {
				written = int(n)
			}
			remaining = data[written:]
			p.done <- doneMsg{fd: job.fd, data: remaining, keepAlive: job.keepAlive}
		}
		p.wake()
	}
}

func newWorker(id, listenFd int, cfg WingConfig, handler transport.Handler) (*worker, error) {
	eng := newEngine()
	wakeR, wakeW, err := createWakeFds()
	if err != nil {
		eng.Close()
		return nil, err
	}
	if err := eng.Init(engineConfig{RingSize: cfg.RingSize, PipeW: wakeW, EventFd: wakeR, RawMode: !cfg.needsAsync()}); err != nil {
		closeFd(wakeR)
		if wakeW != wakeR {
			closeFd(wakeW)
		}
		return nil, err
	}
	// On darwin, kqueueEngine creates its own internal pipe — the external
	// wakeW fd is unused. Close it to avoid a per-worker fd leak.
	if wakeW != wakeR {
		closeFd(wakeW)
	}
	doneCh := make(chan doneMsg, 4096)
	ft := NewFeatherTable(cfg.Feathers, cfg.DefaultFeather)
	w := &worker{
		id:           id,
		listenFd:     listenFd,
		eng:          eng,
		handler:      handler,
		config:       cfg,
		limits:       parserLimits{maxHeaderCount: cfg.MaxHeaderCount, maxHeaderSize: cfg.MaxHeaderSize},
		maxConns:     cfg.MaxConnsPerWorker,
		conns:        make(map[int32]*conn, 1024),
		evfd:         wakeR,
		doneCh:       doneCh,
		feathers:     ft,
		readTimeout:  int64(cfg.ReadTimeout),
		writeTimeout: int64(cfg.WriteTimeout),
		idleTimeout:  int64(cfg.IdleTimeout),
	}
	if cfg.needsPool() {
		w.pool = newWorkerPool(cfg.HandlerPoolSize, handler, doneCh, eng.PostWake)
	}
	return w, nil
}

func (w *worker) run(shutdown *atomic.Bool) {
	w.shutdown = shutdown
	w.ioLoop(shutdown)
}

func (w *worker) ioLoop(shutdown *atomic.Bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w.eng.SubmitAccept(w.listenFd)
	w.eng.Flush()

	hasAsync := w.config.needsAsync()
	hasTimeout := w.readTimeout > 0 || w.writeTimeout > 0 || w.idleTimeout > 0
	w.hasTimeout = hasTimeout
	if hasTimeout {
		w.sweepAt = time.Now().UnixNano() + int64(time.Second)
	}

	for !shutdown.Load() {
		if hasAsync {
		drain:
			for {
				select {
				case msg := <-w.doneCh:
					w.handleDone(msg)
				default:
					break drain
				}
			}
		}

		n, err := w.eng.Wait(w.events[:])
		if err != nil {
			if shutdown.Load() {
				break
			}
			continue
		}
		for i := 0; i < n; i++ {
			w.handleEvent(w.events[i])
		}
		if hasTimeout {
			if now := time.Now().UnixNano(); now >= w.sweepAt {
				w.sweepTimeouts(now)
				w.sweepAt = now + int64(time.Second)
			}
		}
	}
	w.cleanup()
}

// sweepTimeouts closes connections that have exceeded their timeout.
// Called at most once per second — zero cost when no timeouts configured.
func (w *worker) sweepTimeouts(now int64) {
	for fd, c := range w.conns {
		if c.pending > 0 {
			continue // handler in flight — don't close
		}
		// Read timeout: partial request sitting too long.
		if w.readTimeout > 0 && c.readDeadline > 0 && now > c.readDeadline {
			w.closeConn(fd)
			continue
		}
		// Idle timeout: keep-alive conn with no activity.
		if w.idleTimeout > 0 && c.readN == 0 && now-c.lastActive > w.idleTimeout {
			w.closeConn(fd)
			continue
		}
		// Write timeout: sendBuf stuck (partial send not draining).
		if w.writeTimeout > 0 && len(c.sendBuf) > 0 && now-c.lastActive > w.writeTimeout {
			w.closeConn(fd)
		}
	}
}

func (w *worker) handleEvent(ev event) {
	switch ev.Op {
	case opAccept:
		w.handleAccept(ev)
	case opRecv:
		w.handleRecv(ev)
	case opSend:
		w.handleSend(ev)
	case opClose:
		delete(w.conns, ev.Fd)
	case opWake:
		// eventfd re-arms automatically (edge-triggered)
	}
}

func (w *worker) handleAccept(ev event) {
	if ev.Res < 0 {
		w.eng.SubmitAccept(w.listenFd)
		return
	}
	if ev.Flags&cqeFMore == 0 {
		w.eng.SubmitAccept(w.listenFd)
	}
	fd := ev.Res
	if w.maxConns > 0 && len(w.conns) >= w.maxConns {
		closeFd(int(fd))
		return
	}
	setTCPNodelay(fd)
	setTCPQuickACK(fd)
	ctx, cancel := context.WithCancel(context.Background())
	c := &conn{
		fd:         fd,
		readBuf:    make([]byte, w.config.ReadBufSize),
		remoteAddr: getPeerAddr(fd),
		ctx:        ctx,
		cancel:     cancel,
	}
	if w.hasTimeout {
		now := time.Now().UnixNano()
		c.lastActive = now
		if w.readTimeout > 0 {
			c.readDeadline = now + w.readTimeout
		}
	}
	w.conns[fd] = c
	w.eng.RegisterConn(fd, unsafe.Pointer(c))
	// Try direct read — data often arrives with SYN-ACK.
	r, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(&c.readBuf[0])), uintptr(len(c.readBuf)))
	if e == 0 && r > 0 {
		c.readN = int(r)
		w.tryParse(c)
		return
	}
	// Speculative read got EAGAIN — arm read for next data arrival.
	// On Linux this is redundant (ET EPOLLIN already registered by RegisterConn)
	// but on kqueue/darwin SubmitRecv is the only way to register EVFILT_READ.
	w.eng.SubmitRecv(c.fd, nil, 0)
}

func (w *worker) handleRecv(ev event) {
	var c *conn
	if ev.ConnPtr != nil {
		c = (*conn)(ev.ConnPtr)
	} else {
		c = w.conns[ev.Fd]
	}
	if c == nil || c.pending > 0 {
		return
	}
	nr, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(c.fd), uintptr(unsafe.Pointer(&c.readBuf[c.readN])), uintptr(len(c.readBuf)-c.readN))
	if e != 0 || nr <= 0 {
		w.closeConn(c.fd)
		return
	}
	c.readN += int(nr)
	if w.hasTimeout {
		now := time.Now().UnixNano()
		c.lastActive = now
		if c.readN == int(nr) && w.readTimeout > 0 {
			c.readDeadline = now + w.readTimeout
		}
	}
	w.tryParse(c)
}

func (w *worker) tryParse(c *conn) {
	for c.readN > 0 {
		req, consumed, ok := parseHTTPRequest(c.readBuf[:c.readN], w.limits)
		if !ok {
			if c.readN >= len(c.readBuf) {
				w.closeConn(c.fd)
				return
			}
			break
		}
		req.remoteAddr = c.remoteAddr
		req.fd = c.fd
		req.ctx = c.ctx
		// Full request received — clear read deadline, update idle clock.
		if w.hasTimeout {
			c.readDeadline = 0
			c.lastActive = time.Now().UnixNano()
		}

		f := w.feathers.Lookup(req.method, req.path)

		// For async dispatch modes, only one in-flight handler per conn.
		if f.Dispatch != Inline && c.pending > 0 {
			// Don't consume — leave data in readBuf for re-parse after handleDone.
			break
		}

		// Consume parsed bytes.
		remaining := c.readN - consumed
		if remaining > 0 {
			copy(c.readBuf, c.readBuf[consumed:c.readN])
		}
		c.readN = remaining

		switch f.Dispatch {
		case Inline:
			if f.StaticResponse != nil {
				c.keepAlive = req.keepAlive
				c.sendBuf = append(c.sendBuf, f.StaticResponse...)
				releaseRequest(req)
			} else {
				resp := acquireResponse()
				resp.responseMode = f.ResponseMode
				w.handler.ServeKruda(resp, req)
				if resp.fileFd > 0 {
					// Sendfile path: write headers, then sendfile for body.
					hdr := resp.buildZeroCopy()
					c.keepAlive = req.keepAlive
					c.sendBuf = append(c.sendBuf, hdr...)
					c.sendFileFd = resp.fileFd
					c.sendFileSize = resp.fileSize
					releaseResponse(resp)
					releaseRequest(req)
					break
				}
				if resp.plaintextFast {
					c.keepAlive = req.keepAlive
					c.sendBuf = resp.appendPlaintextTo(c.sendBuf)
					releaseResponse(resp)
					releaseRequest(req)
					break
				}
				data := resp.buildZeroCopy()
				c.keepAlive = req.keepAlive
				c.sendBuf = append(c.sendBuf, data...)
				releaseResponse(resp)
				releaseRequest(req)
			}

		case Pool:
			// Dispatch to goroutine pool.
			c.keepAlive = req.keepAlive
			c.pending++
			job := handlerJob{req: req, fd: c.fd, keepAlive: req.keepAlive, responseMode: f.ResponseMode}
			select {
			case w.pool.jobs <- job:
			default:
				// Pool saturated — run inline to avoid deadlock.
				resp := acquireResponse()
				resp.responseMode = f.ResponseMode
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
				releaseResponse(resp)
				releaseRequest(req)
				w.doneCh <- doneMsg{fd: c.fd, data: data, keepAlive: req.keepAlive}
			}
			// Send any buffered inline responses, then wait for pool completion.
			if len(c.sendBuf) > 0 {
				c.sendN = 0
				w.eng.SubmitSend(c.fd, nil)
			}
			return

		case Spawn:
			// New goroutine per request.
			c.keepAlive = req.keepAlive
			c.pending++
			w.dispatchWG.Add(1)
			go func(req *wingRequest, fd int32, ka bool) {
				defer w.dispatchWG.Done()
				resp := acquireResponse()
				resp.responseMode = f.ResponseMode
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
				releaseResponse(resp)
				releaseRequest(req)
				// Direct write from spawn goroutine.
				if len(data) == 0 {
					w.doneCh <- doneMsg{fd: fd, keepAlive: ka}
					w.wake()
					return
				}
				n, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
				if e == 0 && int(n) == len(data) {
					w.doneCh <- doneMsg{fd: fd, keepAlive: ka}
				} else {
					written := 0
					if e == 0 {
						written = int(n)
					}
					w.doneCh <- doneMsg{fd: fd, data: data[written:], keepAlive: ka}
				}
				w.wake()
			}(req, c.fd, req.keepAlive)
			return

		case Takeover:
			// Goroutine takes over the connection with blocking I/O.
			// Detach fd from epoll so the goroutine owns it exclusively.
			c.keepAlive = req.keepAlive
			c.pending++
			w.eng.Detach(c.fd)
			var leftover []byte
			if c.readN > 0 {
				leftover = make([]byte, c.readN)
				copy(leftover, c.readBuf[:c.readN])
				c.readN = 0
			}
			w.dispatchWG.Add(1)
			go func() {
				defer w.dispatchWG.Done()
				w.takeoverLoop(req, c.fd, leftover)
			}()
			return

		default:
			// Persist or unknown — treat as Pool for now.
			c.keepAlive = req.keepAlive
			c.pending++
			job := handlerJob{req: req, fd: c.fd, keepAlive: req.keepAlive, responseMode: f.ResponseMode}
			if w.pool != nil {
				select {
				case w.pool.jobs <- job:
				default:
					releaseRequest(req)
					w.doneCh <- doneMsg{fd: c.fd, data: resp503, keepAlive: false}
				}
			} else {
				resp := acquireResponse()
				resp.responseMode = f.ResponseMode
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
				releaseResponse(resp)
				releaseRequest(req)
				w.doneCh <- doneMsg{fd: c.fd, data: data, keepAlive: req.keepAlive}
			}
			return
		}

		if !c.keepAlive {
			break
		}
	}
	if len(c.sendBuf) > 0 {
		c.sendN = 0
		// Direct write — skip epoll EPOLLOUT round-trip for inline responses.
		w.directSend(c)
	} else {
		w.eng.SubmitRecv(c.fd, nil, 0)
	}
}

func (w *worker) handleDone(msg doneMsg) {
	c := w.conns[msg.fd] // async path — no ConnPtr available
	if c == nil {
		return
	}
	c.pending--
	c.keepAlive = msg.keepAlive
	if len(msg.data) == 0 {
		// Pool goroutine already wrote the response directly.
		if c.keepAlive {
			if c.readN > 0 {
				w.tryParse(c)
			} else {
				w.eng.SubmitRecv(c.fd, nil, 0)
			}
		} else {
			w.closeConn(c.fd)
		}
		return
	}
	// Partial write fallback — pool couldn't write everything.
	c.sendBuf = append(c.sendBuf, msg.data...)
	c.sendN = 0
	w.eng.SubmitSend(c.fd, nil)
}

// directSend attempts a non-blocking write. If partial, falls back to epoll EPOLLOUT.
func (w *worker) directSend(c *conn) {
	for c.sendN < len(c.sendBuf) {
		buf := c.sendBuf[c.sendN:]
		r, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(c.fd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if e != 0 {
			if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
				w.eng.SubmitSend(c.fd, nil)
				return
			}
			w.closeConn(c.fd)
			return
		}
		c.sendN += int(r)
	}
	c.sendBuf = c.sendBuf[:0]
	c.sendN = 0
	// Sendfile: transfer file body after headers are written.
	if c.sendFileFd > 0 {
		for c.sendFileSize > 0 {
			n, err := sendfile(c.fd, c.sendFileFd, nil, int(c.sendFileSize))
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					// Socket buffer full — wait for writable notification.
					w.eng.SubmitSend(c.fd, nil)
					return
				}
				syscall.Close(int(c.sendFileFd))
				c.sendFileFd = 0
				w.closeConn(c.fd)
				return
			}
			c.sendFileSize -= int64(n)
		}
		syscall.Close(int(c.sendFileFd))
		c.sendFileFd = 0
	}
	if !c.keepAlive {
		w.closeConn(c.fd)
		return
	}
	if c.readN > 0 {
		w.tryParse(c)
		return
	}
	r, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(c.fd), uintptr(unsafe.Pointer(&c.readBuf[0])), uintptr(len(c.readBuf)))
	if e == 0 && r > 0 {
		c.readN = int(r)
		w.tryParse(c)
		return
	}
	// Speculative read got EAGAIN — arm read for next data arrival.
	// On Linux this is redundant (ET EPOLLIN already active) but on
	// kqueue/darwin SubmitRecv is required to register EVFILT_READ.
	w.eng.SubmitRecv(c.fd, nil, 0)
}

// takeoverBufPool provides read buffers for Takeover goroutines.
var takeoverBufPool = sync.Pool{New: func() any { b := make([]byte, 8192); return &b }}

// takeoverLoop owns a connection fd: loops read→handle→write using
// blocking syscalls (not RawSyscall) so the Go runtime detects the
// blocking I/O and creates extra OS threads, avoiding starvation
// from ioLoop's LockOSThread.
func (w *worker) takeoverLoop(first *wingRequest, fd int32, leftover []byte) {
	// Set fd to blocking mode so syscall.Read/Write will block the OS thread,
	// triggering Go runtime to spin up new threads for other goroutines.
	syscall.SetNonblock(int(fd), false)

	bp := takeoverBufPool.Get().(*[]byte)
	buf := *bp
	readN := copy(buf, leftover)

	remoteAddr := first.remoteAddr
	connCtx := first.ctx // conn-level context — propagated to all pipelined requests.
	req := first
	keepAlive := req.keepAlive

	for {
		// Handle request.
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
		releaseResponse(resp)
		releaseRequest(req)

		// Write full response — blocking syscall.Write (not RawSyscall).
		for off := 0; off < len(data); {
			n, err := syscall.Write(int(fd), data[off:])
			if err != nil {
				keepAlive = false
				goto done
			}
			if n > 0 {
				off += n
			}
		}

		if !keepAlive {
			goto done
		}

		// Read next request — blocking syscall.Read.
		for {
			if readN > 0 {
				r, consumed, ok := parseHTTPRequest(buf[:readN], w.limits)
				if ok {
					remaining := readN - consumed
					if remaining > 0 {
						copy(buf, buf[consumed:readN])
					}
					readN = remaining
					r.remoteAddr = remoteAddr
					r.fd = fd
					r.ctx = connCtx
					req = r
					keepAlive = req.keepAlive
					goto next
				}
			}
			if readN >= len(buf) {
				keepAlive = false
				goto done
			}
			n, err := syscall.Read(int(fd), buf[readN:])
			if n > 0 {
				readN += n
				continue
			}
			if err != nil {
				keepAlive = false
			}
			goto done
		}
	next:
	}

done:
	takeoverBufPool.Put(bp)
	// Signal worker loop to close conn and fd via closeConn → SubmitClose.
	// Do NOT call syscall.Close here — double-close risks recycled fd corruption.
	w.doneCh <- doneMsg{fd: fd, keepAlive: false}
	w.wake()
}

func (w *worker) handleSend(ev event) {
	var c *conn
	if ev.ConnPtr != nil {
		c = (*conn)(ev.ConnPtr)
	} else {
		c = w.conns[ev.Fd]
	}
	if c == nil || (len(c.sendBuf) == 0 && c.sendFileFd == 0) {
		// No pending data or sendfile — remove EPOLLOUT, listen for EPOLLIN.
		if c != nil {
			w.eng.SubmitRecv(c.fd, nil, 0)
		}
		return
	}
	w.directSend(c)
}

func (w *worker) closeConn(fd int32) {
	if c, ok := w.conns[fd]; ok {
		if c.cancel != nil {
			c.cancel()
		}
		if c.sendFileFd > 0 {
			syscall.Close(int(c.sendFileFd))
			c.sendFileFd = 0
		}
	}
	delete(w.conns, fd)
	w.eng.SubmitClose(fd)
}

func (w *worker) wake() { w.eng.PostWake() }

func (w *worker) cleanup() {
	if w.pool != nil {
		close(w.pool.jobs)
		// Wait for pool goroutines to finish any in-flight job's RawSyscall
		// before we start closing fds. Otherwise a pool goroutine's write
		// could land on an fd the kernel has already recycled.
		w.pool.wg.Wait()
	}
	// Takeover goroutines block on syscall.Read until the client sends data
	// or the connection closes. To unblock them WITHOUT yet closing the fd
	// (which would race with Spawn writes), half-close the read side. The
	// pending Read returns EOF, takeoverLoop exits, dispatchWG.Done fires.
	// SHUT_RD doesn't free the fd number, so kernel won't recycle it — Spawn's
	// write side stays valid until we run closeFd below.
	for _, c := range w.conns {
		if c.pending > 0 {
			_ = syscall.Shutdown(int(c.fd), syscall.SHUT_RD)
		}
	}
	// Drain doneCh concurrently with dispatchWG.Wait. Without this, a wave
	// of Spawn/Takeover goroutines completing simultaneously can fill the
	// 4096-slot doneCh buffer; the next sender blocks on the channel send
	// before reaching its `defer dispatchWG.Done()`, and Wait deadlocks.
	// Bounded by typical workloads but real for high-concurrency shutdowns.
	drainStop := make(chan struct{})
	go func() {
		for {
			select {
			case <-w.doneCh:
				// discard — ioLoop has stopped, nothing left to process
			case <-drainStop:
				return
			}
		}
	}()
	// Now safe to wait for Spawn + Takeover dispatch goroutines.
	w.dispatchWG.Wait()
	close(drainStop)
	for fd, c := range w.conns {
		if c.cancel != nil {
			c.cancel()
		}
		if c.sendFileFd > 0 {
			syscall.Close(int(c.sendFileFd))
		}
		closeFd(int(fd))
	}
	w.eng.Close()
	closeFd(w.evfd)
	closeFd(w.listenFd)
}

func (w *worker) close() { w.cleanup() }
