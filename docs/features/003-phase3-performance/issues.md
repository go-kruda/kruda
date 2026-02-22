# Phase 3 — Netpoll & Performance: Issues

> Track bugs, concerns, and review notes here.
> Use labels: `[BUG]` `[CONCERN]` `[REVIEW]` `[DECISION]`

---

## Open Issues

> Issues identified during Phase 1 code review that impact performance.
> To be addressed during Phase 3 optimization work.

### `[REVIEW]` P3-001: QueryParam re-parses URL query string on every call
- **Description:** `netHTTPRequest.QueryParam()` calls `r.r.URL.Query().Get(key)` which re-parses the entire raw query string into a new `url.Values` map on every invocation. The `Ctx.query` map caches results per key, so repeated lookups of the same key are fine, but each *new* key triggers a full re-parse. High-cardinality query strings (e.g., filter APIs) will allocate repeatedly on the hot path.
- **Severity:** Medium
- **Component:** `transport/nethttp.go:137-139`
- **Action:** Cache `url.Values` on first parse in `netHTTPRequest`:
  ```go
  type netHTTPRequest struct {
      queryValues url.Values
      queryParsed bool
  }
  func (r *netHTTPRequest) QueryParam(key string) string {
      if !r.queryParsed {
          r.queryValues = r.r.URL.Query()
          r.queryParsed = true
      }
      return r.queryValues.Get(key)
  }
  ```

### `[REVIEW]` P3-002: Query cache does not cache misses — repeated miss = repeated parse
- **Description:** `Ctx.Query()` only caches when `QueryParam` returns a non-empty value. If a key is absent, the next call for the same key will call `QueryParam` again (triggering a full query string re-parse per P3-001). This is O(n) per miss per request on the hot path.
- **Severity:** Low
- **Component:** `context.go:141-156`
- **Action:** Cache misses as empty string with a `queryParsed bool` flag, or parse all query params at once on first access. Combine with P3-001 fix.

### `[REVIEW]` P3-003: Compile() is a no-op — no AOT router optimization
- **Description:** `Router.Compile()` only sets `r.compiled = true`. No actual optimization is performed: no child sorting by frequency, no tree flattening, no pre-computed indices optimization. The spec mentions "AOT-compiled at startup" as a feature.
- **Severity:** Medium
- **Component:** `router.go:44-46`
- **Action:** Implement AOT optimizations in Phase 3:
  - Sort children by frequency (most-hit routes first)
  - Flatten single-child chains for cache locality
  - Pre-compute common prefix lengths
  - Consider frozen-array children instead of slices

### `[REVIEW]` P3-004: sync.Pool may retain oversized Ctx objects
- **Description:** After handling a request with many params/headers/query keys, the `Ctx` is returned to the pool with maps at their peak capacity. `cleanup()` calls `clear()` on maps (correct) but does not shrink them. Over time, all pooled contexts grow to the size of the largest request seen, increasing memory footprint under mixed traffic.
- **Severity:** Low
- **Component:** `context.go:108-115`
- **Action:** In `cleanup()`, check if any map exceeds a threshold (e.g., 2x initial capacity) and re-allocate to initial size. Example:
  ```go
  if len(c.params) > 16 {
      c.params = make(map[string]string, 8)
  } else {
      clear(c.params)
  }
  ```

### `[REVIEW]` P3-005: internal/bytesconv unused — zero-copy conversions not yet applied
- **Description:** `UnsafeString` and `UnsafeBytes` in `internal/bytesconv/` are implemented and tested but not imported anywhere. The router, context, and transport still use standard `string()` / `[]byte()` conversions which allocate on every call. Key hot-path locations where zero-copy would help: router path matching, header lookups, param extraction.
- **Severity:** Medium
- **Component:** `internal/bytesconv/bytesconv.go`, `router.go`, `context.go`
- **Action:** Audit hot paths and replace safe-to-convert locations with `UnsafeString`/`UnsafeBytes`. Candidates:
  - `router.find()` path segment comparisons
  - `Ctx.Param()` / `Ctx.Query()` value returns (if backed by byte buffers in Netpoll)
  - Transport request body → string conversion

### `[REVIEW]` P3-006: findAllowedMethods allocates on error path (405)
- **Description:** `findAllowedMethods()` creates a `tmpParams` map and a `[]string` slice with `strings.Join` on every 405 response. While 405s are not the hot path, a misconfigured client hammering the wrong method could cause allocation pressure.
- **Severity:** Low
- **Component:** `router.go:479-493`
- **Action:** Use a pre-allocated buffer or `strings.Builder`. Consider caching allowed methods per path at `Compile()` time since the route tree is frozen.

---

## Resolved Issues

_(Move issues here when fixed)_

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
