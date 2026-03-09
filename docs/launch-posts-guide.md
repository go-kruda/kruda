# Kruda v1.0.0 Launch Posts Guide

คู่มือนี้เป็น playbook สำหรับการโพสต์ประกาศ Kruda v1.0.0 บนแพลตฟอร์มต่าง ๆ พร้อมเนื้อหาที่เตรียมไว้ให้ copy-paste ได้เลย คำแนะนำเรื่อง timing และวิธีรับมือกับ feedback ที่จะตามมา

---

## สารบัญ

1. [r/golang Post](#1-rgolang-post)
2. [Hacker News](#2-hacker-news)
3. [dev.to Blog](#3-devto-blog)
4. [General Tips & Response Templates](#4-general-tips--response-templates)
5. [Timing Strategy](#5-timing-strategy)
6. [Launch Day Checklist](#6-launch-day-checklist)

---

## 1. r/golang Post

### กฎและวัฒนธรรมของ r/golang

**สิ่งที่ชุมชนชอบ:**
- Technical depth -- แสดง code ตัวอย่างจริง ไม่ใช่แค่ marketing text
- ความซื่อสัตย์ -- ยอมรับข้อจำกัดตรง ๆ (เช่น Wing ใช้ได้แค่ Linux)
- Benchmark methodology ชัดเจน -- บอก hardware, tool, command ที่ใช้
- Comparison ที่ fair -- ไม่ด่า framework อื่น
- Show don't tell -- โค้ดพูดดังกว่าคำโฆษณา
- ตอบ comment ทุกอัน อย่างรวดเร็วและ technical

**สิ่งที่ชุมชนไม่ชอบ:**
- Marketing speak / hype ที่มากเกินจริง
- "Why another framework?" ที่ไม่มีคำตอบชัด
- ดูถูก framework อื่น
- Benchmark โดยไม่เปิดเผย methodology
- โพสต์แล้วหายไป ไม่ตอบ comment
- อ้าง star count หรือ social proof ที่ยังไม่มี

**Tone:** ถ่อมตัว + technical + ตื่นเต้นแบบ developer ที่อยากแชร์สิ่งที่สร้าง ไม่ใช่ tone ของบริษัทขายของ

### Title Options

เลือกอันที่รู้สึกเหมาะที่สุด:

**Option A (แนะนำ):**
```
Kruda v1.0.0 — Type-safe Go web framework with auto CRUD, built-in DI, and a custom async I/O transport
```

**Option B (benchmark-focused):**
```
Kruda v1.0.0 — Go web framework with typed handlers C[T], 846K req/s on custom epoll transport
```

**Option C (minimalist):**
```
Kruda — a Go web framework with typed handlers, auto CRUD, and pluggable transport (v1.0.0)
```

> คำแนะนำ: Option A balance ระหว่าง feature highlight กับ ความน่าสนใจ Option B เหมาะถ้าต้องการดึง performance audience แต่ระวังจะโดน benchmark skeptics

### Full Body Text (Ready to Copy-Paste)

```markdown
Hey r/golang!

After 8 months of development and 7 phases of iteration, I'm happy to share the v1.0.0 release
of Kruda — a Go web framework that focuses on type safety through generics and high performance
through a custom async I/O transport.

**GitHub:** https://github.com/go-kruda/kruda
**Docs:** https://kruda.dev
**pkg.go.dev:** https://pkg.go.dev/github.com/go-kruda/kruda

## What makes it different?

**1. Typed handlers with `C[T]`**

Instead of manually parsing body/params/query into variables, you define a struct and Kruda does
the rest — including validation:

```go
type CreateUser struct {
    Name  string `json:"name" validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
}

kruda.Post[CreateUser, User](app, "/users", func(c *kruda.C[CreateUser]) (*User, error) {
    // c.In.Name and c.In.Email are already parsed and validated
    return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
})
```

Body, path params, and query params can all live in one struct with `json:`, `param:`, and `query:` tags.
Validation errors return structured 422 JSON with field-level details.

**2. Auto CRUD**

Implement a `ResourceService[T, ID]` interface and get 5 REST endpoints in one line:

```go
kruda.Resource[User, string](app, "/users", userService)
// GET /users, GET /users/:id, POST /users, PUT /users/:id, DELETE /users/:id
```

**3. Built-in DI container**

Optional, no codegen, uses generics:

```go
c := kruda.NewContainer()
c.Give(&UserService{db: connectDB()})

app := kruda.New(kruda.WithContainer(c))
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.MustResolve[*UserService](c)
    return c.JSON(svc.ListAll())
})
```

**4. Pluggable transport**

- **Wing** (Linux): Custom async I/O using raw epoll + eventfd — no fasthttp, no net/http
- **fasthttp**: Default on macOS
- **net/http**: Default on Windows, auto-fallback for TLS/HTTP2

Kruda auto-selects the best transport for your platform. You can override:

```go
app := kruda.New(kruda.Wing())    // force Wing
app := kruda.New(kruda.NetHTTP()) // force stdlib
```

## Benchmarks

Measured with `wrk -t4 -c256 -d5s` on Linux i5-13500 (8P cores), GOGC=400.
Full source code for all frameworks: https://github.com/go-kruda/kruda/tree/main/bench/reproducible

| Test | Kruda (Go) | Fiber (Go) | Actix Web (Rust) | vs Fiber | vs Actix |
|------|--:|--:|--:|--:|--:|
| plaintext | **846,622** | 670,240 | 814,652 | +26% | +4% |
| JSON | **805,124** | 625,839 | 790,362 | +29% | +2% |
| db | **108,468** | 107,450 | 37,373 | +1% | +190% |
| fortunes | 104,144 | **106,623** | 45,078 | -2% | +131% |

The Wing transport works by bypassing the Go HTTP stack entirely and doing raw syscall I/O.
It uses per-core workers with epoll for readiness + eventfd for inter-goroutine signaling.

Fiber wins on fortunes by ~2% — I consider that within noise.
The `db` and `fortunes` benchmarks use actual PostgreSQL via pgx.

## What's included

- **Core:** Router (radix tree, zero-alloc), Context (sync.Pool), Middleware pipeline,
  Typed handlers, Input binding + validation, OpenAPI 3.1 auto-gen, Error mapping,
  Graceful shutdown, Health checks, SSE, Static files, TestClient
- **Built-in middleware:** Recovery, Logger, CORS, RequestID, Timeout, Security headers
- **10 contrib modules:** jwt, websocket, ratelimit, session, compress, etag, cache, otel,
  prometheus, swagger
- **21 examples:** From hello-world to hexagonal architecture
- **Migration guides:** Coming from Gin, Fiber, Echo, or stdlib
- **CLI:** `kruda dev` (hot reload), `kruda new` (scaffolding)

## Requires Go 1.25+

We use generic type aliases from Go 1.25 for the `C[T]` typed handler system.

## Known limitations

- Wing transport is Linux-only (macOS/Windows fall back to fasthttp/net/http)
- Wing doesn't support HTTP/2 or TLS natively (use net/http transport or a reverse proxy)
- This is a v1.0.0 from a solo developer — battle-testing in production is still early
- Ecosystem is smaller than Gin/Fiber — no third-party middleware ecosystem yet

## What I'd love feedback on

- API design of `C[T]` typed handlers
- DI container ergonomics
- Anything that feels weird or non-idiomatic

Happy to answer any questions. The full benchmark source code is in the repo if you want to
reproduce or challenge the numbers.
```

### Comment ที่คาดว่าจะเจอ และวิธีตอบ

รวบรวมไว้ในหัวข้อ [General Tips](#4-general-tips--response-templates)

---

## 2. Hacker News

### กฎและวัฒนธรรมของ HN

**กฎสำคัญ:**
- Title ต้องไม่เกิน 80 ตัวอักษร
- ห้าม editorialized title (ห้ามใส่ "Amazing!" หรือ "Revolutionary")
- ห้าม upvote ring / ขอให้คนมา upvote
- ห้ามใช้ all caps
- ใช้ "Show HN:" prefix ถ้าเป็นของที่ตัวเองสร้าง (ซึ่งเราเข้าข่าย)
- Link ไปที่ GitHub repo โดยตรง
- เขียน comment อธิบายเพิ่มเติมเป็น first comment ของตัวเอง

**ความแตกต่างจาก r/golang:**
- HN มี audience กว้างกว่า -- ไม่ใช่แค่ Go devs มี Rust, TS, Java, Python devs ด้วย
- สนใจ "why" มากกว่า "how" -- ทำไมต้องสร้างอันใหม่? แก้ปัญหาอะไร?
- ชอบ technical innovation ที่แท้จริง -- Wing transport เป็น hook ที่ดีมากสำหรับ HN
- Skeptical กว่า r/golang มาก -- ต้องมี substance จริง ๆ
- ไม่ค่อยชอบ "yet another X" -- ต้องบอกให้ได้ว่า genuinely ต่างยังไง
- ให้ความสำคัญกับ benchmark ที่ reproducible -- เปิด source ของ bench suite ทั้งหมด

### Title Options

**Option A (แนะนำ):**
```
Show HN: Kruda – Go web framework with typed handlers, auto CRUD, custom epoll transport
```
(79 ตัวอักษร -- พอดี)

**Option B:**
```
Show HN: Kruda – Type-safe Go web framework beating Fiber and Actix in benchmarks
```
(76 ตัวอักษร)

**Option C (เน้น technical):**
```
Show HN: Kruda – Go web framework with epoll+eventfd transport (846K req/s)
```
(68 ตัวอักษร)

> คำแนะนำ: Option A ดีที่สุดเพราะ informative โดยไม่ overpromise Option B อาจดึงความสนใจมากกว่าแต่เสี่ยงโดน skeptics Option C เน้น systems programming angle ซึ่ง HN ชอบ

### Link

ใช้ GitHub repo URL: `https://github.com/go-kruda/kruda`

### First Comment (เขียนทันทีหลังโพสต์)

```markdown
Hi HN, I'm Tiger, the author of Kruda.

I've been working on this for about 8 months. The two things I think are genuinely novel:

1. **Wing transport** — Instead of building on top of net/http or fasthttp, I wrote a custom
async I/O layer using raw epoll + eventfd on Linux. Each worker thread gets its own epoll instance,
and eventfd handles cross-goroutine wakeups. This gets us ~846K req/s plaintext on an i5-13500,
which is +26% vs Fiber (Go) and +4% vs Actix Web (Rust).

The benchmark source code for all three frameworks is in the repo:
https://github.com/go-kruda/kruda/tree/main/bench/reproducible

I'd genuinely welcome anyone reproducing these numbers or finding flaws in the methodology.

2. **Typed handlers with `C[T]`** — Go generics let you define request input as a struct with
tags for body/params/query, and the framework parses + validates automatically. The return type
is also generic, so your handler signature tells you exactly what goes in and what comes out:

    kruda.Post[CreateUser, User](app, "/users", func(c *kruda.C[CreateUser]) (*User, error) {
        return &User{Name: c.In.Name}, nil
    })

This isn't unique to Kruda (Fuego and Huma do something similar), but I think the unified
`C[T]` approach where body+params+query all go in one struct is the cleanest version of this
pattern.

Other things that might interest HN:
- Auto CRUD: implement a service interface, get 5 REST endpoints
- Built-in DI container using generics (optional, not forced)
- 10 contrib modules (JWT, WebSocket, rate limiting, OpenTelemetry, etc.)
- Pluggable transport: Wing on Linux, fasthttp on macOS, net/http on Windows — auto-selected

Limitations I want to be upfront about:
- Solo developer project — I'm aware this is a risk for production adoption
- Wing is Linux-only
- No HTTP/2 on Wing (use reverse proxy or net/http transport)
- Young project — not battle-tested at scale yet

Happy to discuss architecture decisions, benchmark methodology, or anything else.
```

### Post-HN Tips

- ตอบ comment ภายใน 30 นาที โดยเฉพาะ 2 ชั่วโมงแรก -- HN ranking ขึ้นอยู่กับ engagement
- ถ้ามีคนถาม technical question ลึก ๆ ตอบแบบ in-depth เพราะ HN ให้คุณค่ากับ founder ที่ technical จริง
- อย่าปกป้อง project เกินไป -- ยอมรับ criticism ที่สมเหตุสมผล
- ถ้ามีคน submit ซ้ำ (duplicate) ให้ flag ตัวที่ submit ทีหลัง

---

## 3. dev.to Blog

### Blog Post Outline

**Title:** "Building Kruda: A Go Web Framework That Beats Fiber and Actix Web"

**Tags:** `go`, `golang`, `webdev`, `performance`, `opensource`

**Cover Image:** ใช้ benchmark chart จาก README (docs/assets/benchmark-chart.png)

### โครงสร้างบทความ

```markdown
---
title: "Building Kruda: A Go Web Framework That Beats Fiber and Actix Web"
published: true
tags: go, golang, webdev, performance, opensource
cover_image: [benchmark chart URL]
---

## TL;DR

Kruda is a Go web framework with:
- Typed handlers using Go generics (body + params + query in one struct)
- Auto CRUD endpoints from a service interface
- A custom async I/O transport (epoll + eventfd) that gets 846K req/s
- Built-in DI container, no codegen needed

GitHub: https://github.com/go-kruda/kruda

## Why Another Go Web Framework?

[อธิบาย gap ระหว่าง performance frameworks (Fiber) กับ type-safe frameworks (stdlib)]

Go frameworks ตอนนี้แบ่งเป็น 2 ค่าย:
- **Performance-first** (Fiber, fasthttp): เร็วมากแต่ไม่ stdlib compatible, ไม่มี type safety
- **Stdlib-compatible** (Gin, Echo, Chi): ใช้ง่ายแต่ต้องเขียน boilerplate เยอะ ไม่มี auto-validation

Kruda ต้องการรวมทั้งสองโลก: performance ระดับ Fiber (หรือดีกว่า) + type safety ผ่าน generics +
auto-everything ที่ลด boilerplate 60-70%

## Typed Handlers: The Core Innovation

[แสดง before/after comparison กับ Gin/Fiber]

### Before (Gin/Fiber style)
```go
func CreateUser(c *gin.Context) {
    var input CreateUserInput
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    // manual validation...
    // create user...
    c.JSON(201, user)
}
```

### After (Kruda)
```go
kruda.Post[CreateUserInput, User](app, "/users", func(c *kruda.C[CreateUserInput]) (*User, error) {
    // c.In is already parsed and validated
    return createUser(c.In), nil
})
```

## Wing Transport: How We Beat Fiber by 26%

[Technical deep-dive เรื่อง epoll + eventfd architecture]
[แสดง benchmark table + link ไป reproducible bench suite]

## Auto CRUD: 5 Endpoints in One Line

[แสดง ResourceService interface + example]

## Getting Started

[Quick start code]

## What's Next

[Roadmap: TechEmpower submission, HTTP/2 on Wing, community growth]
```

### SEO Tips สำหรับ dev.to

- Title ต้องมี keywords: "Go", "web framework", "performance"
- ใส่ canonical URL ถ้ามี blog post ที่เว็บตัวเองด้วย
- dev.to ชอบ listicle format และ code blocks เยอะ ๆ
- ความยาวที่ดี: 1500-2500 คำ
- ใส่ TOC ตอนต้น

---

## 4. General Tips & Response Templates

### Marketing Do's and Don'ts

**Do:**
- แสดงโค้ดจริง ไม่ใช่แค่พูดว่า "easy to use"
- บอกข้อจำกัดก่อนที่คนอื่นจะมาชี้ให้ดู (preemptive honesty)
- ให้เครดิต framework อื่นที่เป็นแรงบันดาลใจ
- ใช้ตัวเลขที่ reproducible เสมอ -- เปิด benchmark source code
- ตอบ comment ทุกอันที่ constructive ภายใน 2 ชั่วโมง
- ยอมรับเมื่อไม่รู้คำตอบ -- "Good question, I haven't thought about that"
- เปรียบเทียบแบบ fair -- แสดงจุดที่เราแพ้ด้วย (fortunes -2% vs Fiber)

**Don't:**
- อย่าเรียกตัวเองว่า "revolutionary" หรือ "game-changing"
- อย่าด่าหรือ belittle framework อื่น
- อย่าซ่อน benchmark methodology
- อย่า spam หลาย subreddit พร้อมกัน
- อย่าใช้ bot หรือขอให้คนมา upvote
- อย่าพูดว่า "production-ready" ถ้ายังไม่มี production user
- อย่าอ้าง "zero dependencies" ถ้ายังมี Sonic JSON (พูดว่า "minimal dependencies" แทน)

### วิธีตอบ "Why another framework?"

นี่คือคำถามที่จะเจอ 100% และเป็นคำถามที่ถูกต้อง ตอบแบบ specific:

```
That's fair — the Go ecosystem already has great options. Here's the specific gap I was trying to fill:

1. **Fiber** is fast but not stdlib compatible and doesn't have typed handlers.
   If you want to switch away, you rewrite everything.

2. **Gin/Echo** are stdlib compatible but don't use generics for type-safe handlers.
   You still write manual binding and validation code.

3. **Fuego/Huma** do typed handlers but don't have a custom transport layer for max performance.

Kruda tries to combine these: typed handlers (like Fuego) + high-performance transport
(beyond Fiber) + stdlib fallback (like Gin). Whether that combination is worth a new framework
is a fair debate — I think it is, but I'm biased.

If Gin or Fiber is working well for you, there's no urgent reason to switch. Kruda might
be interesting if you're starting a new project and want type-safe handlers + performance.
```

### วิธีตอบ "Benchmarks are meaningless"

```
You're right that micro-benchmarks don't tell the whole story. A few things I try to do to
make ours more useful:

1. **Full source code is public** — you can reproduce everything:
   https://github.com/go-kruda/kruda/tree/main/bench/reproducible

2. **We include realistic tests** — not just plaintext. The `db` and `fortunes` benchmarks
   hit actual PostgreSQL via pgx, which is closer to real workloads.

3. **We show where we lose** — Fiber beats us on fortunes by ~2%, and our Wing transport
   is Linux-only. We don't cherry-pick.

4. **The transport is pluggable** — if you don't trust our benchmarks, you can use net/http
   or fasthttp transport and still get the typed handler DX. Performance is a bonus, not
   the only selling point.

That said, I fully agree that the best benchmark is your own workload on your own hardware.
```

### วิธีตอบ "Solo developer = risk"

```
That's a legitimate concern and I want to be honest about it:

1. Kruda is MIT licensed and the codebase is ~15K lines of well-tested Go. If I disappear
   tomorrow, anyone can fork and maintain it.

2. I'm actively looking for co-maintainers. If you're interested in contributing, I'd love
   to onboard you.

3. The architecture intentionally minimizes coupling. The transport layer is pluggable,
   so even if Wing development stalls, the rest of the framework works fine on fasthttp
   or net/http.

4. For production use today, I'd recommend evaluating based on your risk tolerance.
   If you need an SLA and a company behind it, Gin or stdlib is a safer bet. If you're
   building a new project and want to try something modern, Kruda is ready.
```

### วิธีตอบ "Go doesn't need DI" / "DI is un-idiomatic"

```
Totally valid perspective. That's why DI in Kruda is completely optional.

You can use Kruda without ever touching the container:

    app := kruda.New()
    app.Get("/users", func(c *kruda.Ctx) error {
        return c.JSON(myService.ListAll())
    })

The DI container exists for people who want it — especially for larger apps where manually
wiring dependencies gets tedious. It uses generics (no reflection, no codegen, no struct
tags), so it's about as "Go-like" as DI can get:

    c.Give(&UserService{})
    svc := kruda.MustResolve[*UserService](ctx)

If you prefer manual dependency injection, that works perfectly fine with Kruda. The
container is just a convenience, not a requirement.
```

### วิธีตอบ "How is this different from Fuego/Huma?"

```
Great question — Fuego and Huma are the closest projects to Kruda. Key differences:

1. **Transport**: Fuego and Huma use net/http. Kruda has a custom epoll+eventfd transport
   (Wing) for ~2x throughput on Linux. You can also use net/http if you prefer.

2. **C[T] unification**: Kruda puts body + path params + query params in ONE struct.
   Most typed handler frameworks separate them.

3. **Auto CRUD**: `kruda.Resource[T, ID](app, "/path", service)` generates 5 endpoints
   from a service interface. This is unique to Kruda.

4. **Built-in DI**: Optional container with `Give`/`MustResolve`. Fuego and Huma don't
   include DI.

If you like Fuego or Huma's approach to typed handlers, you'll probably like Kruda too.
The main reasons to choose Kruda would be performance (Wing transport), auto CRUD, or
the DI container.
```

### วิธีตอบ "Go 1.25+ is too new"

```
Fair point. Go 1.25 introduces generic type aliases which we need for C[T]. This is a
deliberate trade-off: we chose to use modern Go features rather than support older versions.

Go's release cycle means 1.25 will be widely available within a few months of its release,
and most teams update Go versions relatively quickly compared to other languages.

If you're stuck on an older Go version, Kruda isn't for you right now — and that's okay.
We'd rather use the right language features than add workarounds for compatibility.
```

---

## 5. Timing Strategy

### เวลาที่ดีที่สุดในการโพสต์

| Platform | วัน | เวลา (US timezone) | เหตุผล |
|----------|-----|---------------------|--------|
| r/golang | อังคาร-พฤหัสบดี | 9:00-11:00 AM EST | Weekday morning US = developer activity สูงสุด |
| Hacker News | อังคาร-พุธ | 8:00-10:00 AM EST | อ้างอิงจากสถิติ HN front page ก่อนหน้า |
| dev.to | จันทร์-อังคาร | ตลอดวัน | dev.to ใช้ algorithm ที่ไม่ time-sensitive มาก |

> สำหรับเวลาไทย: EST +12 ชั่วโมง ดังนั้น 9:00 AM EST = 21:00 (3 ทุ่ม) เวลาไทย

### กลยุทธ์การ Stagger โพสต์

**ห้ามโพสต์ทุกที่พร้อมกัน** -- Stagger เพื่อจัดการ comment ได้ทัน

**วันที่ 1 (อังคาร):**
- 21:00 น. (เวลาไทย): โพสต์ r/golang
- เปิด notification ไว้ ตอบ comment ตลอดคืน
- ตั้งเป้าตอบทุก comment ภายใน 30 นาที ใน 3 ชั่วโมงแรก

**วันที่ 2 (พุธ):**
- ประเมินผล r/golang post -- ถ้าได้ feedback ที่ดี ปรับ messaging ก่อนโพสต์ HN
- 20:00-22:00 น. (เวลาไทย): โพสต์ HN Show HN
- เขียน first comment ทันที (เตรียมไว้ก่อนแล้ว copy-paste)
- อยู่หน้าจอ 3-4 ชั่วโมง ตอบ comment

**วันที่ 3-4 (พฤหัสบดี-ศุกร์):**
- โพสต์ dev.to blog (longer form content)
- ตอบ comment ที่ค้างจาก r/golang + HN
- Cross-link: ถ้า HN ติด front page ให้ update r/golang post ด้วย edit

**สัปดาห์ที่ 2:**
- สรุป feedback ที่ได้
- เขียน follow-up blog post ตอบคำถามที่เจอบ่อย
- แชร์ใน Gopher Slack #frameworks channel
- โพสต์ใน Go Discord

### Follow-up Actions หลังโพสต์

**ภายใน 24 ชั่วโมง:**
- [ ] ตอบ comment ทั้งหมดที่เป็น question
- [ ] จด feedback ที่น่าสนใจเพื่อปรับปรุง project
- [ ] ถ้ามีคนพบ bug หรือ issue ให้ acknowledge ทันที

**ภายใน 1 สัปดาห์:**
- [ ] สร้าง GitHub issue จาก feedback ที่ actionable
- [ ] เขียน follow-up post/comment ถ้ามี interesting discussion
- [ ] ส่งข้อความหา potential contributor ที่แสดงความสนใจ
- [ ] อัพเดท README/docs ตาม feedback

**ภายใน 1 เดือน:**
- [ ] ออก patch release ถ้ามี bug reports
- [ ] เขียน "Lessons Learned from Launching" blog post (dev.to / blog)
- [ ] Submit ไป TechEmpower Framework Benchmarks (ถ้ายังไม่ได้ทำ)
- [ ] ติดต่อ Go Time podcast / Cup o' Go

---

## 6. Launch Day Checklist

### ก่อนโพสต์ (T-24h)

- [ ] README มี benchmark table ที่อัพเดทล่าสุด
- [ ] GitHub repo สะอาด: CI green, no open security issues
- [ ] ทุก example ทำงานได้จริง (`go run .` ทุก example)
- [ ] docs/guide/ ทุกหน้าถูกต้อง
- [ ] CHANGELOG มี v1.0.0 entry
- [ ] LICENSE ถูกต้อง (MIT)
- [ ] CONTRIBUTING.md มีอยู่และชัดเจน
- [ ] SECURITY.md มีอยู่
- [ ] GitHub Discussions เปิดอยู่
- [ ] pkg.go.dev ขึ้นแล้ว (ทำ `go get` จาก module อื่นเพื่อ trigger indexing)
- [ ] Benchmark source code ใน `bench/reproducible/` ทำงานได้

### ก่อนโพสต์ (T-1h)

- [ ] เตรียม post body ใน text editor (ไม่พิมพ์ใน Reddit/HN โดยตรง)
- [ ] เตรียม response templates ไว้ใน clipboard manager
- [ ] เตรียม HN first comment ไว้แล้ว
- [ ] เปิด GitHub notifications
- [ ] ปิดงานอื่นทั้งหมด -- ต้อง available 3-4 ชั่วโมง

### หลังโพสต์ (T+0 to T+4h)

- [ ] โพสต์แล้ว -- copy link เก็บไว้
- [ ] (HN) เขียน first comment ภายใน 2 นาที
- [ ] ตอบ comment ทุกอันภายใน 30 นาที
- [ ] Monitor tone -- ถ้ามี negativity ที่ unfair อย่าตอบแบบ defensive ให้ acknowledge + ตอบด้วย facts
- [ ] ถ้ามีคนรายงาน bug -- acknowledge ทันที สร้าง issue ใน GitHub

### หลังโพสต์ (T+24h)

- [ ] สรุปผลลัพธ์: upvotes, comments, GitHub stars, traffic
- [ ] จด top 5 feedback ที่ได้
- [ ] วางแผน next steps

---

## Appendix: Platform-Specific Formatting

### Reddit Markdown Tips
- ใช้ `##` สำหรับ headings (ไม่ใช่ `#` ซึ่งใหญ่เกินไปบน Reddit)
- Code blocks ใช้ 4 spaces indent หรือ triple backtick (Reddit supports both)
- Table ใช้ pipe `|` format ได้
- Bold ใช้ `**text**`
- Link: `[text](url)`
- แบ่ง paragraph ให้สั้น -- Reddit users scan ไม่ใช่ read

### HN Formatting Tips
- HN ไม่มี markdown -- ใช้ plain text
- Indent 2 spaces สำหรับ code blocks
- ไม่มี bold/italic -- ใช้ CAPS sparingly หรือ *asterisks* สำหรับ emphasis
- Link: paste URL ตรง ๆ จะกลายเป็น clickable link
- ขึ้นบรรทัดใหม่ต้องเว้น blank line (single newline ไม่ work)

### dev.to Formatting Tips
- Full markdown support
- Front matter YAML required (`title`, `tags`, `published`)
- สามารถ embed GitHub gists, CodePen, YouTube
- ใส่ `cover_image` ในfront matter สำหรับ thumbnail
- Series feature: ถ้าจะเขียนหลายตอน ใส่ `series: "Building Kruda"`
