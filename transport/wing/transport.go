package wing

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

type Config struct {
	Workers           int
	RingSize          uint32
	ReadBufSize       int
	MaxHeaderCount    int
	MaxHeaderSize     int
	MaxConnsPerWorker int
	HandlerPoolSize   int                // goroutine pool size per worker (Pool dispatch routes)
	Feathers          map[string]Feather // per-route feather config ("METHOD /path" → Feather)
	DefaultFeather    Feather            // fallback feather for routes not in Feathers
	Bone              Bone               // engine-level optimizations (affects all connections)
	Prefork           bool               // fork N processes with GOMAXPROCS(1) each
	ReadTimeout       time.Duration      // max time to receive a complete request (0 = disabled)
	WriteTimeout      time.Duration      // max time to send a response (0 = disabled)
	IdleTimeout       time.Duration      // max time a keep-alive conn can be idle (0 = disabled)
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
	if c.HandlerPoolSize <= 0 {
		c.HandlerPoolSize = c.Workers
	}
}

// needsPool returns true if any route uses Pool dispatch (requires pre-allocated goroutine pool).
// Spawn dispatch uses ad-hoc goroutines and does NOT require a pool.
func (c *Config) needsPool() bool {
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
func (c *Config) needsAsync() bool {
	d := c.DefaultFeather.Dispatch
	if d == 0 {
		d = Inline
	}
	if d == Pool || d == Spawn {
		return true
	}
	for _, f := range c.Feathers {
		fd := f.Dispatch
		if fd == 0 {
			fd = Inline
		}
		if fd == Pool || fd == Spawn {
			return true
		}
	}
	return false
}

type Transport struct {
	config   Config
	workers  []*worker
	shutdown atomic.Bool
	wg       sync.WaitGroup
	ready    chan struct{}
}

func New(cfg Config) *Transport {
	cfg.defaults()
	return &Transport{config: cfg, ready: make(chan struct{})}
}

func (t *Transport) ListenAndServe(addr string, handler transport.Handler) error {
	if t.config.Prefork {
		return t.listenAndServePrefork(addr, handler)
	}
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

func (t *Transport) Serve(ln net.Listener, handler transport.Handler) error {
	addr := ln.Addr().String()
	_ = ln.Close()
	return t.ListenAndServe(addr, handler)
}

// SetRouteFeather implements transport.FeatherConfigurator.
func (t *Transport) SetRouteFeather(method, path string, feather any) {
	if f, ok := feather.(Feather); ok {
		if t.config.Feathers == nil {
			t.config.Feathers = make(map[string]Feather)
		}
		t.config.Feathers[method+" "+path] = f
	}
}

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
	config       Config
	limits       parserLimits
	maxConns     int
	conns        map[int32]*conn
	events       [maxEventsPerWait]event
	pipeR        int
	pipeW        int
	pipeBuf      [8]byte
	doneCh       chan doneMsg
	pool         *workerPool
	feathers     FeatherTable
	shutdown     *atomic.Bool
	hasTimeout   bool
	readTimeout  int64 // nanoseconds (0 = disabled)
	writeTimeout int64
	idleTimeout  int64
	sweepAt      int64 // next sweep unix nano
}

type handlerJob struct {
	req       *wingRequest
	fd        int32
	keepAlive bool
}

// workerPool is a fixed-size goroutine pool per worker.
type workerPool struct {
	jobs chan handlerJob
	done chan doneMsg
	wake func()
}

func newWorkerPool(size int, h transport.Handler, done chan doneMsg, wake func()) *workerPool {
	p := &workerPool{
		jobs: make(chan handlerJob, size*2),
		done: done,
		wake: wake,
	}
	for i := 0; i < size; i++ {
		go p.loop(h)
	}
	return p
}

func (p *workerPool) loop(h transport.Handler) {
	for job := range p.jobs {
		resp := acquireResponse()
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

func newWorker(id, listenFd int, cfg Config, handler transport.Handler) (*worker, error) {
	eng := newEngine()
	pipeR, pipeW, err := createPipe()
	if err != nil {
		eng.Close()
		return nil, err
	}
	if err := eng.Init(engineConfig{RingSize: cfg.RingSize, PipeW: pipeW, RawMode: !cfg.needsAsync()}); err != nil {
		closeFd(pipeR)
		closeFd(pipeW)
		return nil, err
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
		pipeR:        pipeR,
		pipeW:        pipeW,
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
	canLock := !w.config.needsPool()
	if canLock {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	w.eng.SubmitAccept(w.listenFd)
	w.eng.SubmitPipeRecv(w.pipeR, w.pipeBuf[:])
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
		w.eng.SubmitPipeRecv(w.pipeR, w.pipeBuf[:])
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
	// ET EPOLLIN already registered by RegisterConn — no SubmitRecv needed.
}

func (w *worker) handleRecv(ev event) {
	var c *conn
	if ev.ConnPtr != nil {
		c = (*conn)(ev.ConnPtr)
	} else {
		c = w.conns[ev.Fd]
	}
	if c == nil {
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
				w.handler.ServeKruda(resp, req)
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
			job := handlerJob{req: req, fd: c.fd, keepAlive: req.keepAlive}
			select {
			case w.pool.jobs <- job:
			default:
				// Pool saturated — run inline to avoid deadlock.
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
			go func(req *wingRequest, fd int32, ka bool) {
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
				// Direct write from spawn goroutine.
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

		default:
			// Persist or unknown — treat as Pool for now.
			c.keepAlive = req.keepAlive
			c.pending++
			job := handlerJob{req: req, fd: c.fd, keepAlive: req.keepAlive}
			if w.pool != nil {
				select {
				case w.pool.jobs <- job:
				default:
					w.doneCh <- doneMsg{fd: c.fd, data: resp503, keepAlive: false}
				}
			} else {
				resp := acquireResponse()
				w.handler.ServeKruda(resp, req)
				data := resp.buildZeroCopy()
				releaseResponse(resp)
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
	// Speculative read got EAGAIN — wait for epoll EPOLLIN.
	// No SubmitRecv needed: ET EPOLLIN fires on new data arrival.
}
func (w *worker) send503(c *conn) {
	c.sendBuf = resp503
	c.sendN = 0
	c.keepAlive = false
	w.eng.SubmitSend(c.fd, nil)
}

func (w *worker) handleSend(ev event) {
	var c *conn
	if ev.ConnPtr != nil {
		c = (*conn)(ev.ConnPtr)
	} else {
		c = w.conns[ev.Fd]
	}
	if c == nil || len(c.sendBuf) == 0 {
		// Remove EPOLLOUT — only listen for EPOLLIN.
		if c != nil {
			w.eng.SubmitRecv(c.fd, nil, 0)
		}
		return
	}
	w.directSend(c)
}

func (w *worker) closeConn(fd int32) {
	if c, ok := w.conns[fd]; ok && c.cancel != nil {
		c.cancel()
	}
	delete(w.conns, fd)
	w.eng.SubmitClose(fd)
}

func (w *worker) wake() { w.eng.PostWake() }

func (w *worker) cleanup() {
	for fd := range w.conns {
		closeFd(int(fd))
	}
	w.eng.Close()
	closeFd(w.pipeR)
	closeFd(w.pipeW)
	closeFd(w.listenFd)
}

func (w *worker) close() { w.cleanup() }
