# Wing Feather System (ระบบขนนก)

Wing (ปีก) ใช้ **Feather (ขนนก)** เป็นระบบปรับจูน per-route — แต่ละ route เลือก feather ที่เหมาะกับ workload

Default ไม่ต้องใส่อะไร = ใช้ได้กับทุก workload อยู่แล้ว
Feather มีไว้สำหรับคนที่ต้องการ **บีบ throughput ให้สุด**

Implementation: [`feather.go`](feather.go) | Tests: [`feather_test.go`](feather_test.go)

---

## Type System

```go
// Feather struct — 5 axes, แต่ละ axis เป็น typed uint8 constant
type Feather struct {
    Dispatch DispatchMode  // วิธีรัน handler
    Engine   EngineMode    // kernel I/O interface
    Response ResponseMode  // วิธีส่ง response
    Buffer   BufferMode    // วิธีจัดการ memory
    Conn     ConnMode      // connection lifecycle
}

// Zero-value Feather → defaults() จะ fill เป็น Inline (fast-by-default)
```

---

## Optimization Axes (แกนปรับจูน)

Feather ไม่ใช่ 4 ตัวเลือก — มันคือ **5 แกน** ที่ compose ได้อิสระ

### Axis 1: Dispatch — `DispatchMode` (วิธีรัน handler)

| Constant | กลไก | Overhead | เหมาะกับ |
|----------|-------|----------|----------|
| `Inline` | handler ทำงานใน ioLoop ตรงๆ ไม่ข้าม goroutine | 0 | plaintext, JSON, cached, health check |
| `Pool` | dispatch ไป bounded goroutine pool แล้วรอผล | ~1μs | DB query, Redis, short external API |
| `Spawn` | สร้าง goroutine ใหม่ per-request | ~2-3μs | heavy compute, variable latency |
| `Persist` | goroutine-per-conn ที่อยู่ตลอด lifetime | goroutine cost | SSE, WebSocket, long-poll |

**เลือกยังไง:**
- Handler ไม่มี I/O wait เลย → `Inline`
- Handler รอ I/O สั้นๆ (DB, Redis) → `Pool`
- Handler รอ I/O นานไม่แน่นอน → `Spawn`
- Connection อยู่นานมาก → `Persist`

### Axis 2: Engine — `EngineMode` (kernel I/O interface)

| Constant | กลไก | เหมาะกับ |
|----------|-------|----------|
| `Epoll` | epoll_pwait / kqueue + read/write syscall ตรง | short HTTP request/response (default) |
| `Ring` | io_uring submit/complete queue | file I/O, sendfile, large body, 100K+ conns |
| `Splice` | splice(2) zero-copy fd-to-fd | reverse proxy, pipe between sockets |
| `Net` | Go standard net/http (netpoller) | HTTP/2, TLS termination, gRPC, Windows |

**เลือกยังไง:**
- HTTP API ทั่วไป → `Epoll`
- File serving / upload ใหญ่ → `Ring`
- Reverse proxy → `Splice`
- ต้องการ HTTP/2 หรือ TLS → `Net`

### Axis 3: Response — `ResponseMode` (วิธีส่ง response กลับ)

| Constant | กลไก | เหมาะกับ |
|----------|-------|----------|
| `Direct` | write() ทันทีจาก thread เดียวกับ handler | Inline dispatch (handler อยู่ใน ioLoop) |
| `Writeback` | handler → doneCh → ioLoop → write | Pool/Spawn dispatch, safe ordering |
| `DirectWrite` | handler goroutine ทำ write() เอง ไม่ผ่าน ioLoop | Pool dispatch, **ลด ~1.5μs** pipe overhead |
| `Batch` | รวมหลาย pipelined response → 1 writev() | TFB plaintext pipeline depth 16 |
| `Chunked` | Transfer-Encoding: chunked, flush ทีละ chunk | SSE, streaming response |
| `Sendfile` | sendfile(2) หรือ io_uring splice | static file serving, zero-copy |

**เลือกยังไง:**
- Inline handler → `Direct` (อัตโนมัติ)
- Pool handler ปกติ → `Writeback` (safe default)
- Pool handler ต้องการเร็วสุด → `DirectWrite` (bypass pipe wake)
- Pipelining benchmark → `Batch`
- SSE / streaming → `Chunked`
- File download → `Sendfile`

### Axis 4: Buffer — `BufferMode` (วิธีจัดการ memory)

| Constant | กลไก | เหมาะกับ |
|----------|-------|----------|
| `Fixed` | pre-allocated fixed-size buf จาก sync.Pool | plaintext, JSON, short response |
| `Grow` | เริ่มเล็ก โตตาม response size | template render, variable-size response |
| `ZeroCopy` | buildZeroCopy() detach buf ไม่ต้อง copy | Inline handler (same goroutine ไม่มี race) |
| `Stream` | write ทีละ chunk ไม่ buffer ทั้ง response | large response, SSE, file stream |
| `Registered` | io_uring registered buffers (pinned memory) | Ring engine, avoid page fault per I/O |

**เลือกยังไง:**
- Response < 4KB → `Fixed` หรือ `ZeroCopy`
- Response size ไม่แน่นอน → `Grow`
- Response ใหญ่มาก → `Stream`
- ใช้ io_uring → `Registered`

### Axis 5: Connection — `ConnMode` (วิธีจัดการ connection lifecycle)

| Constant | กลไก | เหมาะกับ |
|----------|-------|----------|
| `Pipeline` | parse request ถัดไปได้เลย ไม่ต้องรอ response | TFB benchmark, HTTP/1.1 pipelining |
| `KeepAlive` | 1 request → 1 response → รอ request ถัดไป | browser, API client ทั่วไป |
| `OneShot` | ปิด connection หลัง response | webhook, health probe, one-off call |
| `Upgrade` | 101 Switching Protocols | WebSocket, HTTP/2 upgrade |

---

## Named Presets (ชุดสำเร็จรูป)

ไม่อยากเลือกทีละ axis? ใช้ preset — ทุกตัวเป็น `var` ใน `feather.go`:

| Preset | Dispatch | Engine | Response | Buffer | Connection | ใช้กับ |
|--------|----------|--------|----------|--------|------------|--------|
| **Bolt** ⚡ | Inline | Epoll | Direct | ZeroCopy | Pipeline | TFB plaintext/JSON max throughput |
| **Flash** | Inline | Epoll | Direct | Fixed | KeepAlive | JSON API ไม่มี I/O |
| **Arrow** ← default | Pool | Epoll | DirectWrite | Fixed | KeepAlive | DB query, Redis, ทั่วไป |
| **Hawk** | Pool | Epoll | Writeback | Grow | KeepAlive | Template render, Fortunes |
| **Glide** | Persist | Epoll | Chunked | Stream | Upgrade | SSE, WebSocket |
| **Talon** | Pool | Ring | Sendfile | Registered | OneShot | File serving |
| **Soar** | Spawn | Net | Direct | Grow | KeepAlive | HTTP/2, gRPC, TLS |

---

## API

### Preset (ง่ายสุด)

```go
app.Get("/plaintext", handler, wing.Bolt)
app.Get("/json",      handler, wing.Flash)
app.Get("/db",        handler)              // Arrow = default
app.Get("/fortunes",  handler, wing.Hawk)
app.Get("/events",    handler, wing.Glide)
app.Get("/files/*",   handler, wing.Talon)
```

### Custom Compose (ปรับเอง)

```go
app.Get("/custom", handler, wing.Feather{
    Dispatch: wing.Pool,
    Engine:   wing.Epoll,
    Response: wing.DirectWrite,
    Buffer:   wing.Grow,
    Conn:     wing.KeepAlive,
})
```

### Override เฉพาะ axis ที่ต้องการ

```go
// เริ่มจาก Arrow (default) แล้วเปลี่ยนแค่ buffer
app.Get("/big-response", handler, wing.Arrow.With(wing.Buffer(wing.Grow)))

// Override หลาย axis พร้อมกัน
app.Get("/special", handler, wing.Bolt.With(
    wing.Dispatch(wing.Pool),
    wing.Buffer(wing.Fixed),
    wing.Conn(wing.KeepAlive),
))
```

### Option Constructors

| Function | Parameter | ใช้กับ `.With()` |
|----------|-----------|-----------------|
| `wing.Dispatch(m)` | `DispatchMode` | เปลี่ยนวิธี dispatch |
| `wing.Engine(m)` | `EngineMode` | เปลี่ยน kernel I/O |
| `wing.Response(m)` | `ResponseMode` | เปลี่ยนวิธีส่ง response |
| `wing.Buffer(m)` | `BufferMode` | เปลี่ยนวิธีจัดการ memory |
| `wing.Conn(m)` | `ConnMode` | เปลี่ยน connection lifecycle |

### Zero-Value Behavior

```go
// Feather{} (zero-value) → defaults() fill เป็น Inline (fast-by-default)
var f wing.Feather
f.defaults() // f == Inline+Epoll+DirectWrite+Fixed+KeepAlive

// ต้องการ Pool dispatch → ใช้ Arrow preset
wing.Arrow // Pool+Epoll+DirectWrite+Fixed+KeepAlive

// Partial init → เฉพาะ axis ที่ไม่ได้ set จะเป็น Inline defaults
wing.Feather{Dispatch: wing.Pool}
// → defaults() จะ fill Engine=Epoll, Response=DirectWrite, Buffer=Fixed, Conn=KeepAlive
```

---

## TFB Route Mapping

```go
app.Get("/plaintext", plaintextHandler, wing.Bolt)    // Inline+ZeroCopy+Pipeline
app.Get("/json",      jsonHandler,      wing.Flash)   // Inline+Fixed+KeepAlive
app.Get("/db",        dbHandler)                       // Arrow (default)
app.Get("/queries",   queriesHandler)                  // Arrow (default)
app.Get("/fortunes",  fortunesHandler,  wing.Hawk)    // Pool+Grow+Writeback
app.Get("/updates",   updatesHandler)                  // Arrow (default)
app.Get("/cached",    cachedHandler,    wing.Flash)   // Inline+Fixed+KeepAlive
```

---

## Performance Baseline (i5-13500, 8 cores)

| Preset | Workload | req/s | vs Fiber |
|--------|----------|------:|----------|
| Bolt | plaintext | ~611K | **+6%** ✓ |
| Flash | JSON | ~527K | **+12%** ✓ |
| Arrow | DB single | ~67K | -13% (WIP) |
| Hawk | Fortunes | TBD | — |
| Glide | SSE | TBD | — |
| Talon | static file | TBD | — |

### Arrow Gap Analysis (-13% vs Fiber)

Current overhead per request ใน Arrow (Pool + Writeback):
```
handler dispatch → pool channel    ~200ns
response return  → doneCh channel  ~200ns
wake pipe write  → syscall         ~500ns
wake pipe read   → syscall         ~500ns
────────────────────────────────────────
total overhead                    ~1.4μs
```

Fix: เปลี่ยนจาก `Writeback` เป็น `DirectWrite`
- handler goroutine ทำ write() ตรง ไม่ผ่าน pendingCh/pipe
- ลด overhead ~1μs per request
- ต้องจัดการ partial write + keepAlive re-arm ให้ถูก

---

## Implementation Status

| Component | Status | File |
|-----------|--------|------|
| Type definitions (5 axes, 23 constants) | ✅ Done | `feather.go` |
| Named presets (7 presets) | ✅ Done | `feather.go` |
| `.With()` + option constructors | ✅ Done | `feather.go` |
| `defaults()` zero-value → Inline | ✅ Done | `feather.go` |
| `String()` ทุก type | ✅ Done | `feather.go` |
| Unit tests | ✅ Done | `feather_test.go` |
| FeatherTable (2-level map lookup) | ✅ Done | `feather_table.go` |
| FeatherTable tests + benchmark | ✅ Done | `feather_table_test.go` |
| Wire into Config + worker | ✅ Done | `transport.go` |
| Per-route dispatch (Inline/Pool/Spawn) | ✅ Done | `transport.go` |
| DirectWrite dispatch path | ⬜ TODO | `transport.go` |
| Batch response (writev) | ⬜ TODO | — |
| Chunked response | ⬜ TODO | — |
| Sendfile response | ⬜ TODO | — |
| Persist dispatch | ⬜ TODO | — |

---

## Decision Log

| Decision | Reason |
|----------|--------|
| ไม่ auto-detect feather | เสีย overhead, ซับซ้อน — ใช้ docs + MCP tool ให้ AI แนะนำแทน |
| Default = Arrow (Pool) | ส่วนใหญ่ route ติด DB — Pool safe สำหรับทุก workload |
| Zero-value = Inline (not Arrow) | fast-by-default — ไม่สร้าง goroutine pool ถ้าไม่ต้องการ, LockOSThread ได้เลย |
| ขนนก ไม่ใช่ กระดูก | ขนนกเปลี่ยนได้ per-route, compose ได้ — กระดูกตายตัวเกิน |
| Composable axes ไม่ใช่ flat enum | ละเอียดกว่า, ผสมได้อิสระ, ไม่จำกัดจำนวน combination |
| Preset มีไว้สำหรับ DX | ไม่ต้องรู้ทุก axis ก็ใช้ได้ — เลือก preset ที่ตรง workload |
| `FeatherOption` functional option | ใช้ `.With()` override ทีละ axis ได้ ไม่ต้องเขียน struct ใหม่ทั้งก้อน |
| Typed uint8 constants | type-safe, ไม่ปนกัน, compiler จับ error ได้ |
| Zero-value → Arrow via `defaults()` | ไม่ต้อง init ครบทุก field ก็ใช้ได้ — safe default |
| `Ring` → `IOURing` rename | หลีกเลี่ยง name collision กับ `type Ring struct` ใน ring.go |
