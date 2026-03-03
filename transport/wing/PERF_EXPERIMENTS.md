# Wing Performance Experiments

Record experiment results here. Each entry: date, hypothesis, config, result, conclusion.

---

## EXP-001 — Turbo vs Normal: Can Prefork Beat Multi-Worker?

**Date:** —
**Status:** TODO

### Hypothesis

Kruda default (Workers=NumCPU, single process) already saturates all cores via LockOSThread.
Turbo (prefork, N children × 1 worker) might win if:
1. SO_REUSEPORT kernel balancing is better than shared accept fd
2. Per-process GC isolation reduces latency spikes at high load
3. L1/L2 cache isolation per process reduces cache thrash

### Setup

Machine: ___  
OS: ___  
Go version: ___  
NumCPU: ___  
wrk: `wrk -t4 -c256 -d10s`

### Variables

| ID | Config | Description |
|----|--------|-------------|
| A0 | Normal baseline | `Workers=NumCPU, GOMAXPROCS=8` |
| A1 | Turbo baseline | `Prefork=true, GOMAXPROCS=1 per child` |
| B1 | Turbo + SQPoll | `Bone{SQPoll: true}` |
| B2 | Turbo + RegisteredBuffers | `Bone{RegisteredBuffers: true}` |
| B3 | Turbo + CoopTaskrun | `Bone{CoopTaskrun: true}` |
| B4 | Turbo + all Bones | `Bone{SQPoll, RegisteredBuffers, CoopTaskrun: true}` |
| C1 | Turbo + StaticResponse | `Feather{StaticResponse: prebuiltResponse}` on /plaintext |
| C2 | Turbo + all Bones + StaticResponse | B4 + C1 |
| D1 | Normal Workers=1 | single-worker normal (baseline for G) |
| D2 | Turbo N=1 | single child (compare isolation effect only) |

### Results

| ID | /plaintext req/s | /json req/s | /db req/s | Notes |
|----|-----------------|-------------|-----------|-------|
| A0 | | | | |
| A1 | | | | |
| B1 | | | | |
| B2 | | | | |
| B3 | | | | |
| B4 | | | | |
| C1 | | | | |
| C2 | | | | |
| D1 | | | | |
| D2 | | | | |

### Conclusion

_Fill after running._

---

## EXP-002 — ElasticEvents Bone: High Connection Count

**Date:** —
**Status:** TODO (blocked on ElasticEvents implementation)

### Hypothesis

`Bone.ElasticEvents` grows the epoll event buffer dynamically when connections spike.
At low concurrency (c=256) it won't matter. At high concurrency (c=1000+) it should reduce
the number of `epoll_wait` calls needed to drain all ready events.

### Setup

Same machine as EXP-001.  
wrk variants: `c=256`, `c=512`, `c=1000`, `c=2000`

### Variables

| ID | Config |
|----|--------|
| E0 | Normal, fixed event buffer (current) |
| E1 | Normal + `Bone{ElasticEvents: true}` |
| E2 | Turbo + `Bone{ElasticEvents: true}` |

### Results

| ID | c=256 | c=512 | c=1000 | c=2000 |
|----|-------|-------|--------|--------|
| E0 | | | | |
| E1 | | | | |
| E2 | | | | |

### Conclusion

_Fill after running._

---

## EXP-003 — CPUAffinity Bone: Pin Worker to Core

**Date:** —
**Status:** TODO (blocked on CPUAffinity implementation in Bone)

### Hypothesis

Previous attempt (Experiment #2 in PERF_ROADMAP) showed -15% with naive affinity.
That was likely because all workers were pinned to the same core, or affinity was set
before `LockOSThread`. Correct approach: pin worker i to core i after `LockOSThread`.

### Variables

| ID | Config |
|----|--------|
| F0 | Normal, no affinity |
| F1 | Normal + `Bone{CPUAffinity: true}` (correct: pin after LockOSThread) |
| F2 | Turbo + `Bone{CPUAffinity: true}` |

### Results

| ID | /plaintext req/s | vs F0 |
|----|-----------------|-------|
| F0 | | — |
| F1 | | |
| F2 | | |

### Conclusion

_Fill after running._

---

## EXP-004 — Workers=1 vs Turbo N=1: Isolation Effect

**Date:** —
**Status:** TODO

### Hypothesis

If `Workers=1` (normal, single worker) and `Turbo N=1` (single child) give the same result,
then prefork overhead is zero and turbo's only advantage is GC isolation at N>1.
If turbo N=1 is faster, there's something in the process model that helps.

### Variables

| ID | Config |
|----|--------|
| G0 | `Workers=1, GOMAXPROCS=1` |
| G1 | `Prefork=true, N=1 child, GOMAXPROCS=1` |

### Results

| ID | /plaintext req/s |
|----|-----------------|
| G0 | |
| G1 | |

### Conclusion

_Fill after running._

### 2026-03-03 20:58 ICT: Kruda Turbo (Supervisor) — Fork Storm Bug

**Problem:** Kruda Supervisor fork children → child sees `KRUDA_TURBO=1` in env →
`config_wing.go` was setting `wcfg.Prefork = true` → Wing prefork forks again → infinite fork storm.

**Symptom:** Hundreds of `[kruda] JSON encoder: sonic` + `[pprof] listening on :6060` lines.
plaintext: 56K, POST JSON: 21 req/s.

**Fix applied:** Removed `wcfg.Prefork = true` from `config_wing.go` — turbo is Kruda-level,
Wing should not prefork again. Also added `IsChild()` guard to default workers to GOMAXPROCS.
Need to re-bench after fix + verify Supervisor flow is correct.

**Fiber Prefork results (same run):**

| Route | Fiber Prefork |
|-------|--------------|
| plaintext | 562K |
| param GET | 549K |
| POST JSON | 456K |

### 2026-03-03 21:20 ICT: Kruda Turbo — After Fork Storm Fix

**Changes:** Removed `wcfg.Prefork = true` from config_wing.go, skip pprof in children,
GoMaxProcs=2 per child, workers=1 per child, Processes=8.

**Bench (wrk -t4 -c256 -d5s, taskset -c 0-7):**

| Route | Kruda | Kruda Turbo | Fiber |
|-------|-------|-------------|-------|
| plaintext | **628K** | 267K | 588K |
| param GET | **559K** | 201K | 557K |

**Issues found:**
- Only 5 processes instead of 9 — children crash silently
- 2 children actually listening, rest die before bind
- GoMaxProcs=2 needed because LockOSThread + GOMAXPROCS=1 starves Go runtime (GC, sysmon)

**How Kruda vs Kruda Turbo work:**

| | Kruda (default) | Kruda Turbo |
|---|---|---|
| Processes | 1 | N (Supervisor + children) |
| Workers per process | 8 (GOMAXPROCS) | 1 |
| Total threads | 8 | N × 1 = N |
| Memory | Shared (1 process) | Separate per process |
| Connection dispatch | epoll per worker | SO_REUSEPORT (kernel) |
| GC | 1 shared GC | Isolated per process |
| Best for | CPU-bound (plaintext, JSON) | DB routes (mutex isolation) |

**Conclusion:** Turbo not faster for CPU-bound routes — same cores, more overhead.
Turbo designed for DB routes where per-process GC/mutex isolation helps.

---

### 2026-03-03 22:20 ICT: HandlerPoolSize Tuning — DB Routes

**Hypothesis:** Default HandlerPoolSize=256/worker (2048 total) causes Go runtime scheduler
contention when goroutines >> DB pool_max_conns. Reducing pool size should reduce contention.

**Config:** i5-13500 8P cores, GOMAXPROCS=8, KRUDA_WORKERS=8, pool_max_conns=64, wrk -t4 -c256 -d5s

**Pool size sweep (/db route):**

| pool/worker | total goroutines | db req/s |
|:-----------:|:----------------:|:--------:|
| 4 | 32 | 90,405 |
| 6 | 48 | 91,460 |
| 8 | 64 | 90,906 |
| 10 | 80 | 81,243 |
| 12 | 96 | 41,026 |
| 256 (old default) | 2048 | 56,741 |

**Full DB bench — Kruda (pool=workers) vs Fiber:**

| Route | Kruda | Fiber | vs Fiber |
|-------|------:|------:|----------|
| db | 91,864 | 86,360 | ✅ +6% |
| queries?q=20 | 21,441 | 20,242 | ✅ +6% |
| fortunes | 80,675 | 78,983 | ✅ +2% |
| updates?q=20 | 69 | 104 | ❌ -34% |

**Root cause:** 2048 pool goroutines fighting for 64 DB connections → `runtime.lock2` + `procyield`
consumed ~22% CPU in pprof. Reducing to 64 total (= pool_max_conns) eliminated contention.

**Action:** Changed default `HandlerPoolSize` from 256 to `Workers` (= GOMAXPROCS by default).
Users can override via `KRUDA_POOL_SIZE` env var or `wing.Config{HandlerPoolSize: N}`.

---

### 2026-03-03 22:47 ICT: Full Bench — Kruda vs Fiber (all 8 routes)

**Config:** i5-13500 8P cores, taskset 0-7, GOMAXPROCS=8, KRUDA_WORKERS=8,
HandlerPoolSize=workers (new default), pool_max_conns=64, wrk -t4 -c256 -d5s

| Route | Kruda | Fiber | vs Fiber |
|-------|------:|------:|----------|
| plaintext | 612,688 | 603,483 | ✅ +2% |
| param GET | 586,944 | 531,438 | ✅ +10% |
| POST JSON | 549,817 | 484,825 | ✅ +13% |
| JSON GET | 558,747 | 481,708 | ✅ +16% |
| db | 91,848 | 86,763 | ✅ +6% |
| queries?q=20 | 21,599 | 20,439 | ✅ +6% |
| fortunes | 80,021 | 79,866 | ✅ +0.2% |
| updates?q=20 | 154 | 87 | ✅ +77% |

**Result: Kruda wins 8/8 routes.**

Notes:
- updates variance is high (DB write bottleneck, both ~80-150 req/s) — not framework-bound
- DB routes improved dramatically after HandlerPoolSize fix (was -37% on db, now +6%)
- Fiber runs single-process normal mode (no prefork) — fair comparison

---

### 2026-03-03 23:00 ICT: Further Optimization Attempts — Diminishing Returns

After winning all 8 routes vs Fiber, tried to push further. All experiments below
showed no meaningful improvement — confirming we've hit kernel/DB/hardware ceiling.

**CPU-bound routes (plaintext 612-628K):**
Profile: 68% syscall (epoll+read+write), 9% procyield, 3% parser, 2% handler.

| Experiment | plaintext | Result |
|-----------|----------:|--------|
| GOMAXPROCS=12 WORKERS=8 | 628K | ≈ same (within variance) |
| GOGC=off | 460K | ❌ worse (cache pollution, sync.Pool broken) |
| KRUDA_STATIC=1 (bypass handler) | 592-638K | ≈ same (handler only 2.3% of CPU) |

**DB routes (db ~92K, fortunes ~80K):**

| Experiment | db | Result |
|-----------|---:|--------|
| pool_max_conns=64 (baseline) | 92K | ✅ best |
| pool_max_conns=128 | 91K | ≈ same |
| pool_max_conns=256 | 90K | ≈ same |
| pool_min_conns=64 (pre-warm) | 88K | ≈ same |
| GOGC=off | 87K | ≈ same |
| tcp_slow_start_after_idle=0 | 91K | ≈ same |
| Spawn dispatch (vs Pool) | 55K | ❌ worse |

**Feather/Bone implementation audit:**
- Dispatch (Inline/Pool/Spawn): ✅ implemented
- StaticResponse bypass: ✅ implemented
- DirectWrite from Pool goroutine: ✅ implemented
- Pipeline (tryParse loop + speculative read): ✅ hardcoded for Inline
- BatchWrite: defined but behavior already hardcoded (Inline batches naturally)
- Response/Buffer/Conn axes: defined but not read — future design
- SkipTimestamp: defined but timeout guard (`if w.hasTimeout`) already handles it

**Conclusion:** Framework overhead is <5% of total CPU. Remaining 95% is kernel syscalls
(68%) + Go runtime scheduler (22%) + DB round-trip. No framework-level optimization
will yield measurable gains. Next perf leap requires io_uring or custom runtime — not worth it.
