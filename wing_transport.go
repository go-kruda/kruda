//go:build linux || darwin

package kruda

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
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
	d := c.DefaultPreset.Dispatch
	if d == 0 {
		d = Inline
	}
	if d == Pool {
		return true
	}
	for _, f := range c.Presets {
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
	d := c.DefaultPreset.Dispatch
	if d == 0 {
		d = Inline
	}
	if d == Pool || d == Spawn || d == Takeover {
		return true
	}
	for _, f := range c.Presets {
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
// transport.PresetConfigurator.
type Transport struct {
	config  WingConfig
	workers []*worker
	// shutdown/connCnt/rejectTtl are shared cold-path atomics touched only on
	// accept and connection close (never per-request). Co-located so they share
	// cache lines away from the hot per-conn state; whether connCnt warrants its
	// own cache line vs. shutdown (false-sharing under an accept storm) is left
	// to a Linux A/B before adding padding.
	shutdown   atomic.Bool
	connCnt    int64 // accepted connections currently admitted (atomic)
	rejectTtl  int64 // connections refused by the total cap (atomic)
	rejectIP   int64 // connections refused by the per-IP cap (atomic)
	rejectRate int64 // connections refused by the accept-rate bucket (atomic)
	wg         sync.WaitGroup
	ready      chan struct{}
}

// connCount returns the number of currently-admitted connections across all
// workers. Test/diagnostics accessor; reads the shared atomic.
func (t *Transport) connCount() int64 { return atomic.LoadInt64(&t.connCnt) }

// RejectStats returns accept-side rejection counters for all three limit kinds
// (total cap, per-IP cap, accept-rate bucket) since startup.
func (t *Transport) RejectStats() RejectStats {
	return RejectStats{
		Total: atomic.LoadInt64(&t.rejectTtl),
		PerIP: atomic.LoadInt64(&t.rejectIP),
		Rate:  atomic.LoadInt64(&t.rejectRate),
	}
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
		w, err := newWorker(i, fd, t.config, handler, &t.connCnt, &t.rejectTtl, &t.rejectIP, &t.rejectRate)
		if err != nil {
			closeFd(fd)
			t.cleanupWorkers(i)
			return fmt.Errorf("wing: worker %d: %w", i, err)
		}
		t.workers[i] = w
	}
	close(t.ready)
	// Startup banner: log the resolved connection cap once, at actual serve
	// time (not at construction — see newWingTransport). Serve() routes through
	// here too, so this fires exactly once per server start.
	if t.config.MaxConns > 0 {
		lg := t.config.Logger
		if lg == nil {
			lg = slog.Default()
		}
		if t.config.MaxConns < acceptCapLowFloor {
			lg.Warn("kruda/wing: derived connection cap is low; raise the fd ulimit or set WithMaxConns", "cap", t.config.MaxConns)
		}
		lg.Info("kruda/wing: connection cap", "max", t.config.MaxConns)
	}
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

// SetRoutePreset implements transport.PresetConfigurator. The preset
// argument is typed as `any` at the interface boundary; route registration
// passes a `*Preset` (so a missing hint is nil); we accept either form for
// backward compatibility with anything still passing `Preset` by value.
func (t *Transport) SetRoutePreset(method, path string, preset any) {
	var f Preset
	switch v := preset.(type) {
	case *Preset:
		if v == nil {
			return
		}
		f = *v
	case Preset:
		f = v
	default:
		return
	}
	if t.config.Presets == nil {
		t.config.Presets = make(map[string]Preset)
	}
	f.path = path
	f.pathClean = !containsDotPercent(path)
	t.config.Presets[method+" "+path] = f
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
	bodyLimit      int
}

type conn struct {
	fd           int32
	peerIP       netip.Addr // peer IP captured at accept (zero value if unknown)
	readBuf      []byte
	readN        int
	sendBuf      []byte
	sendN        int
	keepAlive    bool
	admitted     bool   // true once accepted; cleared once by removeConnBookkeeping (idempotency guard)
	pending      int    // in-flight handler goroutines
	takenOver    bool   // fd detached to a Takeover goroutine and owned by its *os.File
	remoteAddr   string // lazy peer address cache, filled only if Request.RemoteAddr is used
	lastActive   int64  // unix nano — updated on accept + each recv
	readDeadline int64  // unix nano — set when first byte arrives, cleared on full request
	ctx          context.Context
	cancel       context.CancelFunc
	sendFileFd   int32 // sendfile: source fd (0 = none)
	sendFileSize int64 // sendfile: remaining bytes
	// Slow-path body accumulation (populated when a request body spans multiple recvs).
	bodyNeed       int    // total Content-Length expected (0 = not accumulating)
	headerSnapshot []byte // copy of header block for re-parse after body complete
	bodyBuf        []byte // incrementally grown body buffer
}

// testAcceptPeerHook, when non-nil, receives the accept-time peer IP for each
// accepted connection. Tests set it to pin assertions to the accept path rather
// than the lazy getpeername used by RemoteAddr/IP. Always nil in production.
var testAcceptPeerHook func(ip netip.Addr, ok bool)

var resp503 = []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")

type doneMsg struct {
	fd        int32
	data      []byte
	keepAlive bool
	// file is non-nil when a Takeover goroutine owned the fd through an
	// *os.File on the runtime netpoller. The worker must finish conn
	// bookkeeping first and then close through file (not SubmitClose), so
	// the kernel cannot recycle the fd number while the conns map still
	// references it.
	file *os.File
}

type worker struct {
	id       int
	listenFd int
	eng      engine
	handler  transport.Handler
	config   WingConfig
	limits   parserLimits
	maxConns int
	logger   *slog.Logger
	// connCount/rejectTotal/rejectIP/rejectRate point at the shared Transport
	// atomics so caps are enforced server-wide (not per-worker). Set in newWorker.
	connCount    *int64
	rejectTotal  *int64
	rejectIP     *int64  // per-IP reject counter (always wired; connsPerIP map is nil when the cap is off)
	rejectRate   *int64  // accept-rate reject counter (always wired; bucket is nil when rate limiting is off)
	rejectWarned [3]bool // warn-once flags indexed by rejectKind; per-worker so no lock needed
	// Per-worker per-IP connection tracking. Allocated only when maxConnsPerIP>0.
	// Per-worker means no locks: this map is touched only on accept and close,
	// both on the same event-loop goroutine. Under SO_REUSEPORT the same source
	// IP may spread across workers, so the cap is approximate across workers.
	connsPerIP    map[netip.Addr]int
	maxConnsPerIP int
	conns         map[int32]*conn
	events        [maxEventsPerWait]event
	evfd          int // eventfd for wake signaling
	doneCh        chan doneMsg
	pool          *workerPool
	presets       PresetTable
	// Exact-route MRU cache. Paths stored here must come from Preset.path,
	// never directly from the read buffer's unsafe request path.
	lastPreset0     Preset
	lastPreset1     Preset
	lastMethod0     string
	lastMethod1     string
	lastPath0       string
	lastPath1       string
	shutdown        *atomic.Bool
	hasTimeout      bool
	bucket          *tokenBucket // per-worker accept-rate limiter; nil when AcceptRatePerSec==0
	readTimeout     int64        // nanoseconds (0 = disabled)
	writeTimeout    int64
	idleTimeout     int64
	maxInflightBody int   // per-worker budget (0 = unlimited)
	inflightBody    int64 // current in-flight body bytes (atomic)
	trustProxy      bool
	sweepAt         int64 // next sweep unix nano
	now             int64 // unix nano cached once per event batch
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

type wingRouteHandler interface {
	serveKrudaRoute(transport.ResponseWriter, transport.Request, []HandlerFunc)
}

type wingSingleHandler interface {
	serveKrudaSingleHandler(transport.ResponseWriter, transport.Request, HandlerFunc) bool
}

type wingFastSingleHandler interface {
	serveWingSingleHandler(*wingResponse, *wingRequest, HandlerFunc) bool
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

func (w *worker) serveRoute(resp *wingResponse, req *wingRequest, f Preset) {
	if len(f.handlers) > 0 && (f.pathClean || (f.path == "" && !containsDotPercent(req.path))) {
		if len(f.handlers) == 1 {
			if h, ok := w.handler.(wingFastSingleHandler); ok && h.serveWingSingleHandler(resp, req, f.handlers[0]) {
				return
			}
			if h, ok := w.handler.(wingSingleHandler); ok && h.serveKrudaSingleHandler(resp, req, f.handlers[0]) {
				return
			}
		}
		if h, ok := w.handler.(wingRouteHandler); ok {
			h.serveKrudaRoute(resp, req, f.handlers)
			return
		}
	}
	w.handler.ServeKruda(resp, req)
}

func (w *worker) lookupPreset(method, path string) Preset {
	if w.lastPath0 != "" && method == w.lastMethod0 && path == w.lastPath0 {
		return w.lastPreset0
	}
	if w.lastPath1 != "" && method == w.lastMethod1 && path == w.lastPath1 {
		f := w.lastPreset1
		m := w.lastMethod1
		p := w.lastPath1
		w.lastPreset1 = w.lastPreset0
		w.lastMethod1 = w.lastMethod0
		w.lastPath1 = w.lastPath0
		w.lastPreset0 = f
		w.lastMethod0 = m
		w.lastPath0 = p
		return f
	}
	f := w.presets.Lookup(method, path)
	if f.path != "" {
		w.lastPreset1 = w.lastPreset0
		w.lastMethod1 = w.lastMethod0
		w.lastPath1 = w.lastPath0
		w.lastPreset0 = f
		w.lastMethod0 = method
		w.lastPath0 = f.path
	}
	return f
}

func newWorker(id, listenFd int, cfg WingConfig, handler transport.Handler, connCount, rejectTotal, rejectIP, rejectRate *int64) (*worker, error) {
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
	ft := NewPresetTable(cfg.Presets, cfg.DefaultPreset)
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	w := &worker{
		id:              id,
		listenFd:        listenFd,
		eng:             eng,
		handler:         handler,
		config:          cfg,
		limits:          parserLimits{maxHeaderCount: cfg.MaxHeaderCount, maxHeaderSize: cfg.MaxHeaderSize, bodyLimit: cfg.BodyLimit},
		maxConns:        cfg.MaxConns, // global total cap; enforced against shared connCount via CAS
		logger:          lg,
		connCount:       connCount,
		rejectTotal:     rejectTotal,
		rejectIP:        rejectIP,
		rejectRate:      rejectRate,
		maxConnsPerIP:   cfg.MaxConnsPerIP,
		conns:           make(map[int32]*conn, 1024),
		evfd:            wakeR,
		doneCh:          doneCh,
		presets:         ft,
		readTimeout:     int64(cfg.ReadTimeout),
		writeTimeout:    int64(cfg.WriteTimeout),
		idleTimeout:     int64(cfg.IdleTimeout),
		maxInflightBody: cfg.MaxInflightBodyBytes,
		trustProxy:      cfg.TrustProxy,
	}
	if cfg.MaxConnsPerIP > 0 {
		w.connsPerIP = make(map[netip.Addr]int, 64)
	}
	if cfg.AcceptRatePerSec > 0 {
		// Divide the rate and burst across workers so the server-wide rate stays
		// at the configured value under SO_REUSEPORT load distribution.
		// min(1) prevents the per-worker rate from rounding to zero.
		workers := cfg.Workers
		if workers < 1 {
			workers = 1
		}
		perWorkerRate := cfg.AcceptRatePerSec / workers
		if perWorkerRate < 1 {
			perWorkerRate = 1
		}
		perWorkerBurst := cfg.AcceptRateBurst / workers
		if perWorkerBurst < 1 {
			perWorkerBurst = 1
		}
		w.bucket = newTokenBucket(perWorkerRate, perWorkerBurst)
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

// clockNow returns the cached w.now if it is non-zero (set by the ioLoop when
// hasClock is true), otherwise falls back to a live time.Now(). The fallback
// path is defensive — it should only trigger for the first event in a batch
// when neither timeouts nor rate-limiting are configured.
func (w *worker) clockNow() int64 {
	if w.now != 0 {
		return w.now
	}
	return time.Now().UnixNano()
}

func (w *worker) ioLoop(shutdown *atomic.Bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w.eng.SubmitAccept(w.listenFd)
	w.eng.Flush()

	hasAsync := w.config.needsAsync()
	hasTimeout := w.readTimeout > 0 || w.writeTimeout > 0 || w.idleTimeout > 0
	w.hasTimeout = hasTimeout
	// hasClock is true when any subsystem needs a fresh w.now per event batch.
	// The rate bucket needs a real clock even when no connection timeouts are set.
	// Zero cost when both are off: w.now stays 0 and clockNow() falls back lazily.
	hasClock := hasTimeout || w.bucket != nil
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
		if hasClock {
			w.now = time.Now().UnixNano()
		}
		for i := 0; i < n; i++ {
			w.handleEvent(w.events[i])
		}
		if hasTimeout {
			if w.now >= w.sweepAt {
				w.sweepTimeouts(w.now)
				w.sweepAt = w.now + int64(time.Second)
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
		// opClose: vestigial on Linux and darwin — both engines' SubmitClose is
		// synchronous (EpollCtl(DEL)/syscall.Close inline) and never enqueues an
		// opClose event. Routed through the chokepoint anyway for safety/kqueue
		// parity should a future async-close engine emit it.
		w.removeConnBookkeeping(ev.Fd)
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
	// Admission order at accept: (1) accept-rate — cheapest, no map lookup;
	// (2) per-IP precheck (read-only, no mutation);
	// (3) global total reserve via CAS against the shared Transport.connCnt.
	if w.bucket != nil && !w.bucket.allow(w.clockNow()) {
		atomic.AddInt64(w.rejectRate, 1)
		rejectWarnOnce(w, rejectKindRate, 0)
		closeFd(int(fd)) // RST, not 503 — no request parsed at accept time
		return
	}
	// Keyed on ev.PeerIP (kernel accept-time socket peer), never RemoteAddr()/XFF — the latter is
	// attacker-spoofable under TrustProxy, so trusting it here would let one client evade the per-IP cap.
	if w.maxConnsPerIP > 0 && ev.HasPeer && w.connsPerIP[ev.PeerIP] >= w.maxConnsPerIP {
		atomic.AddInt64(w.rejectIP, 1)
		rejectWarnOnce(w, rejectKindIP, w.maxConnsPerIP)
		closeFd(int(fd)) // RST, not 503 — no request parsed at accept time
		return
	}
	if max := w.maxConns; max > 0 {
		for {
			cur := atomic.LoadInt64(w.connCount)
			if cur >= int64(max) {
				atomic.AddInt64(w.rejectTotal, 1)
				rejectWarnOnce(w, rejectKindTotal, max)
				closeFd(int(fd)) // RST, not 503 — no request parsed at accept time
				return
			}
			if atomic.CompareAndSwapInt64(w.connCount, cur, cur+1) {
				break
			}
		}
	}
	setTCPNodelay(fd)
	setTCPQuickACK(fd)
	if testAcceptPeerHook != nil {
		testAcceptPeerHook(ev.PeerIP, ev.HasPeer)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &conn{
		fd:       fd,
		peerIP:   ev.PeerIP,
		admitted: true, // tracked for bookkeeping; per-counter gates inside removeConnBookkeeping decide what to decrement
		readBuf:  make([]byte, w.config.ReadBufSize),
		ctx:      ctx,
		cancel:   cancel,
	}
	if w.hasTimeout {
		now := w.now
		if now == 0 {
			now = time.Now().UnixNano()
		}
		c.lastActive = now
		if w.readTimeout > 0 {
			c.readDeadline = now + w.readTimeout
		}
	}
	w.conns[fd] = c
	if w.maxConnsPerIP > 0 && ev.HasPeer {
		w.connsPerIP[c.peerIP]++
	}
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
	submitIdleRecv(w.eng, c.fd, nil, 0)
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
		now := w.now
		if now == 0 {
			now = time.Now().UnixNano()
		}
		c.lastActive = now
		// Set the read deadline once, on the first byte of a request, and do not
		// refresh it while accumulating a body — otherwise a slow client trickling
		// body bytes resets the deadline forever (slowloris). The deadline then
		// bounds the whole request read, matching net/http's ReadTimeout.
		if c.readN == int(nr) && c.bodyNeed == 0 && w.readTimeout > 0 {
			c.readDeadline = now + w.readTimeout
		}
	}
	// If accumulating a multi-recv body, feed new bytes into the accumulator.
	if c.bodyNeed > 0 {
		var surplus []byte
		c.bodyBuf, surplus = accumBody(c.bodyBuf, c.readBuf[:c.readN], c.bodyNeed)
		if len(surplus) > 0 {
			// Body completed with trailing pipelined bytes — relocate them to the
			// front of readBuf so the post-dispatch parse sees the next request.
			c.readN = copy(c.readBuf, surplus)
		} else {
			c.readN = 0
		}
		w.drainBodyAccum(c)
		return
	}
	w.tryParse(c)
}

func (w *worker) tryParse(c *conn) {
	for c.readN > 0 {
		req, consumed, ok := parseHTTPRequestFast(c.readBuf[:c.readN], w.limits)
		if !ok {
			if c.readN < len(c.readBuf) {
				// Buffer has room — classify to decide whether to wait or error.
				st, need, expectContinue := classifyIncomplete(c.readBuf[:c.readN], w.limits)
				switch st {
				case parseNeedHeaderMore:
					// Not enough data yet; wait for more recvs.
				case parseChunked:
					w.writeAndClose(c, wingStatusClose(501))
					return
				case parseBodyTooLarge:
					w.writeAndClose(c, wingStatusClose(413))
					return
				case parseNeedBody:
					if expectContinue {
						c.sendBuf = append(c.sendBuf, wing100Continue...)
						// Flush it immediately so the client sends the body.
						if len(c.sendBuf) > 0 {
							c.sendN = 0
							w.directSend(c) // non-blocking; rest of buf stays queued if partial
						}
					}
					if !w.beginBodyAccum(c, need) {
						w.writeAndClose(c, wingStatusClose(503))
						return
					}
					return
				case parseHeaderTooLarge:
					w.writeAndClose(c, wingStatusClose(431))
					return
				default: // parseMalformed
					w.closeConn(c.fd)
					return
				}
			} else {
				// Buffer full — classify; if body needed start accum, else error.
				st, need, expectContinue := classifyIncomplete(c.readBuf[:c.readN], w.limits)
				switch st {
				case parseBodyTooLarge:
					w.writeAndClose(c, wingStatusClose(413))
					return
				case parseNeedBody:
					if expectContinue {
						c.sendBuf = append(c.sendBuf, wing100Continue...)
						// Flush it immediately so the client sends the body.
						if len(c.sendBuf) > 0 {
							c.sendN = 0
							w.directSend(c) // non-blocking; rest of buf stays queued if partial
						}
					}
					if !w.beginBodyAccum(c, need) {
						w.writeAndClose(c, wingStatusClose(503))
						return
					}
					return
				default:
					// Headers exceed buffer: 431 Request Header Fields Too Large.
					w.writeAndClose(c, wingStatusClose(431))
					return
				}
			}
			break
		}
		req.fd = c.fd
		req.ctx = c.ctx
		req.remoteAddrRef = &c.remoteAddr
		req.trustProxy = w.trustProxy
		// Full request received — clear read deadline, update idle clock.
		if w.hasTimeout {
			c.readDeadline = 0
			c.lastActive = time.Now().UnixNano()
		}

		f := w.lookupPreset(req.method, req.path)
		finalizeRequestPath(req, f)

		// For async dispatch modes, only one in-flight handler per conn.
		if f.Dispatch != Inline && c.pending > 0 {
			// Don't consume — leave data in readBuf for re-parse after handleDone.
			break
		}

		// Consume parsed bytes.
		remaining := c.readN - consumed
		if remaining > 0 {
			finalizeRequestCommonHeaders(req)
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
				start := time.Now().UnixNano()
				w.serveRoute(resp, req, f)
				if elapsed := time.Now().UnixNano() - start; elapsed >= advisorBlockNanos {
					advisorObserve(req.method, req.path, elapsed, f.explicit)
				}
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
				if resp.stringFast {
					c.keepAlive = req.keepAlive
					c.sendBuf = resp.appendStringTo(c.sendBuf)
					releaseResponse(resp)
					releaseRequest(req)
					break
				}
				if resp.jsonFast {
					c.keepAlive = req.keepAlive
					c.sendBuf = resp.appendJSONTo(c.sendBuf)
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
					w.serveRoute(resp, req, f)
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
			go func(req *wingRequest, fd int32, ka bool, f Preset) {
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
					w.serveRoute(resp, req, f)
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
			}(req, c.fd, req.keepAlive, f)
			return

		case Takeover:
			// Goroutine takes over the connection with blocking I/O.
			// Detach fd from epoll so the goroutine owns it exclusively.
			c.keepAlive = req.keepAlive
			c.pending++
			c.takenOver = true
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
				w.takeoverLoop(req, c.fd, leftover, f.ResponseMode)
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
					w.serveRoute(resp, req, f)
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
		submitIdleRecv(w.eng, c.fd, nil, 0)
	}
}

// writeAndClose queues an error response and triggers a direct send, then closes.
func (w *worker) writeAndClose(c *conn, resp []byte) {
	c.keepAlive = false
	c.sendBuf = append(c.sendBuf, resp...)
	if len(c.sendBuf) > 0 {
		c.sendN = 0
		w.directSend(c)
	}
}

// accumBody appends as much of src as the body still needs into bodyBuf (capped
// at need) and returns the grown buffer plus any surplus — the start of the next
// pipelined request that arrived in the same read. The surplus aliases src's
// backing array, so callers must relocate it (to the front of c.readBuf) before
// reusing that array.
func accumBody(bodyBuf, src []byte, need int) (out, surplus []byte) {
	take := need - len(bodyBuf)
	if take > len(src) {
		take = len(src)
	}
	out = append(bodyBuf, src[:take]...)
	return out, src[take:]
}

// beginBodyAccum starts body accumulation for a connection.
// Returns false if the per-worker in-flight budget would be exceeded.
func (w *worker) beginBodyAccum(c *conn, need int) bool {
	if w.maxInflightBody > 0 {
		newTotal := atomic.AddInt64(&w.inflightBody, int64(need))
		if newTotal > int64(w.maxInflightBody) {
			atomic.AddInt64(&w.inflightBody, -int64(need))
			return false
		}
	}
	headerEnd := bytes.Index(c.readBuf[:c.readN], crlfcrlf)
	if headerEnd < 0 {
		// Should not happen: classifyIncomplete confirmed headers are complete.
		if w.maxInflightBody > 0 {
			atomic.AddInt64(&w.inflightBody, -int64(need))
		}
		return false
	}
	headerEnd += 4 // past \r\n\r\n
	c.headerSnapshot = append([]byte{}, c.readBuf[:headerEnd]...)
	c.bodyNeed = need
	initCap := need
	if initCap > 8192 {
		initCap = 8192
	}
	c.bodyBuf = make([]byte, 0, initCap)
	// Copy any body bytes already present in the read buffer.
	var surplus []byte
	if have := c.readN - headerEnd; have > 0 {
		c.bodyBuf, surplus = accumBody(c.bodyBuf, c.readBuf[headerEnd:c.readN], need)
	}
	if len(surplus) > 0 {
		// The initial buffer already holds the full body plus the start of the
		// next pipelined request — preserve that surplus for the post-dispatch parse.
		c.readN = copy(c.readBuf, surplus)
	} else {
		c.readN = 0
	}
	w.drainBodyAccum(c)
	return true
}

// drainBodyAccum reads body bytes from the socket into c.bodyBuf until the body
// is complete or the socket would block, then dispatches or arms the next recv.
// Draining to EAGAIN is required for edge-triggered epoll on Linux: it does not
// re-notify for bytes already buffered in the socket, so reading a single chunk
// per event would stall a body that spans more than one read. Safe on kqueue too
// (reads stop at EAGAIN). Returns true if the connection was closed.
func (w *worker) drainBodyAccum(c *conn) bool {
	for len(c.bodyBuf) < c.bodyNeed {
		rn, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(c.fd), uintptr(unsafe.Pointer(&c.readBuf[0])), uintptr(len(c.readBuf)))
		if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
			break
		}
		if e != 0 || rn <= 0 {
			w.closeConn(c.fd)
			return true
		}
		var surplus []byte
		c.bodyBuf, surplus = accumBody(c.bodyBuf, c.readBuf[:int(rn)], c.bodyNeed)
		if len(surplus) > 0 {
			// The read overshot the body — the body is now complete, so this loop
			// exits. Relocate the trailing pipelined bytes to the front of readBuf
			// for the post-dispatch parse.
			c.readN = copy(c.readBuf, surplus)
		}
	}
	if len(c.bodyBuf) >= c.bodyNeed {
		// Capture any pipelined request that already arrived behind the body
		// before dispatching: edge-triggered epoll will not re-notify for bytes
		// buffered in the socket before the body completed, so a request split
		// across the completion boundary would otherwise stall.
		w.drainPipelinedAfterBody(c)
		w.finishBodyAccum(c)
	} else {
		// Socket drained but body still incomplete — wait for the next recv.
		submitIdleRecv(w.eng, c.fd, nil, 0)
	}
	return false
}

// drainPipelinedAfterBody appends socket bytes that follow a just-completed body
// (the start of the next pipelined request) into readBuf after any surplus
// already relocated to its front. Non-blocking: it stops at EAGAIN or a full
// buffer, so on the common case where no pipelined request is waiting it costs a
// single EAGAIN read. A pipelined burst exceeding the read buffer is bounded by
// the same buffer-size limit as ordinary pipelining.
func (w *worker) drainPipelinedAfterBody(c *conn) {
	for c.readN < len(c.readBuf) {
		rn, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(c.fd), uintptr(unsafe.Pointer(&c.readBuf[c.readN])), uintptr(len(c.readBuf)-c.readN))
		if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
			return
		}
		if e != 0 || rn <= 0 {
			// Error or EOF: the accumulated request is still dispatched; teardown
			// happens on the post-response keep-alive read.
			return
		}
		c.readN += int(rn)
	}
}

// finishBodyAccum re-parses the completed request and dispatches it inline.
func (w *worker) finishBodyAccum(c *conn) {
	if w.maxInflightBody > 0 {
		atomic.AddInt64(&w.inflightBody, -int64(c.bodyNeed))
	}
	// Reconstruct: header block + body bytes → full request bytes.
	full := append(c.headerSnapshot, c.bodyBuf...)
	c.bodyNeed = 0
	c.headerSnapshot = nil
	c.bodyBuf = nil
	req, _, ok := parseHTTPRequest(full, w.limits)
	if !ok {
		w.closeConn(c.fd)
		return
	}
	req.fd = c.fd
	req.ctx = c.ctx
	req.remoteAddrRef = &c.remoteAddr
	req.trustProxy = w.trustProxy
	if w.hasTimeout {
		c.readDeadline = 0
		c.lastActive = time.Now().UnixNano()
	}
	f := w.lookupPreset(req.method, req.path)
	finalizeRequestPath(req, f)
	w.dispatchAccumulated(c, req, f)
}

// dispatchAccumulated dispatches a body-accumulated request.
// Accumulated bodies are always dispatched inline; non-inline presets
// with large bodies are uncommon and correctness takes precedence here.
func (w *worker) dispatchAccumulated(c *conn, req *wingRequest, f Preset) {
	// A Hijack-preset route (WebSocket upgrade) is a bodyless GET. A body forces
	// inline accumulation, where c.ResponseWriter() is not an http.Hijacker — so
	// reject it cleanly (400) rather than letting the upgrade fail opaquely.
	if f.ResponseMode == responseHijack {
		c.keepAlive = false
		c.sendBuf = append(c.sendBuf, wingStatusClose(400)...)
		releaseRequest(req)
		c.sendN = 0
		w.directSend(c)
		return
	}
	finalizeRequestCommonHeaders(req)
	resp := acquireResponse()
	resp.responseMode = f.ResponseMode
	start := time.Now().UnixNano()
	w.serveRoute(resp, req, f)
	if elapsed := time.Now().UnixNano() - start; elapsed >= advisorBlockNanos {
		advisorObserve(req.method, req.path, elapsed, f.explicit)
	}
	data := resp.buildZeroCopy()
	c.keepAlive = req.keepAlive
	c.sendBuf = append(c.sendBuf, data...)
	releaseResponse(resp)
	releaseRequest(req)
	if len(c.sendBuf) > 0 {
		c.sendN = 0
		w.directSend(c)
	} else {
		submitIdleRecv(w.eng, c.fd, nil, 0)
	}
}

func (w *worker) handleDone(msg doneMsg) {
	if msg.file != nil {
		// Takeover conn finished: clean bookkeeping before closing through
		// the File so the fd number cannot be recycled mid-cleanup. The fd
		// was Detached from the engine at takeover, so SubmitClose must not
		// run here — the *os.File owns the close.
		w.removeConnBookkeeping(msg.fd)
		delete(w.conns, msg.fd)
		msg.file.Close()
		return
	}
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
				submitIdleRecv(w.eng, c.fd, nil, 0)
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
	submitIdleRecv(w.eng, c.fd, nil, 0)
}

// takeoverSpinReads bounds the non-blocking read attempts a Takeover keep-alive
// loop makes before parking on the runtime netpoller. A small spin can catch an
// already-buffered next request without a netpoller wake hop; the fallback park
// keeps the takeover thread count low when the connection is genuinely idle.
const takeoverSpinReads = 8

// takeoverBufPool provides read buffers for Takeover goroutines.
var takeoverBufPool = sync.Pool{New: func() any { b := make([]byte, 8192); return &b }}

// takeoverLoop owns a connection fd: loops read→handle→write through an
// *os.File so a blocked read parks the goroutine on the runtime netpoller
// instead of pinning an OS thread. The previous blocking-syscall variant
// ran one OS thread per connection (237-250 threads, 1.40 context switches
// per request at c256 on /db) — identical CPU per request to this model,
// but scheduler pressure inflated the time a pgx pool conn stayed checked
// out, costing ~6% throughput (results/forensics-20260612T150500Z).
func (w *worker) takeoverLoop(first *wingRequest, fd int32, leftover []byte, mode responseMode) {
	// fds arrive non-blocking from accept4/SetNonblock. os.NewFile registers
	// a non-blocking fd with the runtime poller, so Read/Write below park
	// the goroutine, never the thread.
	f := os.NewFile(uintptr(fd), "wing-takeover")
	if f == nil {
		w.doneCh <- doneMsg{fd: fd, keepAlive: false}
		w.wake()
		return
	}

	// Streaming responses (SSE / chunked) consume the whole connection: the
	// handler holds it open and writes incrementally through a flushing writer,
	// so there is no keep-alive reuse and no next-request parse. Handle the
	// first request as a stream and return — all other dispatch paths fall
	// through to the byte-unchanged keep-alive loop below.
	if mode == responseStream {
		w.streamTakeover(first, fd, f)
		return
	}

	if mode == responseHijack {
		w.hijackTakeover(first, fd, f, leftover)
		return
	}

	bp := takeoverBufPool.Get().(*[]byte)
	buf := *bp
	// The pool's default buffer is 8 KB; grow it to the configured ReadBufSize so
	// the takeover path accepts the same header sizes as the event loop (which
	// sizes readBuf from ReadBufSize/HeaderLimit) and so copy() below never
	// truncates leftover.
	grewBuf := w.config.ReadBufSize > len(buf)
	if grewBuf {
		buf = make([]byte, w.config.ReadBufSize)
	}
	readN := copy(buf, leftover)

	var bodyBuf []byte
	defer func() {
		if !grewBuf {
			*bp = buf
		}
		// grewBuf: *bp still holds the original pool-sized buffer; the
		// grown buf is dropped here and reclaimed by the GC, preventing
		// oversized-buffer retention in the pool.
		takeoverBufPool.Put(bp)
		bodyBuf = nil
	}()

	remoteAddrRef := first.remoteAddrRef
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

		// Write full response — os.File loops over partial writes and parks
		// on EAGAIN via the runtime poller.
		if _, werr := f.Write(data); werr != nil {
			keepAlive = false
			goto done
		}

		if !keepAlive {
			goto done
		}

		// Read next request — parks the goroutine until data arrives.
		for {
			if readN > 0 {
				r, consumed, ok := parseHTTPRequest(buf[:readN], w.limits)
				if ok {
					remaining := readN - consumed
					if remaining > 0 {
						copy(buf, buf[consumed:readN])
					}
					readN = remaining
					r.fd = fd
					r.ctx = connCtx
					r.remoteAddrRef = remoteAddrRef
					r.trustProxy = w.trustProxy
					req = r
					keepAlive = req.keepAlive
					goto next
				}
				// Classify the incomplete/rejected request.
				st, need, expectContinue := classifyIncomplete(buf[:readN], w.limits)
				switch st {
				case parseNeedHeaderMore:
					// Need more header bytes — keep reading.
					if readN >= len(buf) {
						// Buffer full but headers still incomplete: 431.
						f.Write(wingStatusClose(431))
						keepAlive = false
						goto done
					}
				case parseChunked:
					f.Write(wingStatusClose(501))
					keepAlive = false
					goto done
				case parseBodyTooLarge:
					f.Write(wingStatusClose(413))
					keepAlive = false
					goto done
				case parseNeedBody:
					if expectContinue {
						f.Write(wing100Continue)
					}
					// Locate end of header block.
					headerEnd := bytes.Index(buf[:readN], crlfcrlf)
					if headerEnd < 0 {
						keepAlive = false
						goto done
					}
					headerEnd += 4
					// Charge the shared per-worker in-flight body budget so a flood
					// of concurrent takeover uploads cannot allocate unbounded memory
					// (mirrors the event-loop beginBodyAccum). Refunded on every exit
					// below — completion, read error — since takeover never reaches
					// closeConn while accumulating.
					if w.maxInflightBody > 0 {
						if atomic.AddInt64(&w.inflightBody, int64(need)) > int64(w.maxInflightBody) {
							atomic.AddInt64(&w.inflightBody, -int64(need))
							f.Write(wingStatusClose(503))
							keepAlive = false
							goto done
						}
					}
					headerSnapshot := make([]byte, headerEnd)
					copy(headerSnapshot, buf[:headerEnd])
					// Seed bodyBuf with already-received body bytes.
					bodyBuf = bodyBuf[:0]
					if have := readN - headerEnd; have > 0 {
						bodyBuf = append(bodyBuf, buf[headerEnd:readN]...)
					}
					readN = 0
					// Read remaining body bytes, reusing buf as a temp chunk buffer.
					// One absolute deadline bounds the whole body read so a slow
					// client cannot extend it indefinitely by trickling bytes.
					if w.readTimeout > 0 {
						f.SetReadDeadline(time.Now().Add(time.Duration(w.readTimeout)))
					}
					for len(bodyBuf) < need {
						n, rerr := f.Read(buf)
						if n > 0 {
							bodyBuf = append(bodyBuf, buf[:n]...)
						}
						if rerr != nil {
							if w.readTimeout > 0 {
								f.SetReadDeadline(time.Time{})
							}
							if w.maxInflightBody > 0 {
								atomic.AddInt64(&w.inflightBody, -int64(need))
							}
							keepAlive = false
							goto done
						}
					}
					if w.readTimeout > 0 {
						f.SetReadDeadline(time.Time{})
					}
					// Body fully accumulated — release the budget reservation
					// (mirrors finishBodyAccum). Any later exit on this iteration is
					// post-accumulation, so it must not refund again.
					if w.maxInflightBody > 0 {
						atomic.AddInt64(&w.inflightBody, -int64(need))
					}
					// Reconstruct and re-parse with full body present.
					full := append(headerSnapshot, bodyBuf[:need]...)
					// Carry bytes beyond the body (start of the next pipelined
					// request) back into buf so the next iteration parses them
					// instead of dropping them.
					if surplus := len(bodyBuf) - need; surplus > 0 {
						readN = copy(buf, bodyBuf[need:])
					}
					bodyBuf = bodyBuf[:0]
					r2, _, ok2 := parseHTTPRequest(full, w.limits)
					if !ok2 {
						keepAlive = false
						goto done
					}
					r2.fd = fd
					r2.ctx = connCtx
					r2.remoteAddrRef = remoteAddrRef
					r2.trustProxy = w.trustProxy
					req = r2
					keepAlive = req.keepAlive
					goto next
				case parseHeaderTooLarge:
					f.Write(wingStatusClose(431))
					keepAlive = false
					goto done
				default:
					// parseMalformed — silent close.
					keepAlive = false
					goto done
				}
			}
			// Adaptive spin: a few non-blocking raw reads before parking on the
			// netpoller. Catches an already-buffered next keep-alive request
			// without a netpoller wake hop; falls back to f.Read (park) under
			// idle so the takeover thread-count win is preserved. The fd is owned
			// solely by this goroutine, so a raw read cannot race the *os.File.
			got := false
			for s := 0; s < takeoverSpinReads; s++ {
				sn, serr := syscall.Read(int(fd), buf[readN:])
				if sn > 0 {
					readN += sn
					got = true
					break
				}
				if serr == syscall.EAGAIN {
					continue
				}
				// EOF (sn==0, serr==nil) or hard error: stop keep-alive.
				keepAlive = false
				goto done
			}
			if got {
				continue
			}
			n, err := f.Read(buf[readN:])
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
	// Hand the File to the worker loop: it deletes conn bookkeeping first
	// and then closes through the File. Do NOT close here — the kernel
	// could recycle the fd number while the conns map still references it.
	w.doneCh <- doneMsg{fd: fd, keepAlive: false, file: f}
	w.wake()
}

// streamTakeover runs a single streaming response (Stream preset) over the
// takeover fd f. Unlike the keep-alive takeover loop, it gives the handler a
// flushing wingStreamWriter so c.SSE()/c.Stream() write incrementally, installs
// a cancellable context so SSEStream.Done() fires on disconnect, and runs a
// read-watcher that cancels that context when the client closes its half. The
// streaming response is not keep-alive-reused; on return the fd is closed
// exactly once via the shared takeover-done path (doneMsg.file).
func (w *worker) streamTakeover(first *wingRequest, fd int32, f *os.File) {
	// Make the handler's c.Context() cancellable from the conn context so the
	// disconnect watcher can fire SSEStream.Done(). first.ctx flows into c.ctx
	// during ServeKruda (app_serve.go: c.ctx = r.Context()).
	ctx, cancel := context.WithCancel(first.ctx)
	first.ctx = ctx

	sw := newWingStreamWriter(f, time.Duration(w.writeTimeout))

	go watchStreamDisconnect(f, cancel)

	func() {
		defer func() {
			if r := recover(); r != nil {
				// The preamble may already be on the wire; a 500 status line is
				// only meaningful if nothing has been written yet. Either way,
				// stop the stream and let the deferred cancel + close run.
				if !sw.headersSent {
					sw.WriteHeader(500)
					_, _ = sw.Write([]byte("Internal Server Error\n"))
				}
			}
		}()
		w.handler.ServeKruda(sw, first)
	}()

	cancel() // stop the watcher; it parks on f.Read until the fd closes below

	// Return the request to the pool, matching the other dispatch paths. Safe
	// here: the handler has returned, the watcher is cancelled, and the close
	// below uses only fd + the *os.File — nothing references first afterward.
	releaseRequest(first)

	// Close the fd exactly once via the shared takeover-done path. The conn is
	// already takenOver, so handleDone removes bookkeeping and closes through
	// the File — do NOT add a second close here.
	w.doneCh <- doneMsg{fd: fd, keepAlive: false, file: f}
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

// removeConnBookkeeping is the single chokepoint for releasing a connection's
// accounting: it decrements the global connCount and/or per-IP count
// (idempotently, guarded by c.admitted — mirrors the body-budget
// zero-before-delete idiom), releases the in-flight body reservation, cancels
// the conn context, and closes any sendfile source fd. It does NOT close the
// connection fd or delete the conns entry — each caller owns its own fd-close
// policy (closeConn → engine SubmitClose; takeover-done → *os.File; opClose →
// engine already closed).
//
// The two decrements are independently gated: global connCount only when the
// global cap is active (maxConns > 0, matching the CAS reserve); per-IP only
// when per-IP is active. This lets WithMaxConns(0)+WithMaxConnsPerIP(n) work
// correctly — admitted is always true so per-IP reclamation always fires.
func (w *worker) removeConnBookkeeping(fd int32) {
	c, ok := w.conns[fd]
	if !ok {
		return
	}
	if c.admitted {
		if w.maxConns > 0 {
			atomic.AddInt64(w.connCount, -1)
		}
		if w.maxConnsPerIP > 0 && c.peerIP.IsValid() {
			if n := w.connsPerIP[c.peerIP] - 1; n <= 0 {
				delete(w.connsPerIP, c.peerIP) // delete-at-0 keeps map bounded
			} else {
				w.connsPerIP[c.peerIP] = n
			}
		}
		c.admitted = false
	}
	// Release any in-flight body reservation so a connection closed mid-
	// accumulation (timeout, disconnect, parse failure) does not leak the
	// per-worker budget. finishBodyAccum zeroes bodyNeed before any close,
	// so this cannot double-decrement.
	if c.bodyNeed > 0 && w.maxInflightBody > 0 {
		atomic.AddInt64(&w.inflightBody, -int64(c.bodyNeed))
	}
	c.bodyNeed = 0
	c.bodyBuf = nil
	c.headerSnapshot = nil
	if c.cancel != nil {
		c.cancel()
	}
	if c.sendFileFd > 0 {
		syscall.Close(int(c.sendFileFd))
		c.sendFileFd = 0
	}
}

func (w *worker) closeConn(fd int32) {
	w.removeConnBookkeeping(fd)
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
	// Takeover goroutines can block in a syscall.Read, a syscall.Write (a
	// WebSocket WriteMessage), or in app logic with no pending socket call at
	// all. To unblock all three WITHOUT yet closing the fd (which would race
	// with Spawn writes), shut down both directions and cancel the conn
	// context. SHUT_RDWR unblocks a pending Read (EOF) or Write (EPIPE/ECONNRESET);
	// cancelling ctx wakes a handler blocked in app logic via Conn.Done().
	// Either way takeoverLoop exits and dispatchWG.Done fires. SHUT_* doesn't
	// free the fd number, so kernel won't recycle it — Spawn's write side
	// stays valid until we run closeFd below.
	for _, c := range w.conns {
		if c.pending > 0 {
			_ = syscall.Shutdown(int(c.fd), syscall.SHUT_RDWR)
			if c.cancel != nil {
				c.cancel()
			}
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
			case msg := <-w.doneCh:
				// A Takeover completion carries the fd's *os.File. Close it here
				// to release the fd now; discarding it would leak the File to a
				// finalizer that later closes a recycled fd. Non-Takeover
				// completions carry no file and are simply dropped (ioLoop has
				// stopped, nothing left to process).
				if msg.file != nil {
					msg.file.Close()
				}
			case <-drainStop:
				return
			}
		}
	}()
	// Now safe to wait for Spawn + Takeover dispatch goroutines.
	w.dispatchWG.Wait()
	close(drainStop)
	// Close any Takeover Files still buffered in doneCh that the concurrent
	// drain did not consume before it stopped — same recycled-fd hazard.
drainBuffered:
	for {
		select {
		case msg := <-w.doneCh:
			if msg.file != nil {
				msg.file.Close()
			}
		default:
			break drainBuffered
		}
	}
	for fd, c := range w.conns {
		// Decrement connCnt and per-IP counts for every connection still open at
		// shutdown. removeConnBookkeeping is idempotent (c.admitted guard) so
		// connections already closed normally before shutdown are a no-op. It also
		// cancels ctx and releases any sendFileFd reservation, so the manual calls
		// below are replaced by the chokepoint. This satisfies the design §7
		// invariant: "Shutdown … bookkeeping decremented via the chokepoint."
		w.removeConnBookkeeping(fd)
		if c.takenOver {
			// fd is owned by the Takeover *os.File and was already closed via the
			// drain above; raw-closing it here would double-close a possibly-
			// recycled fd.
			continue
		}
		closeFd(int(fd))
	}
	w.eng.Close()
	closeFd(w.evfd)
	closeFd(w.listenFd)
}

func (w *worker) close() { w.cleanup() }
