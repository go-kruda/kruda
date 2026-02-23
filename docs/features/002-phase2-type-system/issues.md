# Phase 2 — Type System & Validation: Issues

> Track bugs, concerns, and review notes here.

## Resolved Issues (Code Review Round 4)

### `[BUG]` R4-01: SSE fails in production — netHTTPResponseWriter missing Flusher (P0)
- **Description:** `Ctx.SSE()` asserts `c.writer.(http.Flusher)` but `netHTTPResponseWriter` didn't implement `http.Flusher`. The assertion always failed with the real net/http transport, returning "SSE requires a transport that supports flushing". Tests passed because mocks implemented `Flusher` directly.
- **Severity:** Critical
- **Component:** `transport/nethttp.go`
- **Action:** Added `Flush()` method to `netHTTPResponseWriter` that delegates to the underlying `http.ResponseWriter` if it supports `http.Flusher`.
- **Status:** ✅ Fixed

### `[PERF]` R4-02: Ctx.query map is redundant cache (P2)
- **Description:** `Ctx` had a `query map[string]string` that cached individual query param lookups. But the transport layer (`netHTTPRequest.queryVals`) already parses and caches the full query string on first access. The Ctx-level cache added: 1 extra map allocation per pooled Ctx, 1 `clear()` call per request reset, and per-key map writes on every `Query()` call — all for zero benefit.
- **Severity:** Medium
- **Component:** `context.go`
- **Action:** Removed `query` field from `Ctx` struct, `newCtx` allocation, `reset()` clear, and cache writes in `Query()`. `Query()` now delegates directly to `c.request.QueryParam()`.
- **Status:** ✅ Fixed

### `[PERF]` R4-03: parseMultipart opens and closes file descriptor unnecessarily (P3)
- **Description:** Single file upload binding called `r.FormFile(tag)` to check existence, immediately closed the returned file, then accessed the header from `r.MultipartForm.File[tag][0]`. The `FormFile` call opened a file descriptor (syscall) just to discard it.
- **Severity:** Low
- **Component:** `bind.go`
- **Action:** Replaced `FormFile()` + `Close()` with direct `r.MultipartForm.File[tag]` slice check. Saves 2 syscalls (open + close) per file upload field.
- **Status:** ✅ Fixed

### `[DX]` R4-04: No typed handler support on Group (P2)
- **Description:** `Group` only had untyped route methods (`Get`, `Post`, etc.). There was no way to register typed handlers `C[T]` with auto-parse/validate on groups. Users had to manually call `buildTypedHandler` or register on `App` with full paths, defeating the purpose of groups.
- **Severity:** Medium
- **Component:** `handler.go`
- **Action:** Added package-level generic functions: `GroupGet[In, Out]`, `GroupPost`, `GroupPut`, `GroupDelete`, `GroupPatch`. They build the full path via `joinPath` for correct OpenAPI metadata and delegate to the group's route registration (inheriting group middleware).
- **Status:** ✅ Fixed

## Resolved Issues (Code Review Round 3)

### `[BUG]` R3-01: Listen() double-compile trick is fragile (P2)
- **Description:** `Listen()` called `Compile()`, then registered the OpenAPI route, reset `compiled=false`, and called `Compile()` again. Calling `Listen()` twice (e.g. in tests) would panic on duplicate route registration.
- **Severity:** Medium
- **Component:** `kruda.go`
- **Action:** Moved OpenAPI route registration before the first `Compile()` call. Single compile, no flag bypass.
- **Status:** ✅ Fixed

### `[BUG]` R3-02: validateGT/GTE/LT/LTE use float64 cast for int/uint (P3)
- **Description:** `validateMin`/`validateMax` had integer comparison paths to avoid `float64` precision loss for large int64/uint64 values, but `gt`/`gte`/`lt`/`lte` still cast everything to `float64`. Values near `math.MaxInt64` would compare incorrectly.
- **Severity:** Low
- **Component:** `validation.go`
- **Action:** Applied the same `ParseInt`/`ParseUint` fast-path pattern from `validateMin`/`validateMax` to all four functions.
- **Status:** ✅ Fixed

### `[DESIGN]` R3-03: ValidationError.MarshalJSON uses hardcoded encoding/json (P3)
- **Description:** `MarshalJSON()` called `encoding/json.Marshal()` directly, ignoring the build-tag-selected JSON engine. When Sonic was active, validation error responses were still encoded with stdlib — inconsistent and slower.
- **Severity:** Low
- **Component:** `validation.go`
- **Action:** Replaced `json.Marshal` with `krudajson.Marshal` which respects the build-tag selection (sonic vs std).
- **Status:** ✅ Fixed

### `[BUG]` R3-04: SSE checks http.Flusher after writing headers (P2)
- **Description:** `Ctx.SSE()` called `writeHeaders()` and `WriteHeader(200)` before checking if the writer supports `http.Flusher`. If flusher was unsupported, the error response couldn't be written because `responded=true` was already set.
- **Severity:** Medium
- **Component:** `context.go`
- **Action:** Moved the `http.Flusher` type assertion before any header writes. Error now returns cleanly before committing the response.
- **Status:** ✅ Fixed

### `[DESIGN]` R3-05: sendBytes/send silently swallow double-write (P3)
- **Description:** If a handler called `c.JSON()` twice, the second call returned `nil` — the caller had no way to know the response was discarded. Silent data loss is a debugging trap.
- **Severity:** Low
- **Component:** `context.go`
- **Action:** Added `ErrAlreadyResponded` sentinel error. `send()` and `sendBytes()` now return it on double-write. Callers can check with `errors.Is(err, ErrAlreadyResponded)`.
- **Status:** ✅ Fixed

### `[BUG]` R3-06: Config.HeaderLimit is dead config (P2)
- **Description:** `Config.HeaderLimit` (default 8KB) was declared and initialized but never passed to the transport. `http.Server.MaxHeaderBytes` was left at Go's default (1MB), making the config misleading.
- **Severity:** Medium
- **Component:** `kruda.go`, `transport/nethttp.go`
- **Action:** Added `MaxHeaderBytes` field to `NetHTTPConfig`. `kruda.go` now passes `HeaderLimit` to the transport. `ListenAndServe` sets `http.Server.MaxHeaderBytes` when configured.
- **Status:** ✅ Fixed

### `[DOC]` R3-07: Validator must be configured before registering typed routes (P3)
- **Description:** `buildTypedHandler` compiles validators at route registration time. If a user calls `app.Validator().Register(...)` after registering typed routes, the custom rules are silently ignored for those routes.
- **Severity:** Low
- **Component:** `handler.go`
- **Action:** Added doc comment to `buildTypedHandler` documenting the ordering requirement.
- **Status:** ✅ Fixed

## Resolved Issues (Code Review Round 2)

### `[BUG]` R2-01: parseMultipart calls RemoveAll() too early (P0)
- **Description:** `parseMultipart` in `bind.go` called `defer r.MultipartForm.RemoveAll()` immediately after parsing. This deleted temp files while `FileUpload.Header` still referenced them — `fu.Open()` would fail.
- **Severity:** Critical
- **Component:** `bind.go`, `context.go`
- **Action:** Removed `defer RemoveAll()` from `parseMultipart`. Added `multipartForm` field to `Ctx`. Store form reference during parse; call `RemoveAll()` in `cleanup()` when the request is done.
- **Status:** ✅ Fixed

### `[BUG]` R2-02: Ctx.Context() defaults to context.Background() (P0)
- **Description:** `Ctx.Context()` returned `context.Background()` when `c.ctx` was nil. This meant client disconnects wouldn't cancel the handler unless Timeout middleware was used.
- **Severity:** Critical
- **Component:** `context.go`
- **Action:** `reset()` now initializes `c.ctx` from `r.RawRequest().(*http.Request).Context()` so the request context propagates by default.
- **Status:** ✅ Fixed

### `[BUG]` R2-03: convertPath doesn't strip regex/optional params (P1)
- **Description:** `convertPath` converted `:id<[0-9]+>` → `{id<[0-9]+>}` and `:id?` → `{id?}` instead of stripping the constraint/optional marker.
- **Severity:** High
- **Component:** `openapi.go`
- **Action:** Updated `convertPath` to skip `<...>` regex constraints and `?` optional markers before closing `}`.
- **Status:** ✅ Fixed

### `[BUG]` R2-04: HSTSMaxAge is dead config (P1)
- **Description:** `SecurityConfig.HSTSMaxAge` was declared in config but never used in `writeHeaders()`. The `Strict-Transport-Security` header was never set.
- **Severity:** High
- **Component:** `context.go`, `config.go`
- **Action:** Added HSTS header writing in `writeHeaders()` when `sec.HSTSMaxAge > 0`.
- **Status:** ✅ Fixed

### `[DESIGN]` R2-05: FileUpload.Open() returns io.Reader instead of io.ReadCloser (P2)
- **Description:** `multipart.FileHeader.Open()` returns `multipart.File` which implements `io.ReadSeekCloser`. Returning `io.Reader` hid the `Close()` method, risking file descriptor leaks for large files stored on disk.
- **Severity:** Medium
- **Component:** `upload.go`
- **Action:** Changed return type to `io.ReadCloser`. Updated doc comment to instruct callers to close.
- **Status:** ✅ Fixed

### `[DESIGN]` R2-06: generateSchema name collision across packages (P2)
- **Description:** `generateSchema` uses `t.Name()` (short name) as the component key. Structs with the same name from different packages (e.g. `user.CreateReq` vs `product.CreateReq`) would collide.
- **Severity:** Medium
- **Component:** `openapi.go`
- **Action:** Added documentation noting the limitation. A proper fix (qualified names or collision detection with suffix) deferred to a future iteration since it requires changing the internal schema generation API.
- **Status:** ⚠️ Documented — deferred

### `[DESIGN]` R2-07: Group missing Options/Head/All methods (P2)
- **Description:** `App` had `Options()`, `Head()`, `All()` but `Group` only had GET/POST/PUT/DELETE/PATCH. Users couldn't register OPTIONS routes in groups (e.g. custom CORS).
- **Severity:** Medium
- **Component:** `group.go`
- **Action:** Added `Options()`, `Head()`, `All()` methods to `Group`, matching `App`'s API surface.
- **Status:** ✅ Fixed

### `[PERF]` R2-08: Ctx.Header() cache is case-sensitive (P3)
- **Description:** Header cache key was case-sensitive but HTTP headers are case-insensitive. `c.Header("Content-Type")` and `c.Header("content-type")` would miss each other's cache entries.
- **Severity:** Low
- **Component:** `context.go`
- **Action:** Normalized cache key with `http.CanonicalHeaderKey(name)`.
- **Status:** ✅ Fixed

### `[DOC]` R2-09: handleError ValidationError unwrapping not documented
- **Description:** Custom `ErrorHandler` receives `*KrudaError` wrapping `*ValidationError`. Accessing the validation details requires `errors.As(ke.Unwrap(), &ve)` which isn't intuitive.
- **Severity:** Low
- **Component:** `kruda.go`
- **Action:** Added detailed doc comment to `handleError` showing the unwrap pattern.
- **Status:** ✅ Fixed

### `[DESIGN]` R2-10: Listen() registers OpenAPI route after Compile (informational)
- **Description:** `Listen()` registers the OpenAPI route after `Compile()`, then resets `compiled` flag and re-compiles. This works but bypasses the internal guard from within the package.
- **Severity:** Low
- **Component:** `kruda.go`
- **Action:** No code change — current approach is correct. The OpenAPI spec must be built after all routes are registered (to collect metadata), so post-Compile registration is intentional. The `compiled` flag reset is safe since it's done atomically within `Listen()`.
- **Status:** ℹ️ Acknowledged — no change needed

## Template

```markdown
### `[LABEL]` ID: Short title
- **Description:** What's the issue?
- **Severity:** Critical / High / Medium / Low
- **Component:** Which file(s)?
- **Action:** What needs to happen?
- **Status:** ✅ Fixed / ⚠️ Deferred / ℹ️ Acknowledged
```
