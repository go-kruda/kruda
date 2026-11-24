//go:build linux || darwin

package kruda

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	stopOnce   sync.Once // the first Shutdown wakes the workers; later ones only wait
	connCnt    int64     // accepted connections currently admitted (atomic)
	rejectTtl  int64     // connections refused by the total cap (atomic)
	rejectIP   int64     // connections refused by the per-IP cap (atomic)
	rejectRate int64     // connections refused by the accept-rate bucket (atomic)
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
	// Add before close(t.ready): a Shutdown that sees ready must not Wait on a zero counter.
	t.wg.Add(len(t.workers))
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
	if _, taken := t.config.Presets[method+" "+path]; taken && f.inherit {
		return
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
	t.stopOnce.Do(func() {
		t.shutdown.Store(true)
		// A worker closes its wake fd when it stops, so a second wake could
		// write to whatever reused that number.
		for _, w := range t.workers {
			if w != nil {
				w.externalWake()
			}
		}
	})
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
	fd              int32
	peerIP          netip.Addr // peer IP captured at accept (zero value if unknown)
	readBuf         []byte
	readN           int
	sendBuf         []byte
	sendN           int
	keepAlive       bool
	admitted        bool     // true once accepted; cleared once by removeConnBookkeeping (idempotency guard)
	pending         int      // in-flight handler goroutines
	takenOver       bool     // fd detached to a Takeover goroutine and owned by its *os.File
	takeoverFile    *os.File // the Takeover goroutine's; Shutdown only sets its read deadline
	takeoverGate    *takeoverGate
	closePending    bool   // close after async ownership returns; prevents fd reuse
	remoteAddr      string // lazy peer address cache, filled only if Request.RemoteAddr is used
	lastActive      int64  // unix nano — updated on accept + each recv
	lastSent        int64  // unix nano — last write that sent bytes; idle time counts from here after a response
	readDeadline    int64  // unix nano — set when first byte arrives, cleared on full request
	readSuppressed  bool   // readiness was not acted on; stays until a read finds EAGAIN or EOF
	readDrained     bool   // socket input exhausted during synchronous inline dispatch
	parseDepth      uint8  // parses nested through completed sends
	resumeParse     bool   // yielded with input to parse; the next write event resumes
	discarded       int    // input dropped behind the final response
	wrShut          bool   // the write side is shut
	responses       int    // responses begun, provisional ones too; past one, an earlier one may still be in the socket
	lingerUntil     int64  // monoNow nanos — the write side is shut; close by then at the latest
	ctx             context.Context
	cancel          context.CancelFunc
	sendFileFd      int32 // sendfile: source fd (0 = none)
	sendFileRelease func()
	sendFileSize    int64 // sendfile: remaining bytes
	fileStream      *fileStream
	// Slow-path body accumulation (populated when a request body spans multiple recvs).
	bodyNeed       int    // total Content-Length expected (0 = not accumulating)
	headerSnapshot []byte // copy of header block for re-parse after body complete
	bodyBuf        []byte // incrementally grown body buffer
}

// testAcceptPeerHook, when non-nil, receives the accept-time peer IP for each
// accepted connection. Tests set it to pin assertions to the accept path rather
// than the lazy getpeername used by RemoteAddr/IP. Always nil in production.
var testAcceptPeerHook func(ip netip.Addr, ok bool)

// testSuppressedRecvHook lets Linux integration tests prove that an EPOLLIN
// event reached the worker while an async handler still owned the connection.
var testSuppressedRecvHook func(fd int32)

// testTakeoverParseHook runs in takeoverLoop after its shutdown check, before
// it parses buffered input. Always nil in production.
var testTakeoverParseHook func()

// testDrainingHook runs in lingerTakeover once the drain owns the read
// deadline. Always nil in production.
var testDrainingHook func()

// testFinalDrainHook runs in lingerTakeover after its first look for unread
// input. Always nil in production.
var testFinalDrainHook func()

// testStreamTeardownHook runs once a Stream's disconnect watcher has stopped,
// before the final drain. Always nil in production.
var testStreamTeardownHook func()

var resp503 = []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")

type doneMsg struct {
	readStream *fileStream
	readErr    error
	stream     *fileStream
	fd         int32
	data       []byte
	keepAlive  bool
	// file is non-nil when a Takeover goroutine owned the fd through an
	// *os.File on the runtime netpoller. The worker must finish conn
	// bookkeeping first and then close through file (not SubmitClose), so
	// the kernel cannot recycle the fd number while the conns map still
	// references it.
	file *os.File
}

type worker struct {
	fileReadOnce    sync.Once
	fileReadMu      sync.Mutex
	fileReadStopped bool
	fileReadStop    chan struct{}
	id              int
	listenFd        int
	eng             engine
	handler         transport.Handler
	config          WingConfig
	limits          parserLimits
	maxConns        int
	logger          *slog.Logger
	// poolFullWarned is touched only by this worker's ioLoop.
	poolFullWarned bool
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
	lastPreset0 Preset
	lastPreset1 Preset
	lastMethod0 string
	lastMethod1 string
	lastPath0   string
	lastPath1   string
	// Blocking-advisor tallies for this worker's routes. Owned by the event-loop
	// goroutine and flushed into shared state only occasionally, so observing
	// every inline request stays free of atomics and allocation.
	advisor         advisorCache
	shutdown        *atomic.Bool
	hasTimeout      bool
	lingering       int // connections in closeAfterFinal's linger; they need the sweep
	lingerQ         []lingerEntry
	lingerHead      int        // lingerQ[:lingerHead] is consumed
	baseWaitMillis  int        // the engine wait when nothing lingers
	waitShortened   bool       // the engine wait is lingerWaitMillis until nothing lingers
	wakeMu          sync.Mutex // orders Shutdown's wake against the wake fd closing
	wakeClosed      bool
	discardBuf      []byte       // shared by discardInput, so a small ReadBufSize does not mean many reads
	bucket          *tokenBucket // per-worker accept-rate limiter; nil when AcceptRatePerSec==0
	readTimeout     int64        // nanoseconds (0 = disabled)
	writeTimeout    int64
	idleTimeout     int64
	maxInflightBody int   // per-worker budget (0 = unlimited)
	inflightBody    int64 // current in-flight body bytes (atomic)
	trustProxy      bool
	proxyPolicy     transport.ProxyPolicy
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
	directWrite  bool
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
		resp := acquireResponseFor(job.req)
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
		keepAlive := job.keepAlive && !resp.closeRequested()
		data := resp.buildZeroCopy()
		stream := resp.fileStream
		resp.fileStream = nil
		releaseResponse(resp)
		releaseRequest(job.req)

		if !job.directWrite || stream != nil {
			p.done <- doneMsg{fd: job.fd, data: data, keepAlive: keepAlive, stream: stream}
			p.wake()
			continue
		}
		// Direct write from pool goroutine — skip doneCh data copy + SubmitSend round-trip.
		if len(data) == 0 {
			p.done <- doneMsg{fd: job.fd, keepAlive: keepAlive}
			p.wake()
			continue
		}
		var remaining []byte
		n, e := rawWrite(job.fd, data)
		if e == 0 && n == len(data) {
			// Full write succeeded — signal ioLoop to re-arm EPOLLIN only.
			p.done <- doneMsg{fd: job.fd, keepAlive: keepAlive}
		} else {
			// Partial or failed write — fall back to ioLoop for remainder.
			written := 0
			if e == 0 {
				written = int(n)
			}
			remaining = data[written:]
			p.done <- doneMsg{fd: job.fd, data: remaining, keepAlive: keepAlive}
		}
		p.wake()
	}
}

func (w *worker) serveRoute(resp *wingResponse, req *wingRequest, f Preset) {
	if f.StaticResponse != nil {
		// The inline branch sends these itself; a request whose body arrives
		// after the headers reaches here, and the route may have no handler.
		resp.setStatic(f.StaticResponse, f.staticClose)
		return
	}
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

// observeAdvisor records one inline request against the blocking advisor.
//
// Keying on the request path rather than Preset.path matters: routes registered
// without an explicit preset get the table default, whose path is empty, so
// keying on that would silently exclude most routes. path aliases the read
// buffer, which advisorCache compares but never retains.
func (w *worker) observeAdvisor(f Preset, method, path string, elapsedNanos int64) {
	w.advisor.observe(method, path, elapsedNanos, f.explicit)
}

func newWorker(id, listenFd int, cfg WingConfig, handler transport.Handler, connCount, rejectTotal, rejectIP, rejectRate *int64) (*worker, error) {
	eng := newEngine()
	wakeR, wakeW, err := createWakeFds()
	if err != nil {
		eng.Close()
		return nil, err
	}
	waitMillis := 0
	if cfg.ReadTimeout > 0 || cfg.WriteTimeout > 0 || cfg.IdleTimeout > 0 {
		waitMillis = 1000
	}
	if err := eng.Init(engineConfig{WaitTimeoutMillis: waitMillis, RingSize: cfg.RingSize, PipeW: wakeW, EventFd: wakeR, RawMode: !cfg.needsAsync()}); err != nil {
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
	if app, ok := handler.(*App); ok {
		ft.cleanPaths = app.config.PathTraversal
	}
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	w := &worker{
		fileReadStop:    make(chan struct{}),
		id:              id,
		listenFd:        listenFd,
		eng:             eng,
		baseWaitMillis:  waitMillis,
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
		proxyPolicy:     transport.NewProxyPolicy(cfg.TrustedProxies),
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
// stopping reports whether Shutdown has begun. A Takeover goroutine checks it
// before it parses input, which the connection's context learns about later.
func (w *worker) stopping() bool { return w.shutdown != nil && w.shutdown.Load() }

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
		w.drainCompletions()

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
		w.tickLinger()
		if hasTimeout {
			if w.now >= w.sweepAt {
				w.sweepTimeouts(w.now)
				w.sweepAt = w.now + int64(time.Second)
			}
		}
	}
	w.cleanup()
}

// monoBase anchors monoNow, a monotonic clock: a wall-clock step back must not
// stretch a linger.
var monoBase = time.Now()

func monoNow() int64 { return int64(time.Since(monoBase)) }

// lingerWaitMillis caps the engine wait while a connection lingers, so the
// sweep runs within that of its deadline.
const lingerWaitMillis = 500

// setWait changes how long the engine waits for events, when it can.
func (w *worker) setWait(ms int) {
	if e, ok := w.eng.(interface{ setWaitTimeout(ms int) }); ok {
		e.setWaitTimeout(ms)
	}
}

// lingerEntry is a lingering connection and the deadline it was queued with.
type lingerEntry struct {
	c     *conn
	until int64
}

// sweepLinger closes lingering connections whose deadline has passed. The
// queue is in deadline order, as every linger lasts lingerDelay, so it stops
// at the first one still due; entries for connections that closed sooner are
// skipped.
func (w *worker) sweepLinger(now int64) {
	q := w.lingerQ
	for w.lingerHead < len(q) {
		e := q[w.lingerHead]
		if e.c.lingerUntil == e.until && w.conns[e.c.fd] == e.c {
			if now < e.until {
				break
			}
			w.closeConn(e.c.fd)
		}
		q[w.lingerHead] = lingerEntry{} // the queue must not keep a closed connection
		w.lingerHead++
	}
	if w.lingerHead == len(q) {
		w.lingerQ, w.lingerHead = q[:0], 0
	} else if w.lingerHead >= 64 && w.lingerHead > len(q)/2 {
		n := copy(q, q[w.lingerHead:]) // amortized: each entry moves at most once per halving
		clear(q[n:])
		w.lingerQ, w.lingerHead = q[:n], 0
	}
}

// drainCompletions handles the completions handlers have sent. A steady
// stream of them must not hold a linger open past its deadline.
func (w *worker) drainCompletions() {
	for {
		select {
		case msg := <-w.doneCh:
			w.handleDone(msg)
			if w.lingering > 0 {
				w.sweepLinger(monoNow())
			}
		default:
			return
		}
	}
}

// tickLinger runs after each event batch. Once no connection lingers, what is
// left in the queue refers to connections that closed sooner: it is released,
// and the engine waits as long as it did before.
func (w *worker) tickLinger() {
	if w.lingering > 0 {
		w.sweepLinger(monoNow())
	}
	if w.lingering == 0 && w.waitShortened {
		w.dropLingerQueue()
		w.setWait(w.baseWaitMillis)
		w.waitShortened = false
	}
}

func (w *worker) dropLingerQueue() {
	clear(w.lingerQ)
	w.lingerQ, w.lingerHead = w.lingerQ[:0], 0
}

// sweepTimeouts closes connections that have exceeded their timeout.
// Called at most once per second — zero cost when no timeouts configured.
func (w *worker) sweepTimeouts(now int64) {
	for _, c := range w.conns {
		if c.lingerUntil > 0 {
			continue // sweepLinger's
		}
		if c.pending > 0 {
			continue // handler in flight — don't close
		}
		sending := c.sending()
		// Read timeout: partial request sitting too long. Its clock restarts while
		// a reply is being sent, because nothing is read then, body bytes excepted.
		if w.readTimeout > 0 && c.readDeadline > 0 {
			if sending && c.bodyNeed == 0 {
				c.readDeadline = now + w.readTimeout
			} else if now > c.readDeadline {
				w.retire(c)
				continue
			}
		}
		// Idle timeout covers a stalled upload when no read deadline is set.
		// Idle time counts from the last byte received or sent. A connection still
		// sending is bounded by WriteTimeout instead, as in net/http: the kernel
		// send buffer can hold seconds of data for a slow reader, so the time
		// since the last write does not show whether the client is reading.
		readingBody := c.bodyNeed > 0 && w.readTimeout > 0 && c.readDeadline > 0
		if w.idleTimeout > 0 && c.readN == 0 && !readingBody && !sending && now-max(c.lastActive, c.lastSent) > w.idleTimeout {
			w.retire(c)
			continue
		}
		// Write timeout: sendBuf stuck (partial send not draining).
		if w.writeTimeout > 0 && sending && now-c.lastActive > w.writeTimeout {
			w.retire(c)
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
	r, e := rawRead(fd, c.readBuf)
	if e == 0 && r > 0 {
		c.readN = r
		c.readDrained = r < len(c.readBuf)
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
	if c == nil {
		return
	}
	if c.lingerUntil > 0 {
		w.closeAfterFinal(c)
		return
	}
	if c.pending > 0 || c.fileStream != nil || c.sendFileFd > 0 {
		c.suppressRead()
		return
	}
	queued := c.sendN < len(c.sendBuf)
	if queued && c.sendingFinal() {
		// An EOF or a failed socket is left to the send. So is input past the
		// budget: it stays unread, and the close after the send handles it.
		w.discardInput(c)
		return
	}
	// Input waits in the socket behind unsent output, so a client that does not
	// read cannot grow the queue. A body being accumulated still reads. A full
	// buffer waits too: it is parsed again once the send completes. So does a
	// yielded turn, or an EOF here would drop the requests it left buffered.
	if queued && c.bodyNeed == 0 || c.resumeParse || c.readN == len(c.readBuf) {
		c.suppressRead()
		return
	}
	c.readDrained = false
	requested := len(c.readBuf) - c.readN
	nr, e := rawRead(c.fd, c.readBuf[c.readN:])
	if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
		// An earlier read already took what this readiness reported.
		c.readSuppressed = false
		submitIdleRecv(w.eng, c.fd, nil, 0)
		return
	}
	if e == 0 && nr == 0 && c.bodyNeed > 0 && c.sending() && w.writeTimeout > 0 {
		w.endIncompleteBody(c)
		return
	}
	if e != 0 || nr <= 0 {
		w.closeConn(c.fd)
		return
	}
	c.readDrained = c.bodyNeed == 0 && nr < requested
	if ev.Flags&evPeerShut != 0 {
		c.readSuppressed = true // the EOF behind this input is still to be read
	}
	c.readN += nr
	if w.hasTimeout {
		now := w.now
		if now == 0 {
			now = time.Now().UnixNano()
		}
		if !queued {
			c.lastActive = now // body bytes behind an unsent reply must not extend its write timeout
		}
		// Set the read deadline once, on the first byte of a request, and do not
		// refresh it while accumulating a body — otherwise a slow client trickling
		// body bytes resets the deadline forever (slowloris). The deadline then
		// bounds the whole request read, matching net/http's ReadTimeout.
		if c.readN == nr && c.bodyNeed == 0 && w.readTimeout > 0 {
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
	if c.fileStream != nil || c.sendFileFd > 0 {
		w.directSend(c)
		return
	}
	for c.readN > 0 {
		req, consumed, ok := parseHTTPRequestFast(c.readBuf[:c.readN], w.limits)
		if !ok {
			if w.readTimeout > 0 && c.readDeadline == 0 {
				// Bytes left behind an earlier request in the same read start
				// this one; recv only sets the deadline on an empty buffer.
				c.readDeadline = time.Now().UnixNano() + w.readTimeout // the batch clock can be older than a send
			}
			if c.readN < len(c.readBuf) {
				// Buffer has room — classify to decide whether to wait or error.
				st, need, expectContinue := classifyIncomplete(c.readBuf[:c.readN], w.limits)
				switch st {
				case parseNeedHeaderMore:
					// Compaction can leave a partial header after a full read.
					// EPOLLET will not notify again for bytes already in the socket.
					if runtime.GOOS == "linux" && (!c.readDrained || c.readSuppressed) && c.pending == 0 {
						// Flush this batch before a refill can queue more responses.
						if c.sendN < len(c.sendBuf) || c.sendFileFd > 0 || c.fileStream != nil {
							w.directSend(c)
							return
						}
						requested := len(c.readBuf) - c.readN
						n, err := rawRead(c.fd, c.readBuf[c.readN:])
						if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
							c.readSuppressed = false
							c.readDrained = true
						} else if err != 0 {
							w.closeConn(c.fd)
							return
						} else if n == 0 {
							// A read half-close must not discard responses already queued.
							c.keepAlive = false
							if c.sendN < len(c.sendBuf) || c.sendFileFd > 0 || c.fileStream != nil {
								w.directSend(c)
							} else {
								w.closeConn(c.fd)
							}
							return
						} else {
							c.readDrained = n < requested
							c.readN += n
							continue
						}
					}
				case parseBadRequest:
					w.writeAndClose(c, wingStatusClose(400))
					return
				case parseChunked:
					w.writeAndClose(c, wingStatusClose(501))
					return
				case parseBodyTooLarge:
					w.writeAndClose(c, wingStatusClose(413))
					return
				case parseNeedBody:
					if !w.beginBodyAccum(c, need, expectContinue) {
						w.writeAndClose(c, wingStatusClose(503))
						return
					}
					return
				case parseHeaderTooLarge:
					w.writeAndClose(c, wingStatusClose(431))
					return
				default: // parseMalformed
					w.retire(c)
					return
				}
			} else {
				// Buffer full — classify; if body needed start accum, else error.
				st, need, expectContinue := classifyIncomplete(c.readBuf[:c.readN], w.limits)
				switch st {
				case parseBadRequest:
					w.writeAndClose(c, wingStatusClose(400))
					return
				case parseBodyTooLarge:
					w.writeAndClose(c, wingStatusClose(413))
					return
				case parseNeedBody:
					if !w.beginBodyAccum(c, need, expectContinue) {
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
		req.proxyPolicy = w.proxyPolicy
		// Full request received — clear read deadline, update idle clock.
		if w.hasTimeout {
			c.readDeadline = 0
			c.lastActive = time.Now().UnixNano()
		}

		f := w.lookupPreset(req.method, req.path)
		finalizeRequestPath(req, f)
		if f.Dispatch != Inline {
			c.readDrained = false
		}

		// For async dispatch modes, only one in-flight handler per conn.
		if f.Dispatch != Inline && c.pending > 0 {
			// Don't consume — leave data in readBuf for re-parse after handleDone.
			break
		}
		if f.Dispatch == Takeover && (c.sendN < len(c.sendBuf) || c.sendFileFd > 0 || c.fileStream != nil) {
			// Keep the takeover request in readBuf until every earlier response is
			// complete; detaching now would abandon the event loop's send state.
			releaseRequest(req)
			w.directSend(c)
			return
		}

		// Consume parsed bytes.
		remaining := c.readN - consumed
		if remaining > 0 {
			finalizeRequestCommonHeaders(req)
			copy(c.readBuf, c.readBuf[consumed:c.readN])
		}
		c.readN = remaining
		c.responses++

		switch f.Dispatch {
		case Inline:
			if f.StaticResponse != nil {
				c.keepAlive = req.keepAlive && !f.staticClose
				c.sendBuf = append(c.sendBuf, f.StaticResponse...)
				releaseRequest(req)
			} else {
				resp := acquireResponseFor(req)
				resp.responseMode = f.ResponseMode
				resp.sendFileEnabled = true
				start := time.Now().UnixNano()
				w.serveRoute(resp, req, f)
				w.observeAdvisor(f, req.method, req.path, time.Now().UnixNano()-start)
				if resp.fileFd > 0 || resp.fileStream != nil {
					// Sendfile path: write headers, then sendfile for body.
					c.keepAlive = req.keepAlive && !resp.closeRequested()
					hdr := resp.buildZeroCopy()
					c.sendBuf = append(c.sendBuf, hdr...)
					c.sendFileFd = resp.fileFd
					c.sendFileRelease = resp.sendFileRelease
					resp.sendFileRelease = nil
					c.sendFileSize = resp.fileSize
					resp.fileFd = 0
					c.fileStream = resp.fileStream
					resp.fileStream = nil
					releaseResponse(resp)
					releaseRequest(req)
					w.directSend(c)
					return
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
				c.keepAlive = req.keepAlive && !resp.closeRequested()
				c.sendBuf = resp.appendTo(c.sendBuf)
				releaseResponse(resp)
				releaseRequest(req)
			}

		default:
			w.dispatchAsync(c, req, f)
			return
		}

		// At the limit, send before building more; the rest is parsed afterwards.
		if !c.keepAlive || len(c.sendBuf)-c.sendN >= maxQueuedSend {
			break
		}
	}
	if c.sendN < len(c.sendBuf) {
		// Direct write — skip epoll EPOLLOUT round-trip for inline responses.
		w.directSend(c)
	} else {
		submitIdleRecv(w.eng, c.fd, nil, 0)
	}
}

// reject503 answers a request the full Pool could not take and closes the
// connection after it. It queues on the connection rather than doneCh: this
// worker is doneCh's only reader, so a full doneCh would block it for good.
func (w *worker) reject503(c *conn) {
	c.pending--
	c.keepAlive = false
	c.sendBuf = append(c.sendBuf, resp503...)
}

// dispatchAsync hands a parsed request to its preset's Pool, Spawn or
// Takeover dispatch. The caller has already consumed the request's bytes from
// c.readBuf and checked that no other handler owns the connection.
func (w *worker) dispatchAsync(c *conn, req *wingRequest, f Preset) {
	switch f.Dispatch {
	case Pool:
		// Dispatch to goroutine pool.
		c.keepAlive = req.keepAlive
		c.pending++
		job := handlerJob{
			req:          req,
			fd:           c.fd,
			keepAlive:    req.keepAlive,
			responseMode: f.ResponseMode,
			directWrite:  c.sendN >= len(c.sendBuf) && c.sendFileFd == 0 && c.fileStream == nil,
		}
		select {
		case w.pool.jobs <- job:
		default:
			// Pool saturated. Running the handler here would block this
			// loop's other connections for as long as the handler waits.
			if !w.poolFullWarned && w.logger != nil {
				w.poolFullWarned = true
				w.logger.Warn("kruda: Wing handler pool is full, answering 503; raise WingConfig.HandlerPoolSize if this repeats",
					"worker", w.id, "pool_size", w.config.HandlerPoolSize)
			}
			releaseRequest(req)
			w.reject503(c)
		}
		// Send any buffered inline responses, then wait for pool completion.
		if c.sendN < len(c.sendBuf) {
			w.eng.SubmitSend(c.fd, nil)
		}
		return

	case Spawn:
		// New goroutine per request.
		c.keepAlive = req.keepAlive
		c.pending++
		directWrite := c.sendN >= len(c.sendBuf) && c.sendFileFd == 0 && c.fileStream == nil
		w.dispatchWG.Add(1)
		go func(req *wingRequest, fd int32, ka, directWrite bool, f Preset) {
			defer w.dispatchWG.Done()
			resp := acquireResponseFor(req)
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
			ka = ka && !resp.closeRequested()
			data := resp.buildZeroCopy()
			stream := resp.fileStream
			resp.fileStream = nil
			releaseResponse(resp)
			releaseRequest(req)
			if !directWrite || stream != nil {
				w.doneCh <- doneMsg{fd: fd, data: data, keepAlive: ka, stream: stream}
				w.wake()
				return
			}
			// Direct write from spawn goroutine.
			if len(data) == 0 {
				w.doneCh <- doneMsg{fd: fd, keepAlive: ka}
				w.wake()
				return
			}
			n, e := rawWrite(fd, data)
			if e == 0 && n == len(data) {
				w.doneCh <- doneMsg{fd: fd, keepAlive: ka}
			} else {
				written := 0
				if e == 0 {
					written = int(n)
				}
				w.doneCh <- doneMsg{fd: fd, data: data[written:], keepAlive: ka}
			}
			w.wake()
		}(req, c.fd, req.keepAlive, directWrite, f)
		if c.sendN < len(c.sendBuf) {
			w.eng.SubmitSend(c.fd, nil)
		}
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
		c.readBuf = nil // the goroutine reads into its own buffer from here on
		// The File is made here so Shutdown can wake its reads; the goroutine owns it.
		file := os.NewFile(uintptr(c.fd), "wing-takeover")
		gate := &takeoverGate{}
		c.takeoverFile, c.takeoverGate = file, gate
		answered := c.responses > 1 // the event loop wrote an earlier response
		w.dispatchWG.Add(1)
		go func() {
			defer w.dispatchWG.Done()
			w.takeoverLoop(req, c.fd, file, gate, leftover, f.ResponseMode, answered)
		}()
	}
}

// writeAndClose queues an error response and triggers a direct send, then closes.
func (w *worker) writeAndClose(c *conn, resp []byte) {
	c.keepAlive = false
	c.sendBuf = append(c.sendBuf, resp...)
	if c.sendN < len(c.sendBuf) {
		// Preserve sendN: sendBuf may already contain a partially written
		// response that must complete before this terminal response.
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
	out = appendBody(bodyBuf, src[:take], need)
	return out, src[take:]
}

// appendBody appends src to a body buffer of declared size need. It doubles
// instead of append's ~1.25x steps for large slices, and never past need, so
// a body arriving in many reads is copied a few times rather than dozens.
func appendBody(dst, src []byte, need int) []byte {
	if n := len(dst) + len(src); n > cap(dst) {
		newCap := max(2*cap(dst), n)
		if newCap > need {
			newCap = max(need, n)
		}
		grown := make([]byte, len(dst), newCap)
		copy(grown, dst)
		dst = grown
	}
	return append(dst, src...)
}

// beginBodyAccum reserves and starts body accumulation for a connection.
// Returns false if the per-worker in-flight budget would be exceeded.
func (w *worker) beginBodyAccum(c *conn, need int, expectContinue bool) bool {
	c.readDrained = false
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
	if len(c.bodyBuf) < c.bodyNeed {
		if expectContinue {
			c.sendBuf = append(c.sendBuf, wing100Continue...)
			c.responses++
		}
		// Flush only after this request owns its body budget, but before draining
		// can complete it and recursively parse a later pipelined request. Earlier
		// replies go out here too: they must not wait, on the write timeout's
		// clock, for the upload behind them.
		if c.sendN < len(c.sendBuf) {
			w.directSend(c) // non-blocking; rest of buf stays queued if partial
			if c.bodyNeed == 0 {
				return true // directSend closed the connection on a hard write error
			}
		}
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
	c.readDrained = false
	for len(c.bodyBuf) < c.bodyNeed {
		rn, e := rawRead(c.fd, c.readBuf)
		if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
			break
		}
		if e == 0 && rn == 0 && c.sending() && w.writeTimeout > 0 {
			w.endIncompleteBody(c)
			return false
		}
		if e != 0 || rn <= 0 {
			w.closeConn(c.fd)
			return true
		}
		var surplus []byte
		c.bodyBuf, surplus = accumBody(c.bodyBuf, c.readBuf[:rn], c.bodyNeed)
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

// endIncompleteBody handles an EOF in the middle of a request body while
// earlier responses are still being sent. That request can never complete, but
// those responses are sent before the connection closes, under WriteTimeout.
// Without one the connection closes at once, as a peer that also stops reading
// could otherwise hold it for good.
func (w *worker) endIncompleteBody(c *conn) {
	if w.maxInflightBody > 0 {
		atomic.AddInt64(&w.inflightBody, -int64(c.bodyNeed))
	}
	c.bodyNeed, c.bodyBuf, c.headerSnapshot = 0, nil, nil
	c.readN = 0
	c.keepAlive = false
}

// drainPipelinedAfterBody appends socket bytes that follow a just-completed body
// (the start of the next pipelined request) into readBuf after any surplus
// already relocated to its front. Non-blocking: it stops at EAGAIN or a full
// buffer, so on the common case where no pipelined request is waiting it costs a
// single EAGAIN read. A pipelined burst exceeding the read buffer is bounded by
// the same buffer-size limit as ordinary pipelining.
func (w *worker) drainPipelinedAfterBody(c *conn) {
	c.readDrained = false
	for c.readN < len(c.readBuf) {
		rn, e := rawRead(c.fd, c.readBuf[c.readN:])
		if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
			return
		}
		if e != 0 || rn <= 0 {
			// Error or EOF: the accumulated request is still dispatched; teardown
			// happens on the post-response keep-alive read, which must not be skipped.
			c.readSuppressed = true
			return
		}
		c.readN += rn
	}
}

// finishBodyAccum re-parses the completed request and dispatches it inline.
func (w *worker) finishBodyAccum(c *conn) {
	if w.maxInflightBody > 0 {
		atomic.AddInt64(&w.inflightBody, -int64(c.bodyNeed))
	}
	// The request takes the accumulated body as its own; only the header is parsed again.
	header, body := c.headerSnapshot, c.bodyBuf
	c.bodyNeed = 0
	c.headerSnapshot = nil
	c.bodyBuf = nil
	req, ok := parseHTTPRequestOwnedBody(header, body, w.limits)
	if !ok {
		w.retire(c)
		return
	}
	req.fd = c.fd
	req.ctx = c.ctx
	req.remoteAddrRef = &c.remoteAddr
	req.trustProxy = w.trustProxy
	req.proxyPolicy = w.proxyPolicy
	if w.hasTimeout {
		c.readDeadline = 0
		c.lastActive = time.Now().UnixNano()
	}
	f := w.lookupPreset(req.method, req.path)
	finalizeRequestPath(req, f)
	w.dispatchAccumulated(c, req, f)
}

// dispatchAccumulated dispatches a body-accumulated request with its preset's
// dispatch, so a Pool, Spawn or Takeover route does not run on the event loop
// because its body arrived in pieces. It runs inline only when another
// handler still owns the connection. A Takeover route behind an unsent reply
// runs as Spawn, since the fd cannot be detached yet.
func (w *worker) dispatchAccumulated(c *conn, req *wingRequest, f Preset) {
	c.readDrained = false
	c.responses++
	// A Hijack-preset route (WebSocket upgrade) is a bodyless GET. A body forces
	// inline accumulation, where c.ResponseWriter() is not an http.Hijacker — so
	// reject it cleanly (400) rather than letting the upgrade fail opaquely.
	if f.ResponseMode == responseHijack {
		c.keepAlive = false
		c.sendBuf = append(c.sendBuf, wingStatusClose(400)...)
		releaseRequest(req)
		w.directSend(c)
		return
	}
	finalizeRequestCommonHeaders(req)
	if f.Dispatch != Inline && c.pending == 0 {
		if f.Dispatch == Takeover && (c.sendN < len(c.sendBuf) || c.sendFileFd != 0 || c.fileStream != nil) {
			// An earlier reply is still unsent, so the fd cannot be detached.
			// Spawn keeps the handler off the loop and queues its reply after.
			f.Dispatch = Spawn
		}
		w.dispatchAsync(c, req, f)
		return
	}
	resp := acquireResponseFor(req)
	resp.responseMode = f.ResponseMode
	start := time.Now().UnixNano()
	w.serveRoute(resp, req, f)
	w.observeAdvisor(f, req.method, req.path, time.Now().UnixNano()-start)
	c.keepAlive = req.keepAlive && !resp.closeRequested()
	data := resp.buildZeroCopy()
	c.sendBuf = append(c.sendBuf, data...)
	c.fileStream = resp.fileStream
	resp.fileStream = nil
	releaseResponse(resp)
	releaseRequest(req)
	if c.sendN < len(c.sendBuf) {
		w.directSend(c)
	} else {
		submitIdleRecv(w.eng, c.fd, nil, 0)
	}
}

func (w *worker) handleDone(msg doneMsg) {
	if msg.readStream != nil {
		c := w.conns[msg.fd]
		if c == nil || c.fileStream != msg.readStream {
			return
		}
		msg.readStream.reading = false
		if msg.readErr != nil {
			w.retire(c)
			return
		}
		msg.readStream.remaining -= int64(len(msg.data))
		c.sendBuf = msg.data
		w.directSend(c)
		return
	}
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
		msg.stream.close()
		return
	}
	c.pending--
	if c.closePending {
		msg.stream.close()
		if c.pending == 0 {
			w.retire(c)
		}
		return
	}
	c.keepAlive = msg.keepAlive
	c.fileStream = msg.stream
	if len(msg.data) == 0 && msg.stream == nil {
		// Pool goroutine already wrote the response directly.
		if c.keepAlive {
			if c.readN > 0 {
				w.tryParse(c)
			} else if c.readSuppressed {
				w.handleRecv(event{Fd: c.fd, ConnPtr: unsafe.Pointer(c)})
			} else {
				submitIdleRecv(w.eng, c.fd, nil, 0)
			}
		} else {
			w.closeAfterFinal(c)
		}
		return
	}
	// Partial write fallback — pool couldn't write everything.
	c.sendBuf = append(c.sendBuf, msg.data...)
	w.eng.SubmitSend(c.fd, nil)
}

// sendingFinal reports whether c only has its last response left to send. It
// is false while a handler runs: that handler's request may alias readBuf.
func (c *conn) sendingFinal() bool {
	return !c.keepAlive && c.pending == 0 && c.bodyNeed == 0
}

// discardResult is what discardInput found when it stopped.
type discardResult uint8

const (
	discardDrained discardResult = iota // no more input for now
	discardEnded                        // EOF or a failed socket
	discardFull                         // maxDiscardInput dropped; the rest stays unread
)

// discardInput drops input that arrives behind a connection's final response.
// Running it would revive a connection that promised to close, and closing
// over unread input would reset it and cut that response. A connection drops
// at most maxDiscardInput in all, so a peer cannot keep the worker copying.
func (w *worker) discardInput(c *conn) discardResult {
	c.discarded += c.readN // input already read is unread by the handler too
	c.readN = 0
	c.readSuppressed = false
	c.readDrained = false
	if w.discardBuf == nil {
		w.discardBuf = make([]byte, 16<<10)
	}
	for {
		room := min(len(w.discardBuf), maxDiscardInput-c.discarded)
		if room <= 0 {
			return discardFull
		}
		n, e := rawRead(c.fd, w.discardBuf[:room])
		if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
			return discardDrained
		}
		if e != 0 || n == 0 {
			return discardEnded
		}
		c.discarded += n
	}
}

// closeAfterFinal ends a connection whose final response is fully written. It
// ends the write side and drops input until the peer closes, as net/http's
// closeWriteAndWait: the peer reads the response and an EOF, not a reset that
// can discard it. That holds with nothing unread yet too, since a request can
// still land before the close. The write side ends before any input is dropped,
// and past maxDiscardInput the connection reads no more but still waits out
// its linger: closing then would reset a response the peer is still reading.
// lingerDelay bounds the wait, timeouts or not.
func (w *worker) closeAfterFinal(c *conn) {
	if !c.shutWrite() || w.discardInput(c) == discardEnded {
		w.closeConn(c.fd) // the peer is gone: nothing left to protect
		return
	}
	if c.lingerUntil == 0 {
		w.releaseBody(c) // an upload abandoned here must not hold the budget through the wait
		c.lingerUntil = monoNow() + lingerDelay
		if !w.waitShortened {
			w.setWait(lingerWaitMillis)
			w.waitShortened = true
		}
		w.lingering++
		w.lingerQ = append(w.lingerQ, lingerEntry{c, c.lingerUntil})
		c.sendBuf, c.sendN = nil, 0 // sent
		c.readBuf, c.readN = nil, 0 // discardInput reads into the worker's buffer
	}
	w.eng.SubmitRecv(c.fd, nil, 0) // drops a write interest; kqueue needs the read armed
}

// shutWrite ends c's write side once. darwin refuses a second SHUT_WR.
func (c *conn) shutWrite() bool {
	if !c.wrShut {
		c.wrShut = syscall.Shutdown(int(c.fd), syscall.SHUT_WR) == nil
	}
	return c.wrShut
}

// maxDiscardInput bounds the input a closing connection drops. A peer that
// sends more is not waiting for its response, and the connection closes.
const maxDiscardInput = 256 << 10

// lingerDelay is how long a connection that ended its write side waits for the
// peer to close, as net/http's rstAvoidanceDelay. A variable only so a test can
// hold a linger open while it checks it.
var lingerDelay = int64(500 * time.Millisecond)

// directSend attempts a non-blocking write. If partial, falls back to epoll EPOLLOUT.
func (w *worker) directSend(c *conn) {
	for c.sendN < len(c.sendBuf) {
		buf := c.sendBuf[c.sendN:]
		r, e := rawWrite(c.fd, buf)
		if e != 0 {
			if e == syscall.EAGAIN || e == syscall.EWOULDBLOCK {
				c.readDrained = false
				w.eng.SubmitSend(c.fd, nil)
				return
			}
			w.retire(c)
			return
		}
		c.sendN += r
		if w.idleTimeout > 0 {
			c.lastSent = w.now
		}
	}
	c.sendBuf = c.sendBuf[:0]
	c.sendN = 0
	if stream := c.fileStream; stream != nil {
		// A sent chunk leaves c.sendBuf on stream.buf. Detach it: the next read
		// refills that array, and after the last chunk the connection must not
		// keep a 32 KiB stream buffer for later responses.
		if cap(c.sendBuf) > 0 && cap(stream.buf) > 0 && &c.sendBuf[:1][0] == &stream.buf[:1][0] {
			c.sendBuf = nil
		}
		if stream.remaining > 0 {
			if !stream.reading {
				stream.reading = true
				// Raw epoll waits must release the scheduler once this worker reads asynchronously.
				if engine, ok := w.eng.(interface{ enableAsync() }); ok {
					engine.enableAsync()
				}
				n := int(min(int64(32*1024), stream.remaining))
				if cap(stream.buf) < n {
					stream.buf = make([]byte, n)
				}
				chunk := stream.buf[:n]
				fd := c.fd

				go func() {
					err := w.readStreamChunk(stream, chunk)
					w.fileReadMu.Lock()
					if w.fileReadStopped {
						stream.close()
					} else {
						select {
						case w.doneCh <- doneMsg{fd: fd, data: chunk, readStream: stream, readErr: err}:
							w.wake()
						case <-w.fileReadStop:
							stream.close()
						}
					}
					w.fileReadMu.Unlock()
				}()
			}
			return
		}
		stream.close()
		c.fileStream = nil
	}
	// Sendfile: transfer file body after headers are written.
	if c.sendFileFd > 0 {
		for c.sendFileSize > 0 {
			n, err := sendfile(c.fd, c.sendFileFd, nil, int(c.sendFileSize))
			if n > 0 {
				c.sendFileSize -= int64(n)
				if w.idleTimeout > 0 {
					c.lastSent = w.now
				}
			}
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					// Socket buffer full — wait for writable notification.
					c.readDrained = false
					w.eng.SubmitSend(c.fd, nil)
					return
				}
				w.retire(c)
				return
			}
			if n == 0 {
				w.retire(c)
				return
			}
		}
		syscall.Close(int(c.sendFileFd))
		c.sendFileFd = 0
		if c.sendFileRelease != nil {
			c.sendFileRelease()
			c.sendFileRelease = nil
		}
	}
	if c.bodyNeed > 0 {
		// Body accumulation owns receive arming; do not apply final-response handling.
		return
	}
	if c.pending > 0 {
		// An async handler still owns this physical connection. Keep the fd open
		// until its completion is processed so the kernel cannot recycle its number.
		return
	}
	if !c.keepAlive {
		w.closeAfterFinal(c)
		return
	}
	if c.readDeadline > 0 {
		// Nothing was read during the send, so a partial request behind it
		// gets its whole interval from here.
		c.readDeadline = time.Now().UnixNano() + w.readTimeout // the batch clock can be older than the send
	}
	if c.readN > 0 {
		w.parseAfterSend(c)
		return
	}
	// A short stream read or EAGAIN exhausts the current EPOLLET input. This evidence
	// survives only synchronous inline work; a later edge will deliver new data.
	if runtime.GOOS == "linux" && c.readDrained && !c.readSuppressed {
		return
	}
	c.readDrained = false
	r, e := rawRead(c.fd, c.readBuf)
	if e == 0 && r > 0 {
		c.readN = r
		c.readDrained = r < len(c.readBuf)
		w.parseAfterSend(c)
		return
	}
	if e != syscall.EAGAIN && e != syscall.EWOULDBLOCK {
		// EOF or a failed socket with everything sent. No later edge reports it.
		w.closeConn(c.fd)
		return
	}
	c.readSuppressed = false
	// Speculative read got EAGAIN — arm read for next data arrival.
	// On Linux this is redundant (ET EPOLLIN already active) but on
	// kqueue/darwin SubmitRecv is required to register EVFILT_READ.
	submitIdleRecv(w.eng, c.fd, nil, 0)
}

// retire ends a live connection, dropping output it has not sent yet. Once a
// response may be in the socket, the close goes through closeAfterFinal, so
// input arriving behind it cannot reset that response. With a handler in
// flight the close waits for it.
func (w *worker) retire(c *conn) {
	c.fileStream.close()
	c.fileStream = nil
	c.sendBuf = c.sendBuf[:0]
	c.sendN = 0
	if c.sendFileFd > 0 {
		syscall.Close(int(c.sendFileFd))
		c.sendFileFd = 0
		if c.sendFileRelease != nil {
			c.sendFileRelease()
			c.sendFileRelease = nil
		}
		c.sendFileSize = 0
	}
	c.keepAlive = false
	if c.pending > 0 {
		c.closePending = true
		return
	}
	if c.responses == 0 {
		w.closeConn(c.fd)
		return
	}
	w.closeAfterFinal(c)
}

// takeoverSpinReads bounds the non-blocking read attempts a Takeover keep-alive
// loop makes before parking on the runtime netpoller. A small spin can catch an
// already-buffered next request without a netpoller wake hop; the fallback park
// keeps the takeover thread count low when the connection is genuinely idle.
const takeoverSpinReads = 8

// maxReusedTakeoverOut bounds what an idle Takeover connection keeps: an
// output buffer grown past it is dropped after the write, so one large reply
// does not stay attached to the connection.
const maxReusedTakeoverOut = 4 << 10

// maxReusedTakeoverBody bounds the body accumulator a Takeover connection
// keeps between requests; a larger one is dropped once its body is copied.
const maxReusedTakeoverBody = 64 << 10

// takeoverBufPool provides read buffers for Takeover goroutines.
var takeoverBufPool = sync.Pool{New: func() any { b := make([]byte, 8192); return &b }}

// takeoverReply sends a reply Wing makes itself, such as an error status or
// 100 Continue, unless Shutdown has begun: then no request is answered any more.
func (w *worker) takeoverReply(f *os.File, data []byte) bool {
	return !w.stopping() && w.writeTakeover(f, data) == nil
}

func (w *worker) writeTakeover(f *os.File, data []byte) error {
	if w.writeTimeout > 0 {
		if err := f.SetWriteDeadline(time.Now().Add(time.Duration(w.writeTimeout))); err != nil {
			return err
		}
	}
	_, err := f.Write(data)
	if w.writeTimeout > 0 {
		if clearErr := f.SetWriteDeadline(time.Time{}); err == nil {
			err = clearErr
		}
	}
	return err
}

func readFileChunk(reader io.Reader, chunk []byte) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("file reader panic: %v", p)
		}
	}()
	_, err = io.ReadFull(reader, chunk)
	return err
}

func (w *worker) readStreamChunk(stream *fileStream, chunk []byte) error {
	select {
	case <-w.fileReadStop:
		return io.ErrClosedPipe
	default:
	}
	if !stream.beginRead() {
		return io.ErrClosedPipe
	}
	result := make(chan error, 1)
	go func() { err := readFileChunk(stream.reader, chunk); stream.finishRead(); result <- err }()
	var timeout <-chan time.Time
	var timer *time.Timer
	if w.writeTimeout > 0 {
		timer = time.NewTimer(time.Duration(w.writeTimeout))
		timeout = timer.C
		defer timer.Stop()
	}
	select {
	case err := <-result:
		return err
	case <-stream.done:
		return io.ErrClosedPipe
	case <-w.fileReadStop:
		stream.close()
		return io.ErrClosedPipe
	case <-timeout:
		stream.close()
		return os.ErrDeadlineExceeded
	}
}

func (w *worker) writeFileStream(f *os.File, stream *fileStream) error {
	if w.writeTimeout > 0 {
		if err := f.SetWriteDeadline(time.Now().Add(time.Duration(w.writeTimeout))); err != nil {
			return err
		}
		defer f.SetWriteDeadline(time.Time{})
	}
	if stream.remaining <= 0 {
		return nil // a handler may declare a non-positive size; nothing to send
	}
	// Write is synchronous, so one buffer serves every chunk.
	buf := make([]byte, int(min(int64(32*1024), stream.remaining)))
	for stream.remaining > 0 {
		chunk := buf[:int(min(int64(len(buf)), stream.remaining))]
		if err := w.readStreamChunk(stream, chunk); err != nil {
			return err
		}
		if _, err := f.Write(chunk); err != nil {
			return err
		}
		stream.remaining -= int64(len(chunk))
	}
	return nil
}

// takeoverLoop owns a connection fd: loops read→handle→write through an
// *os.File so a blocked read parks the goroutine on the runtime netpoller
// instead of pinning an OS thread. The previous blocking-syscall variant
// ran one OS thread per connection (237-250 threads, 1.40 context switches
// per request at c256 on /db) — identical CPU per request to this model,
// but scheduler pressure inflated the time a pgx pool conn stayed checked
// out, costing ~6% throughput (results/forensics-20260612T150500Z).
func (w *worker) takeoverLoop(first *wingRequest, fd int32, f *os.File, g *takeoverGate, leftover []byte, mode responseMode, answered bool) {
	// fds arrive non-blocking from accept4/SetNonblock. os.NewFile registers
	// a non-blocking fd with the runtime poller, so Read/Write below park
	// the goroutine, never the thread.
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
		w.streamTakeover(first, fd, f, g, answered)
		return
	}

	if mode == responseHijack {
		w.hijackTakeover(first, fd, f, g, leftover, answered)
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
	requestStarted := false
	// out is serialized into again once the synchronous write has returned.
	var out []byte
	final := false // the last response was written; unread input must not reset it

	for served := false; ; served = true {
		var static Preset // the route's, when it has a prebuilt response
		if served && w.stopping() {
			releaseRequest(req) // shutting down: no new request starts
			final = true        // the previous response is complete: drain behind it
			goto done
		}
		if served {
			// A later request on this connection may be a static route, which
			// runs no handler whatever its mode, as on a fresh connection, or
			// a Stream or Hijack route, which needs its own writer rather than
			// a buffered one.
			preset := w.presets.Lookup(req.method, req.path)
			mode = preset.ResponseMode
			switch {
			case preset.StaticResponse != nil:
				static = preset
			case preset.ResponseMode == responseStream:
				releaseTakeoverBuffers(&buf, &bodyBuf, grewBuf)
				w.streamTakeover(req, fd, f, g, answered)
				return
			case preset.ResponseMode == responseHijack:
				leftover := append([]byte(nil), buf[:readN]...)
				releaseTakeoverBuffers(&buf, &bodyBuf, grewBuf)
				w.hijackTakeover(req, fd, f, g, leftover, answered)
				return
			}
		}
		if mode == responseKeepAliveStream && static.StaticResponse == nil {
			if !keepAlive || req.method != "GET" {
				releaseTakeoverBuffers(&buf, &bodyBuf, grewBuf)
				w.streamTakeover(req, fd, f, g, answered)
				return
			}
			reuse, sent := w.writeKeepAliveStream(req, f)
			releaseRequest(req)
			if !reuse {
				// A response that never got its preamble out has nothing to
				// protect, unless an earlier one on this connection does.
				final = sent || answered
				goto done
			}
		} else {
			// Handle request.
			resp := acquireResponseFor(req)
			func() {
				defer func() {
					if r := recover(); r != nil {
						resp.WriteHeader(500)
						resp.Write([]byte("Internal Server Error\n"))
					}
				}()
				if static.StaticResponse != nil {
					resp.setStatic(static.StaticResponse, static.staticClose)
					return
				}
				w.handler.ServeKruda(resp, req)
			}()
			keepAlive = keepAlive && !resp.closeRequested()
			resp.buf = out[:0]
			data := resp.buildZeroCopy()
			builtIntoOut := resp.staticResp == nil
			resp.buf = nil // out stays with this loop, never with the pooled response
			stream := resp.fileStream
			resp.fileStream = nil
			releaseResponse(resp)
			releaseRequest(req)

			// Write full response — os.File loops over partial writes and parks
			// on EAGAIN via the runtime poller.
			if werr := w.writeTakeover(f, data); werr != nil {
				stream.close()
				keepAlive = false
				goto done
			}
			if builtIntoOut && cap(data) <= maxReusedTakeoverOut {
				out = data[:0]
			}
			if stream != nil {
				err := w.writeFileStream(f, stream)
				stream.close()
				if err != nil {
					keepAlive = false
					goto done
				}
			}
		}

		answered = true // from here on, every exit must drain behind this response
		if !keepAlive {
			final = true
			goto done
		}
		if readN == 0 && w.idleTimeout > 0 {
			if err := f.SetReadDeadline(time.Now().Add(time.Duration(w.idleTimeout))); err != nil {
				keepAlive = false
				goto done
			}
			if w.stopping() {
				goto done // that deadline may have replaced Shutdown's wake-up
			}
		}

		// Read next request — parks the goroutine until data arrives.
		for {
			if w.stopping() {
				final = true // after a complete response: drain behind it
				goto done    // not even a 100 Continue or an error reply once Shutdown began
			}
			if testTakeoverParseHook != nil && readN > 0 {
				testTakeoverParseHook()
			}
			if readN > 0 && !requestStarted {
				requestStarted = true
				deadline := time.Time{}
				if w.readTimeout > 0 {
					deadline = time.Now().Add(time.Duration(w.readTimeout))
				}
				if err := f.SetReadDeadline(deadline); err != nil {
					keepAlive = false
					goto done
				}
				if w.stopping() {
					goto done // that deadline may have replaced Shutdown's wake-up
				}
			}
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
					r.proxyPolicy = w.proxyPolicy
					req = r
					keepAlive = req.keepAlive
					requestStarted = false
					if err := f.SetReadDeadline(time.Time{}); err != nil {
						keepAlive = false
						goto done
					}
					goto next
				}
				// Classify the incomplete/rejected request.
				st, need, expectContinue := classifyIncomplete(buf[:readN], w.limits)
				switch st {
				case parseNeedHeaderMore:
					// Need more header bytes — keep reading.
					if readN >= len(buf) {
						// Buffer full but headers still incomplete: 431.
						final = w.takeoverReply(f, wingStatusClose(431))
						keepAlive = false
						goto done
					}
				case parseBadRequest:
					final = w.takeoverReply(f, wingStatusClose(400))
					keepAlive = false
					goto done
				case parseChunked:
					final = w.takeoverReply(f, wingStatusClose(501))
					keepAlive = false
					goto done
				case parseBodyTooLarge:
					final = w.takeoverReply(f, wingStatusClose(413))
					keepAlive = false
					goto done
				case parseNeedBody:
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
							final = w.takeoverReply(f, wingStatusClose(503))
							keepAlive = false
							goto done
						}
					}
					if expectContinue {
						if !w.takeoverReply(f, wing100Continue) {
							if w.maxInflightBody > 0 {
								atomic.AddInt64(&w.inflightBody, -int64(need))
							}
							keepAlive = false
							goto done
						}
						answered = true // a provisional response is in the socket too
					}
					headerSnapshot := make([]byte, headerEnd)
					copy(headerSnapshot, buf[:headerEnd])
					// Seed bodyBuf with already-received body bytes. A small bodyBuf
					// is reused across this connection's requests (see below).
					if bodyBuf == nil {
						bodyBuf = make([]byte, 0, min(need, 8192))
					}
					bodyBuf = bodyBuf[:0]
					if have := readN - headerEnd; have > 0 {
						bodyBuf = appendBody(bodyBuf, buf[headerEnd:readN], need)
					}
					readN = 0
					// Read remaining body bytes, reusing buf as a temp chunk buffer.
					// The request deadline was set when its first bytes arrived and is
					// not refreshed here, so trickling cannot extend it indefinitely.
					for len(bodyBuf) < need {
						n, rerr := f.Read(buf)
						if n > 0 {
							bodyBuf = appendBody(bodyBuf, buf[:n], need)
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
					requestStarted = false
					// Body fully accumulated — release the budget reservation
					// (mirrors finishBodyAccum). Any later exit on this iteration is
					// post-accumulation, so it must not refund again.
					if w.maxInflightBody > 0 {
						atomic.AddInt64(&w.inflightBody, -int64(need))
					}
					// Carry bytes beyond the body (start of the next pipelined
					// request) back into buf so the next iteration parses them
					// instead of dropping them.
					if surplus := len(bodyBuf) - need; surplus > 0 {
						readN = copy(buf, bodyBuf[need:])
					}
					// The header is not joined to the body again. A small accumulator
					// is kept for the next request, so the request gets a copy; a
					// large one is handed to the request rather than held while the
					// connection idles.
					var body []byte
					if cap(bodyBuf) > maxReusedTakeoverBody {
						body = bodyBuf[:need:need]
						bodyBuf = nil
					} else {
						body = make([]byte, need)
						copy(body, bodyBuf[:need])
						bodyBuf = bodyBuf[:0]
					}
					r2, ok2 := parseHTTPRequestOwnedBody(headerSnapshot, body, w.limits)
					if !ok2 {
						keepAlive = false
						goto done
					}
					r2.fd = fd
					r2.ctx = connCtx
					r2.remoteAddrRef = remoteAddrRef
					r2.trustProxy = w.trustProxy
					r2.proxyPolicy = w.proxyPolicy
					req = r2
					keepAlive = req.keepAlive
					goto next
				case parseHeaderTooLarge:
					final = w.takeoverReply(f, wingStatusClose(431))
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
				if serr == syscall.EAGAIN || serr == syscall.EINTR {
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
	// The loop can stop before a request it did not answer, at Shutdown or on
	// a failed write; the response before it still needs the drain.
	final = final || answered
	if final {
		bodyBuf = nil
		if grewBuf {
			buf = nil // not kept through the wait: a pooled buffer is enough to drain
			lingerTakeoverConn(f, fd, g)
		} else {
			lingerTakeover(f, fd, buf, g)
		}
	}
	// Hand the File to the worker loop: it deletes conn bookkeeping first
	// and then closes through the File. Do NOT close here — the kernel
	// could recycle the fd number while the conns map still references it.
	w.doneCh <- doneMsg{fd: fd, keepAlive: false, file: f}
	w.wake()
}

// lingerTakeover is closeAfterFinal for a Takeover socket, which this goroutine
// owns: buf is free, and the read deadline bounds the wait.
func lingerTakeover(f *os.File, fd int32, buf []byte, g *takeoverGate) {
	if testFinalDrainHook != nil {
		testFinalDrainHook()
	}
	// Even with nothing unread now, a request can still land before the close
	// and reset it, and with it any response bytes still in the send queue.
	// The write side ends before any input is dropped. A goroutine can afford
	// to wait for the peer's FIN; the slot is held until then, lingerDelay at
	// most.
	until := time.Now().Add(time.Duration(lingerDelay))
	if !g.startDrain(f, fd) {
		return
	}
	if testDrainingHook != nil {
		testDrainingHook()
	}
	for discarded := 0; discarded < maxDiscardInput; {
		n, err := f.Read(buf[:min(len(buf), maxDiscardInput-discarded)])
		if discarded += n; err != nil {
			return
		}
	}
	// Past the cap nothing more is read, but closing now would reset a
	// response the peer is still reading.
	time.Sleep(time.Until(until))
}

// releaseTakeoverBuffers lets go of the keep-alive loop's buffers before a
// Stream or Hijack takes the connection, as they would otherwise stay
// reachable for that whole session. A pool-sized read buffer is kept: the
// loop returns it to the pool when it ends.
func releaseTakeoverBuffers(buf, bodyBuf *[]byte, grew bool) {
	if grew {
		*buf = nil
	}
	*bodyBuf = nil
}

// lingerTakeoverConn is lingerTakeover for a path that holds no read buffer.
func lingerTakeoverConn(f *os.File, fd int32, g *takeoverGate) {
	bp := takeoverBufPool.Get().(*[]byte)
	lingerTakeover(f, fd, *bp, g)
	takeoverBufPool.Put(bp)
}

// streamTakeover runs a single streaming response (Stream preset) over the
// takeover fd f. Unlike the keep-alive takeover loop, it gives the handler a
// flushing wingStreamWriter so c.SSE()/c.Stream() write incrementally, installs
// a cancellable context so SSEStream.Done() fires on disconnect, and runs a
// read-watcher that cancels that context when the client closes its half. The
// streaming response is not keep-alive-reused; on return the fd is closed
// exactly once via the shared takeover-done path (doneMsg.file).
func (w *worker) streamTakeover(first *wingRequest, fd int32, f *os.File, g *takeoverGate, afterResponse bool) {
	// Make the handler's c.Context() cancellable from the conn context so the
	// disconnect watcher can fire SSEStream.Done(). first.ctx flows into c.ctx
	// during ServeKruda (app_serve.go: c.ctx = r.Context()).
	ctx, cancel := context.WithCancel(first.ctx)
	first.ctx = ctx

	sw := newWingStreamWriter(f, time.Duration(w.writeTimeout))
	sw.head = first.method == "HEAD"

	watched := make(chan struct{})
	go func() {
		watchStreamDisconnect(f, cancel)
		close(watched)
	}()

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
	sw.finish() // a handler that wrote no body has not sent its headers yet

	cancel()
	_ = f.SetReadDeadline(time.Now()) // ends the watcher's read, so one reader owns the socket again
	<-watched
	// Return the request to the pool, matching the other dispatch paths. Safe
	// here: the handler has returned, the watcher has stopped, and what follows
	// uses only fd + the *os.File — nothing references first afterward.
	releaseRequest(first)
	if testStreamTeardownHook != nil {
		testStreamTeardownHook()
	}
	if sw.headersSent && !sw.broken || afterResponse {
		lingerTakeoverConn(f, fd, g) // the watcher may not have taken the last input
	}

	// Close the fd exactly once via the shared takeover-done path. The conn is
	// already takenOver, so handleDone removes bookkeeping and closes through
	// the File — do NOT add a second close here.
	w.doneCh <- doneMsg{fd: fd, keepAlive: false, file: f}
	w.wake()
}

// maxQueuedSend is how much unsent output stops a connection from parsing
// further pipelined requests. One response may pass it.
const maxQueuedSend = 256 << 10

// sending reports output the client has yet to make room for. A yielded turn
// counts: it waits for the socket to be writable.
func (c *conn) sending() bool {
	return c.sendN < len(c.sendBuf) || c.fileStream != nil || c.sendFileFd > 0 || c.resumeParse
}

func (c *conn) suppressRead() {
	c.readSuppressed = true
	c.readDrained = false
	if testSuppressedRecvHook != nil {
		testSuppressedRecvHook(c.fd)
	}
}

// maxParseDepth is how many read batches one connection may run per event.
const maxParseDepth = 16

// parseAfterSend is the only way back from a send into tryParse. A client that
// keeps the socket readable and reads its responses would otherwise nest
// without end. At the limit it yields through a write event: buffered input
// gets no further read edge.
func (w *worker) parseAfterSend(c *conn) {
	if c.parseDepth >= maxParseDepth {
		c.resumeParse = true
		w.eng.SubmitSend(c.fd, nil)
		return
	}
	c.parseDepth++
	w.tryParse(c)
	c.parseDepth--
}

func (w *worker) handleSend(ev event) {
	var c *conn
	if ev.ConnPtr != nil {
		c = (*conn)(ev.ConnPtr)
	} else {
		c = w.conns[ev.Fd]
	}
	if c != nil && c.lingerUntil > 0 {
		w.closeAfterFinal(c) // nothing is left to send; the event may hide input
		return
	}
	if c != nil {
		// EPOLLOUT takes priority over a simultaneous EPOLLIN in the engine, so
		// this event may hide input or an EOF that gets no other edge.
		c.readDrained = false
		c.readSuppressed = true
	}
	if c == nil || (len(c.sendBuf) == 0 && c.sendFileFd == 0 && c.fileStream == nil) {
		// No pending data or sendfile — remove EPOLLOUT, listen for EPOLLIN.
		if c != nil {
			w.eng.SubmitRecv(c.fd, nil, 0)
			if c.resumeParse {
				c.resumeParse = false
				w.directSend(c)
			}
		}
		return
	}
	if c.sendingFinal() {
		// kqueue does not watch for reads while a send waits, so drop input
		// here before the send can finish and close over it.
		w.discardInput(c)
	}
	w.directSend(c)
	if c.bodyNeed > 0 && !c.sending() && w.conns[c.fd] == c {
		// Body bytes this event may have hidden. Not before the send completes:
		// an EOF there closes, over whatever was still queued.
		w.drainBodyAccum(c)
	}
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
	w.releaseBody(c)
	if c.lingerUntil > 0 {
		c.lingerUntil = 0
		w.lingering--
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.fileStream.close()
	c.fileStream = nil
	if c.sendFileFd > 0 {
		syscall.Close(int(c.sendFileFd))
		c.sendFileFd = 0
		if c.sendFileRelease != nil {
			c.sendFileRelease()
			c.sendFileRelease = nil
		}
	}
}

// lingerPoll is how often Shutdown reads lingering connections; the stopped
// loop no longer reports their input.
const lingerPoll = 5 * time.Millisecond

// lingerAtShutdown keeps dropping input behind final responses that linger,
// as the running loop would: closing one early can reset the response's
// unread tail. With wait it returns once all have closed, each by its deadline.
func (w *worker) lingerAtShutdown(wait bool) {
	for w.lingering > 0 {
		now := monoNow()
		for fd, c := range w.conns {
			if c.lingerUntil > 0 && (now >= c.lingerUntil || w.discardInput(c) == discardEnded) {
				w.closeConn(fd)
			}
		}
		if !wait || w.lingering == 0 {
			return
		}
		time.Sleep(lingerPoll)
	}
}

// rawRead is read(2) on a non-blocking socket. A signal can interrupt it
// without changing the socket, so EINTR is retried rather than taken for a
// failed connection.
func rawRead(fd int32, p []byte) (int, syscall.Errno) {
	for {
		n, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)))
		if e != syscall.EINTR {
			return int(n), e
		}
	}
}

// rawWrite is write(2) on a non-blocking socket, retried on EINTR as rawRead.
func rawWrite(fd int32, p []byte) (int, syscall.Errno) {
	for {
		n, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)))
		if e != syscall.EINTR {
			return int(n), e
		}
	}
}

// releaseBody gives back c's in-flight body reservation, so a connection that
// ends mid-upload (timeout, disconnect, parse failure, linger) does not keep
// the per-worker budget. It zeroes bodyNeed, as finishBodyAccum does, so the
// reservation is never returned twice.
func (w *worker) releaseBody(c *conn) {
	if c.bodyNeed > 0 && w.maxInflightBody > 0 {
		atomic.AddInt64(&w.inflightBody, -int64(c.bodyNeed))
	}
	c.bodyNeed = 0
	c.bodyBuf = nil
	c.headerSnapshot = nil
}

func (w *worker) closeConn(fd int32) {
	w.removeConnBookkeeping(fd)
	delete(w.conns, fd)
	w.eng.SubmitClose(fd)
}

func (w *worker) wake() { w.eng.PostWake() }

// externalWake is wake for Shutdown, which can reach a worker that has
// already stopped and closed its wake fd.
func (w *worker) externalWake() {
	w.wakeMu.Lock()
	if !w.wakeClosed {
		w.wake()
	}
	w.wakeMu.Unlock()
}

func (w *worker) cleanup() {
	w.fileReadOnce.Do(func() {
		if w.fileReadStop != nil {
			close(w.fileReadStop)
		}
	})
	w.fileReadMu.Lock()
	if !w.fileReadStopped {
		w.fileReadStopped = true
	}
	w.fileReadMu.Unlock()
	w.advisor.flush()
	// Pool, Spawn and Takeover goroutines can block in a syscall.Read, a syscall.Write (a
	// WebSocket WriteMessage), or in app logic with no pending socket call at
	// all. To unblock all three WITHOUT yet closing the fd (which would race
	// with Spawn writes), shut down both directions and cancel the conn
	// context. SHUT_RDWR unblocks a pending Read (EOF) or Write (EPIPE/ECONNRESET);
	// cancelling ctx wakes a handler blocked in app logic via Conn.Done().
	// Either way takeoverLoop exits and dispatchWG.Done fires. SHUT_* doesn't
	// free the fd number, so kernel won't recycle it — Spawn's write side
	// stays valid until we run closeFd below.
	// Cancel first and shut the sockets only after the Pool has finished, so a
	// Pool or Takeover handler that completes during shutdown still writes its
	// response.
	for _, c := range w.conns {
		if c.pending > 0 && c.cancel != nil {
			c.cancel()
		}
	}
	for _, c := range w.conns {
		c.fileStream.close()
		if c.pending > 0 && c.takenOver {
			c.takeoverGate.wake(c.takeoverFile, c.fd) // its goroutine still owns the fd, so the number is not recycled yet
		}
	}
	w.dropLingerQueue()
	// Completions are taken here while the Pool and the dispatched handlers
	// finish: a full doneCh would block a sender before its dispatchWG.Done.
	// A Takeover completion carries the fd's *os.File; this goroutine removes
	// the connection before closing it, so the raw shutdown pass below never
	// reaches a number the kernel may have reused. Other completions are
	// dropped: the stopped loop sends nothing more.
	var lingerTick <-chan time.Time
	if w.lingering > 0 {
		t := time.NewTicker(lingerPoll)
		defer t.Stop()
		lingerTick = t.C
	}
	drainUntil := func(done <-chan struct{}, grace <-chan time.Time) {
		for {
			select {
			case msg := <-w.doneCh:
				msg.stream.close()
				if msg.file != nil {
					w.removeConnBookkeeping(msg.fd)
					delete(w.conns, msg.fd)
					msg.file.Close()
				}
			case <-lingerTick:
				w.lingerAtShutdown(false)
			case <-grace:
				// A Pool handler may be waiting on a Takeover writer that is
				// blocked on its client, so the Takeover sockets cannot wait for
				// the Pool past this.
				w.shutTakeoverWrites()
				grace = nil
			case <-done:
				return
			}
		}
	}
	waitFor := func(wait func()) <-chan struct{} {
		done := make(chan struct{})
		go func() { wait(); close(done) }()
		return done
	}
	if w.pool != nil {
		close(w.pool.jobs)
		// Wait for pool goroutines to finish any in-flight job's RawSyscall
		// before we start closing fds. Otherwise a pool goroutine's write
		// could land on an fd the kernel has already recycled. This waits
		// only after the conn contexts are cancelled, so a Pool handler
		// waiting on its context, or blocked publishing a completion, can
		// return. Sockets are still open here.
		grace := time.NewTimer(shutdownTakeoverGrace)
		drainUntil(waitFor(w.pool.wg.Wait), grace.C)
		grace.Stop()
	}
	w.shutTakeoverWrites()
	// The write side alone: Spawn writes are non-blocking, and a response may
	// already be in the socket, which the read side must keep draining for.
	for _, c := range w.conns {
		if c.pending > 0 && !c.takenOver {
			c.shutWrite()
		}
	}
	// Now safe to wait for Spawn + Takeover dispatch goroutines.
	drainUntil(waitFor(w.dispatchWG.Wait), nil)
	// Every handler has returned. A connection the loop left open may still
	// have a response in its socket: it lingers too, rather than closing over
	// input that arrives behind that response.
	for _, c := range w.conns {
		if !c.takenOver && c.lingerUntil == 0 {
			c.pending = 0
			w.retire(c)
		}
	}
	w.lingerAtShutdown(true)
	// What was queued before the last sender returned.
drainBuffered:
	for {
		select {
		case msg := <-w.doneCh:
			msg.stream.close()
			if msg.file != nil {
				w.removeConnBookkeeping(msg.fd)
				delete(w.conns, msg.fd)
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
	w.wakeMu.Lock()
	w.wakeClosed = true
	w.eng.Close()
	closeFd(w.evfd)
	w.wakeMu.Unlock()
	closeFd(w.listenFd)
}

func (w *worker) close() { w.cleanup() }

// shutdownTakeoverGrace is how long Takeover handlers may keep writing while
// Shutdown waits for the Pool.
const shutdownTakeoverGrace = time.Second

// shutTakeoverWrites shuts the Takeover sockets still open, both ways: a
// handler blocked on a write, or a read whose deadline was set again after
// Shutdown's, ends now. Only cleanup calls it, the goroutine that also retires
// finished Takeovers, so no number here can belong to another socket yet.
func (w *worker) shutTakeoverWrites() {
	for _, c := range w.conns {
		if c.pending > 0 && c.takenOver {
			c.takeoverGate.shut(c.fd)
		}
	}
}

// takeoverGate orders Shutdown against a Takeover goroutine's own use of its
// socket. A nil gate (tests that drive a loop directly) does nothing.
type takeoverGate struct {
	mu       sync.Mutex
	draining bool        // in the final drain: its deadline and the peer decide the close
	woken    atomic.Bool // Shutdown has begun; a hijacked socket's reads end for good
	wrShut   bool        // the write side is shut; darwin refuses a second SHUT_WR
}

// wake ends an idle read at the start of Shutdown. A deadline, not a shutdown,
// so the read side stays open for the final drain that may follow: a handler
// still running can write its response, and the drain sets a deadline of its
// own. An app reading a hijacked socket cannot retry past it, as its reads
// report an EOF once the gate is woken.
func (g *takeoverGate) wake(f *os.File, fd int32) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.woken.Store(true)
	if !g.draining && f != nil {
		_ = f.SetReadDeadline(time.Now())
	}
}

// shut cuts the write side once the Pool has finished or the grace has passed,
// which ends a handler blocked on a write. A draining socket is left to its
// drain, which ends by itself. The read side stays, so the final drain that
// may follow still sees a late request rather than an EOF; its reads were
// woken by the deadline already.
func (g *takeoverGate) shut(fd int32) {
	if g == nil {
		_ = syscall.Shutdown(int(fd), syscall.SHUT_RDWR)
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.draining {
		g.shutWriteLocked(fd)
	}
}

// shutWriteLocked ends the write side once; the caller holds g.mu.
func (g *takeoverGate) shutWriteLocked(fd int32) {
	if !g.wrShut {
		g.wrShut = syscall.Shutdown(int(fd), syscall.SHUT_WR) == nil
	}
}

// startDrain begins the final drain: the write side ends and the read deadline
// becomes the drain's. Under the gate, so Shutdown cannot replace it.
func (g *takeoverGate) startDrain(f *os.File, fd int32) bool {
	shut := false
	if g != nil {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.draining, shut = true, g.wrShut
	}
	return (shut || syscall.Shutdown(int(fd), syscall.SHUT_WR) == nil) &&
		f.SetReadDeadline(time.Now().Add(time.Duration(lingerDelay))) == nil
}

// stopped reports whether Shutdown has ended a hijacked socket's reads.
func (g *takeoverGate) stopped() bool { return g != nil && g.woken.Load() }
