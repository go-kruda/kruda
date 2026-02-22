# Phase 1 — Foundation: Issues

> Track bugs, concerns, and review notes here.
> Use labels: `[BUG]` `[CONCERN]` `[REVIEW]` `[DECISION]`

---

## Open Issues

### `[CONCERN]` C-002: Context locals key type
- **Description:** Current `locals` map uses `map[string]any`. For Phase 2 Provide/Need (type-safe DI), we need `map[any]any` keyed by `reflect.Type`. Consider using `any` as key type from the start or maintaining two maps.
- **Severity:** Low (Phase 2 concern, but affects Phase 1 design)
- **Component:** `context.go`
- **Action:** Keep `map[string]any` for user-facing `Set/Get`. Add separate `map[any]any` for internal type-keyed storage in Phase 2.

### `[DECISION]` D-001: Router — handle trailing slashes
- **Description:** Should `/users` and `/users/` match the same route? Options:
  1. Strict: different routes (Gin behavior)
  2. Redirect: 301 from `/users/` to `/users` (common in REST)
  3. Config: `StrictRouting` option (Fiber behavior)
- **Decision:** Go with option 3 (configurable). Default: redirect mode.
- **Component:** `router.go`, `config.go`

### `[DECISION]` D-002: 404 vs 405 differentiation
- **Description:** When a path exists but the method doesn't match, return 405 Method Not Allowed with `Allow` header (RFC 7231). Need router to check across all method trees.
- **Decision:** Implement proper 405 with Allow header.
- **Component:** `router.go`, `kruda.go`

### `[CONCERN]` C-003: sync.Pool and GC pressure
- **Description:** sync.Pool objects may be collected by GC between requests. Under low traffic, this means we lose the pooling benefit. Not a real issue under normal load, but worth noting for benchmarks.
- **Severity:** Low
- **Component:** `context.go`
- **Action:** No action needed. sync.Pool is the standard Go approach. Monitor in Phase 3 benchmarks.

### `[DECISION]` D-003: Transport Handler signature deviation from spec
- **Description:** Spec Section 3.1 shows `ServeKruda(c *Ctx)` but implementation uses `ServeKruda(w ResponseWriter, r Request)`. The spec signature would create a circular import (`transport` → `kruda.Ctx` → `transport`).
- **Decision:** Keep current signature `ServeKruda(w ResponseWriter, r Request)`. App wraps w/r into Ctx internally. This is the correct Go package design.
- **Component:** `transport/transport.go`

### `[DECISION]` D-004: Built-in middleware in Phase 1 (not Phase 4)
- **Description:** Spec Section 22 Phase 4 lists built-in middleware (logger, recovery, cors, requestid). However, a framework without basic middleware is unusable for demos.
- **Decision:** Move built-in middleware to Phase 1. Phase 4 focuses on DI, auto CRUD, contrib middleware.
- **Component:** `middleware/`

### `[CONCERN]` C-004: Context.File() and Cookie() use net/http type assertion
- **Description:** `File()` and `Cookie()` cast `RawRequest()` to `*http.Request`, breaking transport abstraction. When Netpoll transport is added (Phase 3), these need Netpoll-specific implementations.
- **Severity:** Low (Phase 3 concern)
- **Component:** `context.go`
- **Action:** For Phase 1, net/http-only is fine. Refactor in Phase 3 to use transport-agnostic cookie/file interfaces.

---

## Resolved Issues

### `[BUG]` C-001: Transport timeout wiring ✅ Fixed
- **Description:** `transport/nethttp.go` had placeholder timeout logic with `(1 << 63) - 1`. Config used `time.Duration` but NetHTTPConfig used `int64`.
- **Resolution:** Changed NetHTTPConfig to use `time.Duration`. Timeout now correctly wired to `http.Server`.
- **Resolved:** 2026-02-22

---

## Template

```markdown
### `[LABEL]` ID: Short title
- **Description:** What's the issue?
- **Severity:** Critical / High / Medium / Low
- **Component:** Which file(s)?
- **Reproduce:** Steps to reproduce (for bugs)
- **Action:** What needs to happen?
- **Resolved:** Date + commit (when fixed)
```
