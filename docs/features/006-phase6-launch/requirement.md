# Phase 6 — Launch: Requirements

> **Goal:** Public launch, community building
> **Timeline:** Week 16-18
> **Spec Reference:** Sections 21, 22, 23
> **Depends on:** Phase 1-5 complete

## Milestone

```
Kruda Framework v1.0.0 RELEASED
- GitHub public repo: https://github.com/go-kruda/kruda
- go.pkg.dev page published
- 4 blog posts published
- Community: Discord + Telegram
- Reddit/HN launch posts
- 100+ GitHub stars (goal)
```

## Components

| # | Component | File(s) | Est. Days | Priority | Status |
|---|-----------|---------|-----------|----------|--------|
| 1 | GitHub Repo Setup | README, licenses, workflows | 1 | 🔴 | 🔴 |
| 2 | Release v1.0.0 | tags, CHANGELOG, artifacts | 1 | 🔴 | 🔴 |
| 3 | Blog Posts (4) | posts/ | 3 | 🔴 | 🔴 |
| 4 | Community Setup | Discord, Telegram | 1 | 🔴 | 🔴 |
| 5 | Launch Posts | Reddit, HN, Twitter | 1 | 🟡 | 🔴 |
| 6 | Press Kit | `docs/press-kit/` | 1 | 🟡 | 🔴 |
| 7 | Migration Guides | `docs/guide/from-*.md` | 2 | 🔴 | 🔴 |
| 8 | Interactive Playground | `play.kruda.dev` | 3 | 🟡 | 🔴 |

## Key Requirements

### GitHub Repository
- Repository: `https://github.com/go-kruda/kruda`
- Public: open source, MIT licensed
- README: compelling, feature highlights, benchmarks
- CONTRIBUTING.md: contribution guidelines
- CODE_OF_CONDUCT.md: code of conduct
- License file: MIT
- All workflows enabled: test, bench, deploy docs

### Release v1.0.0
- Tag: `v1.0.0`
- Release notes: summary of all 6 phases
- Binaries: kruda CLI for Windows, Linux, macOS
- Artifacts: Go modules published to pkg.dev
- Changelog: complete from phase 1-6
- Docker image: `docker pull go-kruda/kruda:1.0.0` (optional)

### Blog Posts (4 planned)
1. **"Introducing Kruda: The Type-Safe Go Framework"**
   - Problem statement: Gin/Fiber are fast but not type-safe
   - Solution: Kruda with Go generics
   - Demo: simple typed handler
   - Performance: benchmark results
   - Published: dev.to, Medium, personal blog

2. **"Netpoll Performance: Linux/macOS Optimization"**
   - Technical deep dive: Netpoll vs net/http
   - Implementation details
   - Benchmark results
   - Performance tuning guide

3. **"Building DI Containers in Go"**
   - Design patterns for dependency injection
   - Kruda's Container implementation
   - Examples: service injection, lazy loading
   - Best practices

4. **"Typed Handlers with Go Generics"**
   - Go 1.18+ generics explained
   - Kruda's C[T] pattern
   - Pre-compiled parser benefits
   - Zero-alloc validation

### Community Setup
- **Discord:** server creation, channels (announcements, questions, showcase)
- **Telegram:** group for quick questions + announcements
- **GitHub Discussions:** for long-form topics
- **Twitter:** @kruda_framework for announcements
- Invite link in README, docs, first issue

### Migration Guides (NEW — lower adoption barrier)

Help developers move from Gin/Fiber with minimal friction:

**"Coming from Gin" guide (`docs/guide/from-gin.md`):**
- Side-by-side code comparison: Gin vs Kruda for common patterns
- Handler migration: `gin.HandlerFunc` → `kruda.HandlerFunc` / `C[T]`
- Middleware migration: gin middleware → kruda middleware
- Context API mapping: `c.JSON()`, `c.Bind()`, etc.
- Router patterns: Gin routes → Kruda routes

**"Coming from Fiber" guide (`docs/guide/from-fiber.md`):**
- Side-by-side code comparison: Fiber vs Kruda
- Handler migration: `fiber.Handler` → `kruda.HandlerFunc` / `C[T]`
- Context safety: explain why Kruda copies strings (Fiber doesn't by default)
- Performance comparison: fasthttp vs Netpoll

**"Coming from net/http" guide (`docs/guide/from-stdlib.md`):**
- Handler migration: `http.HandlerFunc` → `kruda.HandlerFunc`
- Middleware pattern: `func(next http.Handler) http.Handler` → `kruda.MiddlewareFunc`
- Why upgrade: typed handlers, auto validation, DI, better error handling

### Interactive Playground (NEW — try without installing)

Web-based playground to try Kruda code without installation:

**Requirements:**
- URL: `https://play.kruda.dev` (or embedded in docs)
- Pre-loaded examples: hello world, typed handlers, validation, DI
- Run button: compile + execute in sandbox (use Go Playground API or custom backend)
- Share button: generate shareable URL with code
- Template selector: choose from 10+ starter examples
- Mobile-friendly: works on phone/tablet

**Implementation options (choose one):**
1. **Go Playground fork** — custom UI, send code to golang.org/x/playground
2. **WASM** — compile Go→WASM, run in browser (limited but serverless)
3. **Custom backend** — Docker sandbox, more control but needs hosting

### Launch Posts
- **Reddit:** r/golang post (discussion-focused)
- **Hacker News:** submission (technical angle)
- **Twitter:** thread + links
- **Go Forum:** golang-nuts post (if appropriate)
- **IndieHackers:** project launch

### Press Kit
- `docs/press-kit/PRESS_KIT.md`
- Project description (1-2 paragraphs)
- Key facts (founder, license, links)
- Testimonials (if available)
- Benchmark highlights
- Logo + screenshots

## Non-Functional Requirements

- **NFR-1:** All Phase 1-5 requirements met
- **NFR-2:** Repository is public and maintained
- **NFR-3:** All documentation is accurate and up-to-date
- **NFR-4:** Examples and tutorials work as written
- **NFR-5:** Community channels actively monitored
- **NFR-6:** Contribution process is clear and welcoming
