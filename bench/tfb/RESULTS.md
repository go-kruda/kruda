# TFB Benchmark Results — Kruda vs Fiber

## 2026-02 Baseline (macOS, single-process, GOGC=off)

Config: `wrk -t4 -c128 -d10s`, 5 rounds per test, median reported.

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 217,449 | 212,488 | +2.3% | Kruda |
| plaintext | 207,296 | 214,114 | -3.2% | Fiber |
| db | 25,237 | 19,629 | +28.6% | Kruda |
| queries | 7,677 | 6,219 | +23.4% | Kruda |
| fortunes | 31,188 | 23,939 | +30.3% | Kruda |
| cached | 217,768 | 175,619 | +24.0% | Kruda |
| updates | 2,120 | 1,999 | +6.1% | Kruda |

Score: Kruda 6 — Fiber 1

---

## 2026-02 Best Run — 7/7 Sweep 🏆

Config: same as above.

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 221,471 | 214,126 | +3.4% | Kruda |
| plaintext | 216,269 | 215,630 | +0.3% | Kruda |
| db | 25,774 | 20,152 | +27.9% | Kruda |
| queries | 7,720 | 6,295 | +22.6% | Kruda |
| fortunes | 32,077 | 24,425 | +31.3% | Kruda |
| cached | 218,149 | 199,860 | +9.2% | Kruda |
| updates | 2,122 | 2,032 | +4.5% | Kruda |

Score: Kruda 7 — Fiber 0

---

## 2026-02 Linux Single-Process Go 1.26.0 (dev-server 8-core, GOGC=400, Green Tea GC)

Config: `wrk -t4 -c128 -d10s`, 5 rounds per test, median reported.

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 578,370 | 550,607 | +5.0% | Kruda |
| plaintext | 563,582 | 543,999 | +3.6% | Kruda |
| db | 76,286 | 69,952 | +9.1% | Kruda |
| queries | 15,860 | 15,500 | +2.3% | Kruda |
| fortunes | 91,275 | 86,820 | +5.1% | Kruda |
| cached | 518,914 | 498,668 | +4.1% | Kruda |
| updates | 5,124 | 5,730 | -10.6% | Fiber |

Score: Kruda 6 — Fiber 1

Green Tea GC impact vs Go 1.25: db +7.9%, fortunes +6.1% for Kruda.

---

## 2026-02 Linux Single-Process Go 1.25.7 (dev-server 8-core, GOGC=400)

Config: `wrk -t4 -c128 -d10s`, 5 rounds per test, median reported.

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 563,378 | 543,136 | +3.7% | Kruda |
| plaintext | 562,690 | 548,086 | +2.7% | Kruda |
| db | 70,693 | 70,123 | +0.8% | Kruda |
| queries | 15,964 | 15,697 | +1.7% | Kruda |
| fortunes | 86,016 | 88,757 | -3.1% | Fiber |
| cached | 518,064 | 493,431 | +5.0% | Kruda |
| updates | 5,860 | 5,795 | +1.1% | Kruda |

Score: Kruda 6 — Fiber 1

---

## 2026-02 Linux Multi-Process (dev-server 8-core, GOGC=400)

Config: same. Kruda Turbo (8 children, SO_REUSEPORT, CPU pinning) vs Fiber Prefork.

### Run 1 — Before pool fix (MaxConns=4/child)

| Test | Kruda Turbo | Fiber Prefork | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 475,019 | 519,833 | -8.6% | Fiber |
| plaintext | 485,957 | 515,444 | -5.7% | Fiber |
| db | 64,834 | 70,829 | -8.5% | Fiber |
| queries | 15,196 | 15,113 | +0.5% | Kruda |
| fortunes | 77,837 | 84,933 | -8.4% | Fiber |
| cached | 463,584 | 484,657 | -4.3% | Fiber |
| updates | 5,871 | 5,084 | +15.5% | Kruda |

Score: Kruda 2 — Fiber 5

### Run 2 — After pool fix (MaxConns=32/child via KRUDA_WORKERS)

| Test | Kruda Turbo | Fiber Prefork | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 481,180 | 507,622 | -5.2% | Fiber |
| plaintext | 475,184 | 516,802 | -8.1% | Fiber |
| db | 68,719 | 69,034 | -0.5% | Fiber |
| queries | 15,071 | 14,856 | +1.4% | Kruda |
| fortunes | 84,161 | 85,960 | -2.1% | Fiber |
| cached | 464,382 | 477,514 | -2.8% | Fiber |
| updates | 5,064 | 5,018 | +0.9% | Kruda |

Score: Kruda 2 — Fiber 5

Note: Pool fix improved DB tests significantly (db: -8.5% → -0.5%, fortunes: -8.4% → -2.1%).
Both frameworks slower in multi-process than single-process — wrk -t4 -c128 insufficient
to saturate 8 prefork processes. Need higher concurrency (e.g., -t8 -c512) for valid multi-process bench.

---

## 2026-02 Linux Multi-Process Go 1.26.0 (dev-server 8-core, GOGC=400, Green Tea GC)

Config: same. Kruda Turbo (8 children, SO_REUSEPORT, CPU pinning) vs Fiber Prefork.

| Test | Kruda Turbo | Fiber Prefork | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 479,673 | 520,370 | -7.8% | Fiber |
| plaintext | 473,030 | 519,209 | -8.9% | Fiber |
| db | 67,281 | 71,071 | -5.3% | Fiber |
| queries | 14,764 | 14,792 | -0.2% | Fiber |
| fortunes | 84,074 | 85,771 | -2.0% | Fiber |
| cached | 459,909 | 484,644 | -5.1% | Fiber |
| updates | 5,053 | 5,033 | +0.4% | Kruda |

Score: Kruda 1 — Fiber 6

Worse than Go 1.25 multi (was 2-5). Green Tea GC didn't help multi-process mode.
Same root issue: wrk -t4 -c128 insufficient to saturate 8 prefork processes.

---

## 2026-02 Linux Multi-Process Go 1.26.0 — No CPU Pinning (dev-server 8-core, GOGC=400)

Config: `wrk -t4 -c128 -d10s`, 3 rounds per test, median reported.
Change: Removed `sched_setaffinity` CPU pinning from `SetupChild()`.

| Test | Kruda Turbo | Fiber Prefork | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 523,955 | 515,385 | +1.7% | Kruda |
| plaintext | 518,423 | 530,385 | -2.3% | Fiber |
| db | 70,264 | 70,282 | -0.0% | Fiber |
| queries | 14,706 | 14,908 | -1.4% | Fiber |
| fortunes | 84,491 | 87,005 | -2.9% | Fiber |
| cached | 489,711 | 486,624 | +0.6% | Kruda |
| updates | 5,061 | 5,081 | -0.4% | Fiber |

Score: Kruda 2 — Fiber 5

Massive improvement from removing CPU pinning:
- json: -7.8% → +1.7% (won back)
- plaintext: -8.9% → -2.3%
- db: -5.3% → -0.0% (essentially tied)
- cached: -5.1% → +0.6% (won back)
- All gaps narrowed significantly. Remaining gap is framework overhead, not Turbo architecture.

---

## 2026-02 Linux Go 1.26.0 — No CPU Pinning + Socket Opts (dev-server 8-core, GOGC=400)

Config: `wrk -t4 -c128 -d10s`, 3 rounds per test, median reported.
Changes: Removed CPU pinning + added TCP_DEFER_ACCEPT + TCP_FASTOPEN on listener.

### Single-Process

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 567,111 | 520,456 | +9.0% | Kruda |
| plaintext | 559,346 | 542,536 | +3.1% | Kruda |
| db | 75,632 | 69,724 | +8.5% | Kruda |
| queries | 15,802 | 15,272 | +3.5% | Kruda |
| fortunes | 89,923 | 83,978 | +7.1% | Kruda |
| cached | 517,566 | 503,017 | +2.9% | Kruda |
| updates | 4,975 | 5,829 | -14.6% | Fiber |

Score: Kruda 6 — Fiber 1

### Multi-Process (Turbo/Prefork)

| Test | Kruda Turbo | Fiber Prefork | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 526,438 | 513,345 | +2.6% | Kruda |
| plaintext | 514,443 | 517,713 | -0.6% | Fiber |
| db | 69,435 | 70,254 | -1.2% | Fiber |
| queries | 14,805 | 14,840 | -0.2% | Fiber |
| fortunes | 86,786 | 86,088 | +0.8% | Kruda |
| cached | 481,536 | 470,811 | +2.3% | Kruda |
| updates | 5,053 | 4,926 | +2.6% | Kruda |

Score: Kruda 4 — Fiber 3

Socket opts + no CPU pinning: multi-process improved from 2-5 to 4-3.
All 3 losses within 1.2% — essentially noise margin.
Single-process remains dominant at 6-1.

---

## 2026-02 Linux Micro-benchmark (i5-13500 8-core, Go 1.26.0)

Config: `go test -bench -benchmem -count=3 -benchtime=3s`, median of 3 runs.

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| StaticGET (ns/op) | 51.6 | 54.5 | +5.3% | Kruda |
| ParamGET (ns/op) | 53.5 | 55.6 | +3.8% | Kruda |

allocs/op = 0 for both. Security fix (query string safe copy) applied — no perf regression.



- Both apps use `valyala/fasthttp` + `jackc/pgx/v5` with prepared statements
- macOS results: single-process, GOGC=off
- Linux results: GOGC=400 (GOGC=off causes OOM on 16GB dev-server)
- Dev-server: 8-core, 16GB RAM, Ubuntu, Go 1.25.7 → upgraded to Go 1.26.0
- DB-bound tests (db, queries, updates, fortunes) have high variance between runs
- plaintext/json are CPU-bound and very close since both use fasthttp
- Multi-process mode: wrk -t4 -c128 insufficient to saturate 8 prefork processes

---

## 2026-02-28 Linux Full Bench v2 — UNNEST + pool fix (i5-13500 8-core, Go 1.26.0, GOGC=400)

Config: `wrk -t4 -c128 -d10s`, 3 rounds per test, median reported.

Changes vs v1:
- `updatesHandler`: replaced batch tx (N UPDATEs) with single UNNEST query → updates +76%
- `calcPoolConfig`: default workers = `gomaxprocs` (was 1) → pool=32 instead of 256

### Single-Process 🏆 PERFECT SCORE

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 568,358 | 532,104 | +6.8% | Kruda |
| plaintext | 572,916 | 522,763 | +9.6% | Kruda |
| db | 70,303 | 70,097 | +0.3% | Kruda |
| queries | 16,061 | 15,684 | +2.4% | Kruda |
| fortunes | 86,704 | 83,719 | +3.6% | Kruda |
| cached | 503,144 | 490,433 | +2.6% | Kruda |
| updates | 10,028 | 9,709 | +3.3% | Kruda |

Score: **Kruda 7 — Fiber 0**

### Multi-Process (Turbo vs Prefork) — v2 post-UNNEST fix

| Test | Kruda Turbo | Fiber Prefork | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 521,947 | 516,634 | +1.0% | Kruda |
| plaintext | 530,083 | 526,141 | +0.7% | Kruda |
| db | 72,803 | 71,638 | +1.6% | Kruda |
| queries | 15,285 | 15,394 | -0.7% | Fiber |
| fortunes | 84,121 | 86,271 | -2.5% | Fiber |
| cached | 487,777 | 495,923 | -1.6% | Fiber |
| updates | 8,511 | 8,676 | -1.9% | Fiber |

Score: Kruda 3 — Fiber 4 (all losses ≤ 2.5% — within noise margin)

---

## 2026-02-28 GOMAXPROCS Comparison (i5-13500 8-core, Go 1.26.0, GOGC=400)

Config: `wrk -t4 -c256 -d8s`, 1 round, warmup included.  
Script: `bench_gomaxprocs.sh`

Purpose: ทดสอบว่า GOMAXPROCS=1 ต่อ child process ดีกว่า default หรือไม่ และเทียบกับ Fiber TFB-style

### 4 Configs

| Config | Processes | GOMAXPROCS/process | Total parallelism |
|--------|----------:|-------------------:|------------------:|
| Kruda turbo GMAX=1 (current) | 8 | 1 | 8 |
| Kruda turbo GMAX=default | 8 | 8 | 64 |
| Fiber prefork GMAX=1 | 2* | 1 | 2 |
| Fiber prefork GMAX=default (TFB style) | 8 | 8 | 64 |

*Fiber prefork GMAX=1 spawn เพียง 2 processes เพราะ Fiber ใช้ GOMAXPROCS เพื่อกำหนดจำนวน children

### Results

| Test | Kruda GMAX=1 | Kruda GMAX=def | Fiber GMAX=1 | Fiber GMAX=def |
|------|------:|------:|------:|------:|
| json | **602,244** | 424,258 | 185,593 | 557,520 |
| plaintext | **566,066** | 424,340 | 186,956 | 560,376 |
| db | **66,715** | 55,929 | 59,671 | 66,285 |
| queries | 15,848 | 14,388 | **17,490** | 15,621 |
| fortunes | **80,422** | 69,354 | 49,937 | 77,210 |
| cached | **532,040** | 401,919 | 162,040 | 499,827 |
| updates | 7,241 | 7,354 | **11,647** | 7,564 |

### Kruda GMAX=1 vs Fiber GMAX=default (TFB-style fair comparison)

| Test | Kruda | Fiber | Diff |
|------|------:|------:|-----:|
| json | 602,244 | 557,520 | +8.0% |
| plaintext | 566,066 | 560,376 | +1.0% |
| db | 66,715 | 66,285 | +0.6% |
| queries | 15,848 | 15,621 | +1.5% |
| fortunes | 80,422 | 77,210 | +4.2% |
| cached | 532,040 | 499,827 | +6.4% |
| updates | 7,241 | 7,564 | -4.3% |

Score: **Kruda 6 — Fiber 1** (Fiber wins only updates by 4.3%)

### Key Findings

1. **Kruda GOMAXPROCS=1 ชนะ Fiber TFB-style ใน 6/7 tests** — design ปัจจุบันถูกต้อง
2. **Kruda GOMAXPROCS=default แย่กว่า** — 8 processes × GMAX=8 ทำให้ scheduler/GC contention สูง ไม่ควรเปลี่ยน
3. **Fiber GOMAXPROCS=1 แย่มาก** — json/plaintext/cached ตก ~65% เพราะ Fiber ไม่ได้ออกแบบสำหรับ single-thread per process
4. **updates/queries** — Fiber GMAX=1 ชนะเพราะ spawn แค่ 2 processes → pool connections ต่อ process มากกว่า → DB throughput สูงกว่า ไม่ใช่ framework overhead
5. **TFB จริง** ใช้ `-c 16,32,64,128,256,512` เอา best — เราใช้ `-c 256` fixed ซึ่งเป็น representative level

### Conclusion

Kruda turbo GOMAXPROCS=1 คือ optimal config สำหรับ TFB submission

---

## 2026-02-28 Multi-Process v3 — GoMaxProcs=2 (4 processes × GMAX=2)

Config: `wrk -t8 -c512 -d10s`, 3 rounds per test, median reported.
Kruda: 4 processes × GOMAXPROCS=2 × 64 conns = 256 total DB conns
Fiber: 8 processes × GOMAXPROCS=8 × 32 conns = 256 total DB conns

| Test | Kruda | Fiber | Diff | Winner |
|------|------:|------:|-----:|--------|
| json | 610,462 | 577,340 | +5.7% | **Kruda** |
| plaintext | 609,409 | 594,051 | +2.6% | **Kruda** |
| db | 63,832 | 60,303 | +5.9% | **Kruda** |
| queries | 14,879 | 14,872 | +0.0% | **Kruda** |
| fortunes | 76,674 | 72,198 | +6.2% | **Kruda** |
| cached | 543,204 | 534,165 | +1.7% | **Kruda** |
| updates | 6,972 | 7,518 | -7.3% | Fiber |

**Score: Kruda 6 — Fiber 1** (improved from 3-4 in v2)

### vs Multi-Process v2 (8 processes × GMAX=1)

| Test | v2 | v3 | Change |
|------|---:|---:|-------:|
| json | 521,947 | 610,462 | +17.0% |
| plaintext | 530,083 | 609,409 | +15.0% |
| db | 72,803 | 63,832 | -12.3% |
| queries | 15,285 | 14,879 | -2.7% |
| fortunes | 84,121 | 76,674 | -8.9% |
| cached | 487,777 | 543,204 | +11.4% |
| updates | 8,511 | 6,972 | -18.1% |

### Analysis

- CPU-bound tests (json/plaintext/cached) ดีขึ้นมากเพราะ GMAX=2 ลด process overhead
- DB-bound tests (db/queries/fortunes/updates) แย่ลงเพราะ 4 processes แทน 8 → kernel SO_REUSEPORT distribution ไม่สม่ำเสมอ
- updates แพ้ -7.3% เพราะ write contention บน DB สูงขึ้นเมื่อ connections ต่อ process มากขึ้น (64 vs 32)
- **Best multi config ยังต้องหา** — v2 ดีกว่าสำหรับ DB-bound, v3 ดีกว่าสำหรับ CPU-bound
