# Wing v1.5 Hybrid Rewrite — Implementation Plan

## Summary

Rewrite transport.go from v2 (goroutine-per-conn + 6 channel ops) to v1.5 hybrid (inline parse in ioLoop + bounded goroutine pool). Target: 1 channel op, 1 pipe write, 2 context switches per request.

---

## File Changes

### 1. `transport.go` — MAJOR REWRITE

#### DELETE these types/fields:
```go
// Remove entirely:
type connState struct { ... }      // line 125-129
type sqeKind uint8                 // line 131
const sqeRecv/sqeSend/sqeClose     // line 133-137
type sqeReq struct { ... }         // line 139-144

// Remove from worker struct:
conns    map[int32]*connState      // replace with map[int32]*conn
connsMu  sync.Mutex               // no longer needed (ioLoop owns conns)
acceptCh chan int32                // replaced by inline accept
sqCh     chan sqeReq               // replaced by handlerCh/pendingCh
```

#### DELETE these functions:
```go
func (w *worker) acceptLoop()           // line 359-367
func (w *worker) serveConn(fd int32)    // line 370-457
func (w *worker) dispatchRecv(...)      // line 332-343
func (w *worker) dispatchSend(...)      // line 345-356
```

#### ADD new types:
```go
type conn struct {
    fd        int32
    gen       uint32  // generation — detect fd recycling
    readBuf   []byte
    readN     int
    sendBuf   []byte  // non-nil = send in progress
    sendN     int     // bytes already sent
    keepAlive bool
}

type handlerReq struct {
    fd  int32
    gen uint32
    req *wingRequest
}

type pendingResp struct {
    fd        int32
    gen       uint32
    data      []byte
    keepAlive bool
}

// Pre-built 503 response
var resp503 = []byte("HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n")
```

#### ADD to Config:
```go
HandlerPoolSize int // handler goroutine pool per worker (0 = Workers count)
```

#### ADD to defaults():
```go
if c.HandlerPoolSize <= 0 {
    c.HandlerPoolSize = c.Workers
}
```

#### REWRITE worker struct:
```go
type worker struct {
    id        int
    listenFd  int
    eng       engine
    handler   transport.Handler
    config    Config
    limits    parserLimits
    maxConns  int
    nextGen   uint32                    // NEW: generation counter
    conns     map[int32]*conn           // CHANGED: *conn not *connState
    events    [maxEventsPerWait]event
    pipeR     int
    pipeW     int
    pipeBuf   [8]byte
    handlerCh chan handlerReq            // NEW: ioLoop → pool
    pendingCh chan pendingResp           // NEW: pool → ioLoop
    shutdown  *atomic.Bool
}
```

#### REWRITE newWorker:
- Remove `acceptCh`, `sqCh`, `connsMu`
- Add `handlerCh: make(chan handlerReq, cfg.HandlerPoolSize)`
- Add `pendingCh: make(chan pendingResp, cfg.HandlerPoolSize)`
- `conns: make(map[int32]*conn, 1024)`

#### REWRITE run():
```go
func (w *worker) run(shutdown *atomic.Bool) {
    w.shutdown = shutdown
    // Start handler pool goroutines
    for i := 0; i < w.config.HandlerPoolSize; i++ {
        go w.handlerWorker()
    }
    w.ioLoop(shutdown)
}
```

#### REWRITE ioLoop():
```go
func (w *worker) ioLoop(shutdown *atomic.Bool) {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    w.eng.SubmitAccept(w.listenFd)
    w.eng.SubmitPipeRecv(w.pipeR, w.pipeBuf[:])
    w.eng.Flush()

    for !shutdown.Load() {
        n, err := w.eng.Wait(w.events[:])
        if err != nil {
            if shutdown.Load() { break }
            continue
        }
        for i := 0; i < n; i++ {
            w.handleEvent(w.events[i])
        }
        w.eng.Flush()
    }
    w.cleanup()
}
```
**Key change**: No more sqCh drain loop. Events handled inline.

#### NEW handleEvent():
```go
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
        w.drainPending()
        w.eng.SubmitPipeRecv(w.pipeR, w.pipeBuf[:])
    }
}
```

#### REWRITE handleAccept():
```go
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
        return  // BUG 3 FIX: no duplicate SubmitAccept
    }

    setTCPNodelay(fd)
    w.nextGen++
    c := &conn{
        fd:      fd,
        gen:     w.nextGen,
        readBuf: make([]byte, w.config.ReadBufSize),
    }
    w.conns[fd] = c
    w.eng.SubmitRecv(fd, c.readBuf, 0)
}
```

#### NEW handleRecv():
```go
func (w *worker) handleRecv(ev event) {
    c := w.conns[ev.Fd]
    if c == nil { return }

    if ev.Res <= 0 {
        w.closeConn(ev.Fd)
        return
    }
    c.readN += int(ev.Res)
    w.tryParse(c)
}
```

#### NEW tryParse():
```go
func (w *worker) tryParse(c *conn) {
    req, consumed, ok := parseHTTPRequest(c.readBuf[:c.readN], w.limits)
    if !ok {
        if c.readN >= len(c.readBuf) {
            w.closeConn(c.fd)  // buffer full, can't parse
            return
        }
        w.eng.SubmitRecv(c.fd, c.readBuf, c.readN)
        return
    }

    // Shift remaining data
    remaining := c.readN - consumed
    if remaining > 0 {
        copy(c.readBuf, c.readBuf[consumed:c.readN])
    }
    c.readN = remaining

    // Non-blocking dispatch to handler pool
    select {
    case w.handlerCh <- handlerReq{fd: c.fd, gen: c.gen, req: req}:
        // dispatched — wait for pendingResp via Wake
    default:
        // pool full → 503 directly from ioLoop
        w.send503(c)
    }
}
```

#### NEW send503():
```go
func (w *worker) send503(c *conn) {
    c.sendBuf = resp503
    c.sendN = 0
    c.keepAlive = false  // close after 503
    w.eng.SubmitSend(c.fd, resp503)
}
```

#### NEW handlerWorker():
```go
func (w *worker) handlerWorker() {
    for hr := range w.handlerCh {
        resp := acquireResponse()
        func() {
            defer func() {
                if r := recover(); r != nil {
                    resp.WriteHeader(500)
                    resp.Write([]byte("Internal Server Error\n"))
                }
            }()
            w.handler.ServeKruda(resp, hr.req)
        }()

        data := resp.build()  // SAFE COPY — cross-goroutine
        keepAlive := hr.req.keepAlive
        releaseResponse(resp)

        w.pendingCh <- pendingResp{
            fd:        hr.fd,
            gen:       hr.gen,
            data:      data,
            keepAlive: keepAlive,
        }
        w.eng.PostWake()
    }
}
```

#### NEW drainPending():
```go
func (w *worker) drainPending() {
    for {
        select {
        case pr := <-w.pendingCh:
            c := w.conns[pr.fd]
            if c == nil || c.gen != pr.gen {
                continue  // stale — fd recycled
            }
            c.sendBuf = pr.data
            c.sendN = 0
            c.keepAlive = pr.keepAlive
            w.eng.SubmitSend(pr.fd, pr.data)
        default:
            return
        }
    }
}
```

#### NEW handleSend():
```go
func (w *worker) handleSend(ev event) {
    c := w.conns[ev.Fd]
    if c == nil { return }

    if ev.Res < 0 {
        w.closeConn(ev.Fd)
        return
    }

    c.sendN += int(ev.Res)
    if c.sendN < len(c.sendBuf) {
        // Partial send — continue
        w.eng.SubmitSend(ev.Fd, c.sendBuf[c.sendN:])
        return
    }

    // Send complete
    c.sendBuf = nil
    c.sendN = 0

    if !c.keepAlive {
        w.closeConn(ev.Fd)
        return
    }

    // Try parse pipelined data
    if c.readN > 0 {
        w.tryParse(c)
    } else {
        w.eng.SubmitRecv(ev.Fd, c.readBuf, c.readN)
    }
}
```

#### REWRITE closeConn():
```go
func (w *worker) closeConn(fd int32) {
    delete(w.conns, fd)
    w.eng.SubmitClose(fd)
}
```

#### REWRITE cleanup():
```go
func (w *worker) cleanup() {
    close(w.handlerCh)  // stops handler goroutines
    for fd := range w.conns {
        closeFd(int(fd))
    }
    w.eng.Close()
    closeFd(w.pipeR)
    closeFd(w.pipeW)
    closeFd(w.listenFd)
}
```

---

### 2. `http.go` — Minor cleanup

#### DELETE:
- `statusText()` function (line 467-508) — dead code, replaced by `statusLines[]`
- Old `conn` struct (line 166-172) — moved to transport.go
- Old `pendingResp` struct (line 174-178) — moved to transport.go

---

### 3. `connlimit_test.go` — Update test helpers

#### REWRITE newTestWorker():
```go
func newTestWorker(maxConns int) (*worker, *mockEngine) {
    eng := &mockEngine{}
    w := &worker{
        id:        0,
        listenFd:  100,
        eng:       eng,
        config:    Config{ReadBufSize: 4096, HandlerPoolSize: 256},
        maxConns:  maxConns,
        conns:     make(map[int32]*conn, 16),
        handlerCh: make(chan handlerReq, 256),
        pendingCh: make(chan pendingResp, 256),
    }
    return w, eng
}
```

#### UPDATE all test functions:
- Replace `*connState` with `*conn`
- Replace `&connState{recvCh: ..., sendCh: ...}` with `&conn{fd: N, gen: 1, readBuf: make([]byte, 4096)}`
- Remove `connsMu.Lock()/Unlock()` (conns owned by ioLoop, no mutex)
- Remove `acceptCh` reads
- Tests that check conns map can access directly (no mutex)

#### UN-SKIP unit tests:
```go
// These can now test inline handlers:
TestHandleRecv_CloseOnEOF
TestHandleRecv_CloseOnNegativeResult
TestHandleSend_CloseOnError
TestHandleSend_CloseOnConnectionClose
```

#### FIX TestHandleAccept_ConnectionLimitReached:
- Remove `eng.acceptArmed != 1` check (Bug 3 fix: no duplicate SubmitAccept when at limit with cqeFMore)

---

### 4. `config_wing.go` — Pass HandlerPoolSize

```go
return wing.New(wing.Config{
    Workers:         workers,
    HandlerPoolSize: handlerPoolSize,  // from env WING_HANDLER_POOL
})
```

---

## Implementation Order

1. Config: add `HandlerPoolSize` + defaults
2. Structs: `conn`, `handlerReq`, `pendingResp`, `resp503`
3. `newWorker` — new channels, remove old
4. `run` — handler pool, remove acceptLoop
5. `ioLoop` — simple Wait loop
6. Event handlers: `handleEvent`, `handleAccept`, `handleRecv`, `tryParse`, `send503`
7. `handlerWorker` + `drainPending`
8. `handleSend` (partial send + pipelining)
9. `closeConn` + `cleanup`
10. Delete dead code from transport.go + http.go
11. Update connlimit_test.go

---

## Verification

```bash
# 1. Build
go build ./transport/wing/

# 2. Unit tests
go test ./transport/wing/ -run TestHandle -v

# 3. Integration tests
go test ./transport/wing/ -run TestTransport -v
go test ./transport/wing/ -run TestIntegration -v

# 4. Fuzz
go test ./transport/wing/ -fuzz FuzzParseHTTPRequest -fuzztime 10s

# 5. Race detector
go test ./transport/wing/ -race -count 3

# 6. Pipelining
go test ./transport/wing/ -run TestIntegration_.*Pipeline -v

# 7. Benchmark
go test ./transport/wing/ -bench . -benchmem
```

---

## Critical Safety Notes

- **`build()` not `buildZeroCopy()`** — handler goroutine builds, ioLoop sends = cross-goroutine → must safe copy
- **Generation counter** — prevents stale pendingResp from corrupting recycled fd
- **Non-blocking handlerCh send** — prevents ioLoop blocking when pool full
- **No mutex on conns map** — single owner (ioLoop goroutine) only
- **Pipelining order** — 1 request at a time per conn (tryParse only after handleSend completes)
