# Wing Performance Roadmap

## Current Status (2026-03-03 18:27 ICT)

**Wing beats Fiber on all 3 CPU-bound routes in normal mode (through framework layer).**

| Route | Wing | Fiber | vs Fiber |
|-------|------|-------|----------|
| plaintext | **634K** | 595K | ✅ +7% |
| param GET | **558K** | 543K | ✅ +3% |
| POST JSON | **549K** | 486K | ✅ +13% |

Bench: `wrk -t4 -c256 -d5s`, GOMAXPROCS=8, KRUDA_WORKERS=8, Linux i5-13500.
All routes go through kruda framework layer (router → middleware → handler).

---

## Architecture: Wing + Bone + Feather

```
Wing (ปีก)        = request-type profile (Plaintext, ParamJSON, Query...)
├── Bone (กระดูก)  = engine-level I/O optimization (affects all connections)
└── Feather (ขนนก) = per-route optimization (dispatch, buffer, response mode)
```

See `docs/features/wing-feather-redesign.md` for full design.

---

## What's Done

| Technique | Source | Status |
|-----------|--------|--------|
| Custom HTTP parser | silverlining | ✅ |
| Method intern O(1) | silverlining | ✅ (switch by len) |
| Pre-computed Date+Server header | silverlining | ✅ (atomic.Pointer, Tick(1s)) |
| Pre-computed status lines | silverlining | ✅ (statusLines[600]) |
| sync.Pool (request, response) | silverlining | ✅ |
| Goroutine pool (Pool dispatch) | silverlining | ✅ (workerPool) |
| Event-driven epoll/kqueue | gnet | ✅ |
| Edge-triggered (EPOLLET) | gnet | ✅ |
| SO_REUSEPORT | gnet | ✅ |
| LockOSThread | gnet | ✅ |
| Multiple event loops per core | gnet | ✅ (Workers=NumCPU) |
| Adaptive epoll_wait | gnet | ✅ (idle counter) |
| CAS-gated pipe wakeup | gnet | ✅ |
| Pointer-in-epoll data | gnet | ✅ (ConnPtr) |
| RawSyscall for epoll_wait | gnet | ✅ (RawMode) |
| Prefork / SO_REUSEPORT | fasthttp | ❌ removed (doesn't help Wing) |
| Object pooling | fasthttp | ✅ |
| Zero-copy headers (byte offsets) | fasthttp | ✅ |
| Per-connection buffer reuse | fasthttp | ✅ |
| Direct write + speculative read | actix/gnet | ✅ (directSend) |
| Static response shortcut | actix | ✅ (StaticResponse) |
| JSON fast path (SetJSON) | — | ✅ (JSONResponder) |
| Wing type per-route API | — | ✅ (WingPlaintext, WingParamJSON...) |
| hasTimeout guard (skip time.Now) | — | ✅ (Bone: SkipTimestamp) |
| pgx raw driver + prepared stmts | TFB | ✅ (bench app) |
| Batch updates (unnest) | TFB | ✅ (bench app) |

---

## What's NOT Done

| # | Technique | Impact | Effort | Source |
|---|-----------|--------|--------|--------|
| 1 | Soft/Hard reset pattern | +1-2% | easy | silverlining |
| 2 | Ring buffer (zero-alloc I/O) | +3-5% | hard | gnet |
| 3 | XOR method hash table | +1-2% | trivial | silverlining |
| 4 | writev scatter-gather | +2-3% | easy | gnet |
| 5 | Elastic event list | +2-5% high conn | easy | gnet |
| 6 | CPU affinity per worker | +5-10% | easy | ntex |
| 7 | Work-stealing goroutine pool | +3-5% | hard | silverlining |
| 8 | LRU route cache | +2-3% many routes | medium | gearbox |
| 9 | Disable default headers option | +1-2% | trivial | gearbox |
| 10 | EventfdWake (Go netpoller) | +10-20% | hard | — |
| 11 | io_uring engine | +15-25% | very hard | — |

---

## Priority: What to Do Next

### Tier 1 — Low-hanging fruit (ทำได้เลย, รวม +5-10%)

| # | Task | Files | Impact |
|---|------|-------|--------|
| 3 | XOR method hash | `http.go` | +1-2% |
| 9 | NoDefaultHeaders option | `bone.go`, `http.go` | +1-2% |
| 5 | Elastic event list | `engine_linux.go` | +2-5% |
| 1 | Soft/Hard reset | `http.go` | +1-2% |

### Tier 2 — ~~Medium effort~~ ❌ Won't help

| # | Task | Why skip |
|---|------|----------|
| ~~6~~ | ~~CPU affinity per worker~~ | Experiment #2: -15%, reverted |
| ~~4~~ | ~~writev scatter-gather~~ | Wing builds single buffer (buildZeroCopy), writev needs separate bufs |
| ~~8~~ | ~~LRU route cache~~ | Router not in pprof top 20, not a bottleneck |

### Tier 3 — Architecture change

| # | Task | Status | Note |
|---|------|--------|------|
| ~~11~~ | ~~io_uring engine~~ | ❌ Skip | No Go HTTP framework proves io_uring > epoll for HTTP bench; high complexity |
| ~~7~~ | ~~Work-stealing pool~~ | ❌ Skip | Go scheduler already does work-stealing |
| 10 | EventfdWake — Go netpoller | 💤 Someday | gnet + silverlining prove approach works; engine rewrite, not urgent |

Note: silverlining (28M RPS TFB) uses Go netpoller, not custom epoll.
io_uring may not help keep-alive HTTP bench but needs real measurement to confirm.

---

## Experiment: Turbo vs Normal — How to Make Turbo Win

### Hypothesis

Turbo (prefork) currently loses because Wing's multi-worker epoll already saturates all cores.
Turbo can only win if it eliminates something Normal can't: cross-process GC isolation or kernel-level SO_REUSEPORT balancing at very high connection counts.

### Variables to Test

| Variable | Normal baseline | Turbo variant | Bone/Feather |
|----------|----------------|---------------|--------------|
| A | Workers=NumCPU, GOMAXPROCS=8 | N children × 1 worker, GOMAXPROCS=1 | — |
| B | same | + `Bone.SQPoll` | Bone |
| C | same | + `Bone.RegisteredBuffers` | Bone |
| D | same | + `Bone.CoopTaskrun` | Bone |
| E | same | + `Bone.ElasticEvents` (when done) | Bone |
| F | same | + `Feather.StaticResponse` on plaintext | Feather |
| G | Workers=1 (single-worker normal) | N children × 1 worker | compare isolation |

### Expected Outcome

Turbo likely wins only on G (single-worker vs multi-process at same core count) if SO_REUSEPORT kernel balancing is better than Wing's shared accept. All other variables unlikely to flip the result based on current data.

### How to Run

```bash
# Bench
GOMAXPROCS=8 KRUDA_WORKERS=8 wrk -t4 -c256 -d10s http://localhost:3000/plaintext
```

Record results in `PERF_EXPERIMENTS.md`.

---

## DB Routes (separate concern)

DB routes (db, fortunes, updates) have different bottleneck: pgx pool contention.

| Route | Wing (w=1) | Wing (w=8) | Fiber |
|-------|-----------|-----------|-------|
| db | **78K** | 54K | 76K |
| fortunes | **84K** | 49K | 79K |
| updates=20 | 109 | **240** | 165 |

Key finding: workers=1 beats Fiber for db/fortunes (zero pgx mutex contention).
TFB benches 1 route at a time → use optimal worker count per route.

---

## Prefork Results

Kruda prefork (turbo) hurts performance — Wing's epoll per-worker already parallelizes.

| Route | Kruda | Kruda Turbo (prefork) | Fiber | Fiber Prefork |
|-------|-----------|-------------|-------------|--------------|
| plaintext | **634K** | 477K | 595K | 588K |
| param GET | **558K** | 403K | 543K | 573K |
| POST JSON | **549K** | 365K | 486K | 528K |

Conclusion: Don't use turbo (prefork) with Wing transport. Fiber benefits from prefork, Wing doesn't.
