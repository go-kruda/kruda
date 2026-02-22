# Phase 2 — Type System & Validation: Tasks

> To be detailed when Phase 1 is complete.

## Components

- [ ] `handler.go` — Full generic typed handlers with pre-compiled parser
- [ ] `bind.go` — Struct tag-based binding (param, query, json, default)
- [ ] `validation.go` — Pre-compiled validators, structured errors, custom validators
  - [ ] `ValidationError` type with `[]FieldError` — implements `error` + `json.Marshaler`
  - [ ] `FieldError` struct: field, rule, param, message, value
  - [ ] Auto-generate human-readable messages from rule+param
  - [ ] Support `message:"..."` struct tag for custom/i18n messages
  - [ ] `app.Validator().Messages()` for global message overrides
  - [ ] Default ErrorHandler integration: return 422 with structured JSON
  - [ ] Custom validator registration: `app.Validator().Register("thai_phone", fn)`
- [ ] `json/sonic.go` — Sonic JSON adapter with build tags
- [ ] `json/std.go` — encoding/json fallback
- [ ] `lifecycle.go` — Full lifecycle hooks (OnParse, Provide/Need)
- [ ] `concurrent.go` — Parallel(), Race(), Each() helpers
- [ ] `openapi.go` — OpenAPI 3.1 spec generation from typed handlers
  - [ ] Extract request/response schemas from `C[T]` type params
  - [ ] Map validation tags → JSON Schema constraints
  - [ ] Route metadata: method, path, params, query, description, tags
  - [ ] `kruda.OpenAPISpec()` handler to serve pre-built JSON
  - [ ] `WithOpenAPIInfo(title, version, description)` config
  - [ ] `WithOpenAPITag(name, description)` for route grouping
  - [ ] `WithDescription()`, `WithTags()` per-route options
  - [ ] Build spec at `Listen()` time — zero runtime overhead
- [ ] `bind.go` (extend) — File upload with struct tag validation
  - [ ] `FileUpload` struct: Name, Size, ContentType, Header, `Open()`
  - [ ] `form:"fieldname"` tag for multipart binding
  - [ ] Validation: `max_size=5mb`, `mime=image/*`
  - [ ] Multiple files: `[]*kruda.FileUpload`
  - [ ] Integration with input parsing pipeline
- [ ] `context.go` (extend) — SSE helper
  - [ ] `c.SSE(fn func(*SSEStream) error)` method
  - [ ] `SSEStream` type: `Event()`, `Data()`, `Comment()`, `Retry()`
  - [ ] Auto headers + flush + disconnect detection
- [ ] Tests for all components
  - [ ] `validation_test.go` — structured errors, message generation, i18n overrides, custom validators
  - [ ] `openapi_test.go` — spec generation, schema mapping, validation tag conversion
  - [ ] `bind_test.go` — file upload binding, size/mime validation
  - [ ] `sse_test.go` — SSE stream formatting, disconnect handling
