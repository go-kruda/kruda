# Wing + Feather Redesign

## Concept

**Wing** (ปีก) = request-type profile — ปีกแต่ละแบบออกแบบมาสำหรับการบินแบบต่างกัน
**Feather** (ขนนก) = individual optimization technique — ชิ้นส่วนเล็กๆ ที่ประกอบกันเป็น Wing

User เลือก Wing type ตาม request → kruda ประกอบ Feather set ที่ให้ perf ดีสุดให้อัตโนมัติ

## Current State

```go
// Feather = 5-axis config struct (ชื่อ misleading — มันคือ Wing ไม่ใช่ Feather)
type Feather struct {
    Dispatch, Engine, Response, Buffer, Conn
    StaticResponse []byte
}

// Presets ตั้งชื่อตาม bird traits (ไม่สื่อ request type)
Bolt, Flash, Arrow, Hawk, Glide, Talon, Soar

// User config ผ่าน env vars — ไม่ type-safe, ไม่ discoverable
WING_POOL_ROUTES="GET /db,GET /queries"
WING_STATIC=1
```

## Anatomy

```
Wing (ปีก)        = request-type profile — เลือกตาม request type
├── Bone (กระดูก)   = I/O engine structure — syscall strategy, kernel interface
│                    มีผลทั้ง Wing, ไม่เปลี่ยนตาม route
└── Feather (ขนนก)  = per-route optimization — dispatch, buffer, response mode
                     เบา, เปลี่ยนได้ตาม route
```

### Bone Catalog (engine-level optimizations)

| Bone | What it does | Impact |
|------|-------------|--------|
| `BatchWrite` | Coalesce pipelined responses → 1 write syscall per batch | ลด write syscall 5-16x |
| `SQPoll` | Kernel thread poll SQ → zero io_uring_enter for submit | ลด submit syscall เป็น 0 |
| `RegisteredBuf` | Pinned memory buffers → ลด page table copy per I/O | ลด read/write overhead |
| `DirectSyscall` | Raw syscall แทน Go wrapper → ลด cgo overhead | ~5ns per syscall |
| `EventfdWake` | eventfd + Go netpoller แทน LockOSThread + pipe | ลบ LockOSThread penalty |
| `CoopTaskrun` | IORING_SETUP_COOP_TASKRUN → ลด IPI interrupt | ลด kernel overhead |

Bone ต่างจาก Feather:
- Feather เปลี่ยนตาม route (`/users/:id` ใช้ Inline, `/db` ใช้ Pool)
- Bone มีผลทั้ง engine (เปิด BatchWrite = ทุก connection ได้ประโยชน์)
- Feather เบา ไม่มี risk, Bone หนัก ต้อง test ระวัง

## Wing Types (user-facing)

| Wing | Request Type | Example Routes |
|------|-------------|----------------|
| `Plaintext` | Static text, health check | `GET /`, `GET /health` |
| `JSON` | JSON response, no param | `GET /json` |
| `ParamJSON` | JSON + route param extraction | `GET /users/:id` |
| `PostJSON` | POST body parse + JSON response | `POST /json`, `POST /users` |
| `Query` | DB/Redis short I/O | `GET /db`, `GET /queries` |
| `Render` | Template/HTML rendering | `GET /fortunes` |
| `Stream` | SSE, WebSocket, long-poll | `GET /events` |

### Feather Catalog (composable optimizations)

Each Feather is a concrete optimization with real code path:

| Feather | What it does | Used by |
|---------|-------------|---------|
| `InlineDispatch` | Run handler in ioLoop, zero goroutine | Plaintext, JSON, ParamJSON, PostJSON |
| `PoolDispatch` | Dispatch to goroutine pool | Query, Render |
| `PersistDispatch` | Keep goroutine alive per conn | Stream |
| `ZeroCopyBuf` | No memcpy on response build | Plaintext, JSON, ParamJSON, PostJSON |
| `FixedBuf` | Pre-allocated buffer from pool | Query, Render |
| `StreamBuf` | Incremental write, no full buffer | Stream |
| `DirectResponse` | Write response in same goroutine | Plaintext, JSON, ParamJSON, PostJSON |
| `DirectWrite` | Handler goroutine calls write() directly | Query |
| `Writeback` | Response via channel to ioLoop | Render |
| `ChunkedResponse` | Transfer-Encoding: chunked | Stream |
| `PipelineConn` | Parse next request before response sent | Plaintext |
| `KeepAliveConn` | One request at a time | JSON, ParamJSON, PostJSON, Query, Render |
| `StaticResponse` | Pre-built response bytes, bypass handler | (opt-in per route) |
| `JSONFastPath` | Skip generic header build for JSON | JSON, ParamJSON, PostJSON |
| `SkipTimeout` | Skip time.Now() per request | all (when no timeout configured) |

### Wing = Feather Composition

```
Plaintext  = InlineDispatch + ZeroCopyBuf + DirectResponse + PipelineConn + SkipTimeout
JSON       = InlineDispatch + ZeroCopyBuf + DirectResponse + JSONFastPath + KeepAliveConn + SkipTimeout
ParamJSON  = InlineDispatch + ZeroCopyBuf + DirectResponse + JSONFastPath + KeepAliveConn + SkipTimeout
PostJSON   = InlineDispatch + ZeroCopyBuf + DirectResponse + JSONFastPath + KeepAliveConn + SkipTimeout
Query      = PoolDispatch   + FixedBuf    + DirectWrite    + KeepAliveConn
Render     = PoolDispatch   + FixedBuf    + Writeback      + KeepAliveConn
Stream     = PersistDispatch + StreamBuf  + ChunkedResponse + KeepAliveConn
```

## User API

### Per-route Wing type (primary API)

```go
app := kruda.New(kruda.Wing())

app.Get("/", handler, wing.Plaintext)
app.Get("/users/:id", handler, wing.ParamJSON)
app.Post("/json", handler, wing.PostJSON)
app.Get("/db", handler, wing.Query)
app.Get("/fortunes", handler, wing.Render)
```

### Custom Feather composition (advanced)

```go
// Start from a Wing, swap individual Feathers
app.Get("/cached", handler, wing.JSON.With(wing.PipelineConn))
app.Get("/heavy-db", handler, wing.Query.With(wing.SpawnDispatch))
```

### Static response opt-in

```go
app.Get("/health", nil, wing.Plaintext.Static("OK"))
```

### Bone Config (engine-level)

```go
wing.New(wing.Config{
    Bone: wing.Bone{
        BatchWrite:    true,  // coalesce pipelined responses
        SkipTimestamp: true,  // skip time.Now() when no timeout
    },
})
```

Env var: `KRUDA_BATCH_WRITE=1`

### Handler Pool Size

Pool dispatch routes (Query, Render) use a goroutine pool per worker.
Default: `Workers` count (e.g. 8 workers → 8 goroutines/worker → 64 total).

Tune via env var or config:

```go
// Env var
KRUDA_POOL_SIZE=16  // per worker

// Go config
kruda.New(kruda.Wing(wing.Config{HandlerPoolSize: 16}))
```

Rule of thumb: keep total goroutines (workers × pool_size) ≤ DB pool_max_conns.
Too many goroutines cause Go runtime scheduler contention.

### Bone preset

```go
// Skeleton = common bench optimizations
wing.New(wing.Config{
    Bone: wing.Skeleton,
})
```

## Implementation Tasks

### Task 1: Rename Feather → Wing type (internal)

**Files:** `transport/wing/feather.go`, `transport/wing/feather_table.go`

- Rename `Feather` struct → keep as internal config (or rename to `WingConfig`)
- Rename presets: `Bolt` → `Plaintext`, `Flash` → `JSON`, `Arrow` → `Query`, etc.
- Keep `FeatherTable` → `WingTable` (or keep name, just change preset names)
- Backward compat: type alias old names

### Task 2: Expose Wing types through kruda package

**Files:** `config_wing.go`, `route.go` (or new `wing_api.go`)

- Add optional `...RouteOption` param to `app.Get()`, `app.Post()`, etc.
- `RouteOption` wraps `wing.Feather` so user doesn't import `transport/wing` directly
- Auto-register in FeatherTable when route has Wing type specified

### Task 3: Default Wing auto-detection (optional, lower priority)

**Files:** `route.go`

- If user doesn't specify Wing type, infer from handler signature:
  - No body parse → `JSON` or `Plaintext`
  - Has `c.Bind()` → `PostJSON`
  - Has `c.Param()` → `ParamJSON`
- This is hard to do statically — may skip for v1

### Task 4: Update bench app

**Files:** `bench/cross-runtime/kruda/main.go`

```go
app.Get("/", handler, wing.Plaintext)
app.Get("/users/:id", handler, wing.ParamJSON)
app.Post("/json", handler, wing.PostJSON)
app.Get("/db", handler, wing.Query)
app.Get("/queries", handler, wing.Query)
app.Get("/fortunes", handler, wing.Render)
app.Get("/updates", handler, wing.Query)
```

### Task 5: Benchmark & validate

- Run cross-runtime bench with new Wing types
- Target: beat Fiber on all 3 routes (plaintext, param GET, POST JSON)
- Current gap: param GET -6% vs Fiber (508K vs 541K)

## Mapping: Old Presets → New Wing Types

| Old Name | New Name | Notes |
|----------|----------|-------|
| `Bolt` | `Plaintext` | Pipeline + ZeroCopy + Inline |
| `Flash` | `JSON` | KeepAlive + Fixed + Inline |
| `Arrow` | `Query` | Pool + DirectWrite |
| `Hawk` | `Render` | Pool + Writeback + Grow |
| `Glide` | `Stream` | Persist + Chunked |
| `Talon` | (remove) | io_uring not implemented yet |
| `Soar` | (remove) | net/http fallback, not Wing |

## Non-Goals

- TLS/WebSocket (reverted, out of scope)
- io_uring engine (future)
- Auto-detection of Wing type from handler code (maybe v2)

## TODO

- [ ] Update `frameworks/Go/kruda/src/` with latest Wing types + Bone config for TFB submission
- [ ] Sync bench app routes with TFB required tests (db, queries, fortunes, updates, cached-queries)

## Success Criteria

1. User API is `app.Get("/path", handler, wing.ParamJSON)` — one extra arg
2. All 3 bench routes beat Fiber through framework layer
3. Features that cost perf are opt-in (user must specify Wing type)
4. Default (no Wing specified) = safe `JSON` preset (Inline + KeepAlive)
