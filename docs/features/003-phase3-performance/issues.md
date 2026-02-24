# Phase 3 — Netpoll & Performance: Issues

> Track bugs, concerns, and review notes here.
> Use labels: `[BUG]` `[CONCERN]` `[REVIEW]` `[DECISION]`

---

## Open Issues

_(None — all identified issues resolved)_

---

## Resolved Issues (Code Review Round 1)

### `[BUG]` R1-01: Data race in NetpollTransport.eventLoop (P0)
- **Description:** `ListenAndServe` wrote `t.eventLoop` in one goroutine while `Shutdown` read it from another without synchronization. Race detector flagged this in integration tests.
- **Severity:** Critical
- **Component:** `transport/netpoll.go`
- **Action:** Added `sync.Mutex` to `NetpollTransport`. `ListenAndServe` locks before writing `eventLoop`, `Shutdown` locks before reading it.
- **Status:** ✅ Fixed

### `[BUG]` R1-02: QUIC transport missing body size limit (P1)
- **Description:** `quicRequest.Body()` used `io.ReadAll` without any size limit. A malicious client could send an arbitrarily large body and cause OOM, unlike nethttp and netpoll transports which enforce `MaxBodySize`.
- **Severity:** High
- **Component:** `transport/quic/quic.go`
- **Action:** Added `maxBody` field to `quicRequest` and `Config.MaxBodySize`. `Body()` now uses `io.LimitReader` and rejects bodies exceeding the limit.
- **Status:** ✅ Fixed

### `[BUG]` R1-03: QUIC RemoteAddr includes port — inconsistent with other transports (P2)
- **Description:** `quicRequest.RemoteAddr()` returned `r.r.RemoteAddr` raw (e.g. `192.168.1.1:12345`), while nethttp and netpoll strip the port to return bare IP. This caused inconsistent behavior for IP-based logic (rate limiting, logging).
- **Severity:** Medium
- **Component:** `transport/quic/quic.go`
- **Action:** Used `net.SplitHostPort` to strip port, matching nethttp/netpoll behavior.
- **Status:** ✅ Fixed

## Resolved Issues (from Phase 1 review — addressed in Phase 3)

### `[REVIEW]` P3-001: QueryParam re-parses URL query string on every call
- **Description:** `netHTTPRequest.QueryParam()` called `r.r.URL.Query()` on every invocation, re-parsing the entire query string each time.
- **Severity:** Medium
- **Component:** `transport/nethttp.go`
- **Action:** Added `queryVals url.Values` and `queryDone bool` fields. Query string is parsed once on first access and cached.
- **Status:** ✅ Fixed

### `[REVIEW]` P3-002: Query cache does not cache misses
- **Description:** `Ctx.Query()` only cached non-empty values. Absent keys triggered re-parse on every call.
- **Severity:** Low
- **Component:** `context.go`
- **Action:** Removed redundant `Ctx.query` map entirely (R4-02). `Query()` now delegates directly to transport's cached `QueryParam()`.
- **Status:** ✅ Fixed (removed in Phase 2 R4-02)

### `[REVIEW]` P3-003: Compile() is a no-op — no AOT router optimization
- **Description:** `Router.Compile()` only set a boolean flag with no actual optimization.
- **Severity:** Medium
- **Component:** `router.go`
- **Action:** Implemented three AOT optimizations: (1) sort children by hit frequency, (2) flatten single-child static chains, (3) build allowed methods cache for static paths.
- **Status:** ✅ Fixed

### `[REVIEW]` P3-004: sync.Pool may retain oversized Ctx objects
- **Description:** After handling requests with many params/headers, pooled Ctx maps grew monotonically and never shrank.
- **Severity:** Low
- **Component:** `context.go`
- **Action:** Added shrink thresholds in `cleanup()`. Maps exceeding 4x initial capacity are reallocated to initial size instead of just cleared.
- **Status:** ✅ Fixed

### `[REVIEW]` P3-005: internal/bytesconv unused — zero-copy conversions not applied
- **Description:** `UnsafeString`/`UnsafeBytes` were implemented but not used anywhere.
- **Severity:** Medium
- **Component:** `internal/bytesconv/`, `transport/netpoll.go`
- **Action:** Applied `bytesconv.UnsafeString` in netpoll HTTP parser for zero-copy request line and header parsing. Not applicable to core (works with strings from transport layer, no byte buffers to convert).
- **Status:** ✅ Fixed (netpoll), N/A (core)

### `[REVIEW]` P3-006: findAllowedMethods allocates on error path (405)
- **Description:** `findAllowedMethods()` allocated a map and string slice on every 405 response.
- **Severity:** Low
- **Component:** `router.go`
- **Action:** Two-tier approach: (1) `Compile()` pre-builds `allowedMethodsCache` for static paths (zero alloc), (2) dynamic paths use `sync.Pool` for tmpParams and `strings.Builder` instead of `strings.Join`.
- **Status:** ✅ Fixed

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
