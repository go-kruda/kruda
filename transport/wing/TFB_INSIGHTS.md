# TFB Insights — เทคนิคจาก Go Framework ที่ทำคะแนนดี

วิเคราะห์จาก source code ของ framework ที่ติด top ใน TechEmpower Framework Benchmarks
เป้าหมาย: เอามาปรับใช้กับ Kruda/Wing

---

## Framework ที่ศึกษา

| Framework | ภาษา | จุดเด่นใน TFB | เทคนิคหลัก |
|-----------|------|---------------|------------|
| silverlining | Go | #2 Plaintext (28M RPS) | Custom HTTP parser, goroutine pool, pre-computed headers |
| gnet | Go | #8 Plaintext (25M RPS) | Event-driven epoll/kqueue, ring buffer, RawSyscall6 |
| fasthttp-prefork | Go | Top Go ใน DB tests ทั้งหมด | Prefork SO_REUSEPORT, zero-copy headers, object pooling |
| gearbox | Go | #12 Single Query, #16 Fortunes | Ternary search tree router, LRU route cache, fasthttp base |
| fiber-prefork | Go | #3 Cached Queries | fasthttp + prefork + caching layer |
| goravel fiber | Go | #2 Cached Queries | Fiber base + ORM caching |

---

## เทคนิคที่ PERF_ROADMAP ยังไม่ cover

### A. Hot Path — ลด overhead per-request

#### #13 LRU Route Cache

Source: gearbox — ใช้ LRU cache 1000 entries สำหรับ route lookup

```go
type router struct {
    cache    map[string]*matchResult // method+path → handler chain
    settings *Settings
}
```

TFB มีแค่ 7 routes แต่ cache ช่วยลด radix tree traversal overhead
Wing ทำ inline dispatch → route lookup ทุก request ยังเป็น cost

Impact: +2-3% | Effort: easy
แก้ที่: FeatherTable หรือ router layer

#### #14 Adaptive epoll_wait Timeout

Source: gnet — ใช้ adaptive timeout คู่กับ RawSyscall6

```
load สูง (events ต่อเนื่อง) → msec=0 (non-blocking spin, RawSyscall6)
idle (ไม่มี events)         → msec=-1 (block, Syscall6, yield to scheduler)
```

ใช้คู่กับ PERF_ROADMAP #4 RawSyscall6 จะได้ผลดีมาก
ป้องกัน CPU spin ตอน idle + ลด latency ตอน load สูง

```go
func (e *epollEngine) adaptiveWait(events []epollEvent) (int, error) {
    n, err := epollWait(e.epfd, events, 0) // try non-blocking first
    if n > 0 {
        e.idleCount = 0
        return n, err
    }
    e.idleCount++
    if e.idleCount > 64 {
        return epollWait(e.epfd, events, -1) // block
    }
    runtime.Gosched() // yield briefly
    return 0, nil
}
```

Impact: +3-5% | Effort: easy
แก้ที่: engine_linux.go

#### #15 Disable Header Normalization Option

Source: gearbox/fasthttp — ปิด header name normalization ลด CPU

```go
// gearbox ตั้ง fasthttp server:
DisableHeaderNamesNormalizing: true  // ไม่ normalize content-type → Content-Type
NoDefaultDate:                true  // Wing จัดการ Date เอง
NoDefaultContentType:         true  // handler set เอง
```

Wing ควรมี option ให้ปิดได้:

```go
type Config struct {
    DisableHeaderNormalizing bool
}
```

Impact: +1-2% | Effort: trivial
แก้ที่: http.go (parser)

---

### B. TFB DB Tests — เทคนิคเฉพาะ database

ทุก Go framework ที่ทำ DB tests ดีใน TFB ใช้เทคนิคเหล่านี้ร่วมกัน

#### #16 pgx Raw Protocol (ไม่ผ่าน database/sql)

```go
// ❌ ช้า — database/sql abstraction layer
db.QueryRow("SELECT id, randomNumber FROM World WHERE id = $1", id).Scan(&w.Id, &w.Num)

// ✅ เร็ว — pgx direct
pool.QueryRow(ctx, "SELECT id, randomNumber FROM World WHERE id = $1", id).Scan(&w.Id, &w.Num)
```

database/sql เพิ่ม overhead: interface conversion, prepared statement management, connection checkout
pgx ตรงลด ~15-20% latency per query

Impact: +15-20% DB tests | Effort: medium (TFB app level)

#### #17 Prepared Statement Cache

```go
// ตอน startup — prepare ทุก statement ครั้งเดียว
afterConnect := func(ctx context.Context, conn *pgx.Conn) error {
    _, err := conn.Prepare(ctx, "worldSelect",
        "SELECT id, randomNumber FROM World WHERE id = $1")
    _, err = conn.Prepare(ctx, "worldUpdate",
        "UPDATE World SET randomNumber = $1 WHERE id = $2")
    _, err = conn.Prepare(ctx, "fortuneSelect",
        "SELECT id, message FROM Fortune")
    return err
}

// ตอน query — ใช้ชื่อ prepared statement
pool.QueryRow(ctx, "worldSelect", id).Scan(&w.Id, &w.Num)
```

ลด round-trip ของ parse+plan per query

Impact: +5-10% DB tests | Effort: easy (TFB app level)

#### #18 Batch Updates (สำหรับ /updates test)

TFB updates test ต้อง update 1-20 rows per request
framework ที่เร็วใช้ pgx batch แทน individual statements:

```go
// ❌ ช้า — 20 round-trips
for _, w := range worlds {
    pool.Exec(ctx, "worldUpdate", w.RandomNumber, w.Id)
}

// ✅ เร็ว — 1 round-trip
batch := &pgx.Batch{}
for _, w := range worlds {
    batch.Queue("worldUpdate", w.RandomNumber, w.Id)
}
results := pool.SendBatch(ctx, batch)
defer results.Close()
for range worlds {
    _, _ = results.Exec()
}
```

Impact: +20-30% updates test | Effort: easy (TFB app level)

---

### C. Advanced — ยากขึ้นแต่ได้ผลดี

#### #19 Work-Stealing Goroutine Pool

Source: silverlining gopool

Wing มี Pool dispatch อยู่แล้ว (shared channel)
ปัญหา: shared channel เป็น contention point ตอน high concurrency

Work-stealing pattern:
- แต่ละ worker มี local queue (lock-free ring buffer)
- worker idle → steal จาก worker อื่น
- ลด contention บน shared channel

```
┌─────────┐  ┌─────────┐  ┌─────────┐
│ Worker 0 │  │ Worker 1 │  │ Worker 2 │
│ [local Q]│  │ [local Q]│  │ [local Q]│
│  ↕ steal │  │  ↕ steal │  │  ↕ steal │
└─────────┘  └─────────┘  └─────────┘
```

Impact: +3-5% Pool dispatch | Effort: hard
แก้ที่: transport.go (worker pool)

#### #20 Pre-computed JSON Response

Extension ของ PERF_ROADMAP #2 Pre-built Response

TFB JSON test return `{"message":"Hello, World!"}` ทุกครั้ง
Pre-build full HTTP response รวม headers:

```go
var jsonFullResponse = buildStaticResponse(200, "application/json",
    []byte(`{"message":"Hello, World!"}`))

// Per request: syscall.Write(fd, jsonFullResponse) — no marshal, no build
```

ใช้กับ Bolt/Flash preset ที่ response คงที่
Feather system รองรับอยู่แล้ว — แค่ต้อง implement static response registry

Impact: +5-8% JSON test | Effort: easy
แก้ที่: http.go หรือ static_response.go ใหม่

---

## สิ่งที่ PERF_ROADMAP มีแล้วและตรงกับ TFB analysis ✅

| PERF_ROADMAP # | เทคนิค | ใช้โดย |
|----------------|--------|--------|
| #1 | Direct Write + Read Loop | silverlining, actix, gnet |
| #2 | Pre-built Response | silverlining, actix, gnet TFB |
| #3 | Pointer in epoll data | gnet poll_opt |
| #4 | RawSyscall6 | gnet |
| #5 | XOR Method Hash | silverlining |
| #6 | Combined Date+Server Header | silverlining FastDateServer |
| #7 | CAS-gated Wakeup | gnet |
| #8 | Larger Read Buffers | actix (32KB default) |
| #9 | CPU Affinity | ntex |
| #12 | Prefork Mode | silverlining, fasthttp, fiber |

---

## ช่องว่างระหว่าง Go กับ Rust/C++ ใน TFB

| Test | Top Go | Top Rust | Gap | สาเหตุหลัก |
|------|--------|----------|-----|------------|
| Plaintext | 28M (silverlining) | 28M (faf) | ≈0% | Go ไม่ได้ช้ากว่าถ้า optimize ถูกจุด |
| JSON | 3.1M (silverlining) | 3.1M (may-minihttp) | ≈0% | JSON serialization เป็น bottleneck ไม่ใช่ runtime |
| Single Query | 1.07M (gearbox) | 1.45M (may-minihttp) | -26% | DB driver overhead (pgx vs tokio-postgres) |
| Fortunes | 959K (fasthttp) | 1.33M (may-minihttp) | -28% | Template engine + DB driver |
| Updates | 30.8K (fiber) | 59.4K (viz) | -48% | Batch strategy + async driver |

Insight: Plaintext/JSON แทบไม่มี gap — ปัญหาอยู่ที่ DB layer
Wing + pgx optimized น่าจะปิด gap ได้เหลือ ~10-15%

---

## Priority สำหรับ Kruda TFB Submission

```
Phase 1 — Wing transport optimizations (PERF_ROADMAP #1-#12)
Phase 2 — เพิ่ม #13-#15 (route cache, adaptive wait, header opts)
Phase 3 — TFB app: #16-#18 (pgx raw, prepared stmts, batch updates)
Phase 4 — Advanced: #19-#20 (work-stealing pool, static JSON response)
```

---

## Sources

- [silverlining](https://github.com/go-www/silverlining) — Custom h1 parser, FastDateServer, gopool
- [gnet](https://github.com/panjf2000/gnet) — Event-driven, RawSyscall6, ring buffer, elastic events
- [fasthttp](https://github.com/valyala/fasthttp) — Zero-alloc, prefork, object pooling
- [gearbox](https://github.com/gogearbox/gearbox) — TST router, LRU cache, fasthttp tuning
- [fiber](https://github.com/gofiber/fiber) — fasthttp + Express API + prefork
- [GoFrame TFB analysis](https://goframe.org/en/articles/techempower-web-benchmarks-r23)
