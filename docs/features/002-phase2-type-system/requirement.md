# Phase 2 — Type System & Validation: Requirements

> **Goal:** Typed handlers must feel magical — parse, validate, and deliver typed data in one step.
> **Timeline:** Week 4-6
> **Spec Reference:** Sections 7, 8, 10
> **Depends on:** Phase 1 complete

## Milestone

```go
kruda.Post[CreateUserReq, UserRes](app, "/users", func(c *kruda.C[CreateUserReq]) (*UserRes, error) {
    return svc.Create(c.In)  // c.In = CreateUserReq, already parsed + validated
})
```

## Components

| # | Component | File(s) | Est. Days | Priority |
|---|-----------|---------|-----------|----------|
| 1 | Generic Typed Handlers | `handler.go` | 4 | 🔴 |
| 2 | Input Binding | `bind.go` | 3 | 🔴 |
| 3 | Validation Engine | `validation.go` | 4 | 🔴 |
| 4 | Sonic JSON | `json/sonic.go` | 1 | 🔴 |
| 5 | Lifecycle Hooks (full) | `lifecycle.go` | 2 | 🟡 |
| 6 | Concurrency Helpers | `concurrent.go` | 2 | 🟡 |
| 7 | OpenAPI 3.1 Generation | `openapi.go` | 4 | 🔴 |
| 8 | File Upload Binding | `bind.go` (extend) | 2 | 🔴 |
| 9 | SSE Helper | `context.go` (extend) | 1 | 🟡 |

## Key Requirements

### Typed Handler — `C[T]`
- `C[T any]` struct: embeds `*Ctx`, has `In T` field
- Generic functions: `Get[In, Out]`, `Post[In, Out]`, `Put[In, Out]`, `Delete[In, Out]`
- Pre-compiled parser per type at registration time
- Zero reflection at request time

### Input Binding — Struct Tags
- `param:"id"` → from path parameter
- `query:"page"` → from query string
- `json:"name"` → from request body
- `default:"1"` → default value if missing
- Mixed: a single struct can have param + query + body fields
- Pre-compiled `inputParser` per type (built once at route registration)

### Validation
- Tag: `validate:"required,email,min=2,max=100"`
- Pre-compiled validators from struct tags at registration
- Structured error response with field names
- Custom validators: `app.Validator().Register("thai_phone", fn)`
- Compatible with `go-playground/validator` tag syntax

### Structured Validation Errors (NEW — competitive advantage)

Kruda validation errors must be **frontend-ready** out of the box. Unlike go-playground/validator
which returns cryptic error strings, Kruda returns structured JSON that frontend devs can use directly.

**Response format:**
```json
{
  "code": 422,
  "message": "Validation failed",
  "errors": [
    {
      "field": "email",
      "rule": "required",
      "param": "",
      "message": "Email is required",
      "value": ""
    },
    {
      "field": "age",
      "rule": "min",
      "param": "18",
      "message": "Age must be at least 18",
      "value": "15"
    }
  ]
}
```

**Requirements:**
- `ValidationError` type with `[]FieldError` slice
- Each `FieldError`: field name, rule name, rule param, human-readable message, rejected value
- Auto-generated messages from rule + param (e.g. `min=18` → "must be at least 18")
- Custom messages per field: `message:"Email ต้องกรอก"` struct tag
- i18n-ready: message templates overridable via `app.Validator().Messages(map[string]string{...})`
- HTTP status: 422 Unprocessable Entity (not 400)
- Implements `error` interface for seamless error handler integration
- `ValidationError` implements `json.Marshaler` for clean output

**Comparison with competitors:**
| Feature | Gin | Fiber | Kruda |
|---------|-----|-------|-------|
| Structured errors | ❌ manual | ❌ manual | ✅ built-in |
| Field names in errors | ❌ | ❌ | ✅ |
| i18n messages | ❌ | ❌ | ✅ |
| Custom message per field | ❌ | ❌ | ✅ struct tag |
| Frontend-ready JSON | ❌ | ❌ | ✅ |

### Sonic JSON
- Pluggable JSON engine via `JSONEncoder`/`JSONDecoder` interfaces
- `json/sonic.go` — Sonic adapter (build tag: `!no_sonic`)
- `json/std.go` — encoding/json fallback (CGO_ENABLED=0)

### OpenAPI 3.1 Auto-generation (NEW — core, not contrib)

Auto-generate OpenAPI spec from Go types at startup. This is core because typed handlers already have all the type info — just need to extract it.

```go
// Auto-registered when typed handlers are used
app.Get("/openapi.json", kruda.OpenAPISpec())  // serves OpenAPI 3.1 JSON

// Optional: Swagger UI (this part is contrib — serves static HTML)
// import "github.com/go-kruda/kruda/contrib/swagger"
// app.Get("/docs/*", swagger.UI("/openapi.json"))
```

**Requirements:**
- OpenAPI 3.1 (not 3.0) — compatible with JSON Schema draft 2020-12
- Auto-extract from typed handlers: `C[T]` → request schema, return type → response schema
- Route info: method, path, params, query, description
- Validation tags → schema constraints: `min=2` → `minimum: 2`, `required` → `required: [...]`
- Output: JSON at configurable path (default `/openapi.json`)
- Spec built once at `Listen()` time (same as router compile)
- `WithOpenAPIInfo(title, version, description)` config option
- `WithOpenAPITag(name, description)` for grouping routes
- Per-route: `kruda.WithDescription("Create a user")`, `kruda.WithTags("users")`
- Zero runtime overhead: spec is pre-built, served as static JSON

**What stays in core vs contrib:**
- **Core:** `openapi.go` — spec generation from types (no deps, uses `encoding/json`)
- **Contrib:** `contrib/swagger/` — Swagger UI HTML (static files, optional)

### File Upload with Struct Tag Validation (NEW — core)

Multipart file upload with validation via struct tags:

```go
type UploadReq struct {
    Avatar *kruda.FileUpload `form:"avatar" validate:"required,max_size=5mb,mime=image/*"`
    Name   string            `form:"name" validate:"required"`
}

app.Post("/upload", func(c kruda.C[UploadReq]) error {
    file := c.In.Avatar
    // file.Name, file.Size, file.ContentType, file.Open() io.Reader
    return c.JSON(200, kruda.Map{"uploaded": file.Name})
})
```

**Requirements:**
- `FileUpload` struct: Name, Size, ContentType, Header, `Open() (io.Reader, error)`
- Struct tag binding: `form:"avatar"` → from multipart form field
- Validation tags: `max_size=5mb`, `mime=image/*`, `required`
- Multiple files: `[]*kruda.FileUpload` for multi-file upload
- Uses `mime/multipart` from stdlib — no external deps
- Body limit still applies (Config.BodyLimit)
- Integrates with input parsing pipeline: form fields + file fields in same struct

### SSE (Server-Sent Events) Helper (NEW — core)

Small helper on Ctx for clean SSE streaming:

```go
app.Get("/events", func(c *kruda.Ctx) error {
    return c.SSE(func(stream *kruda.SSEStream) error {
        for msg := range channel {
            if err := stream.Event("message", msg); err != nil {
                return err // client disconnected
            }
        }
        return nil
    })
})
```

**Requirements:**
- `c.SSE(fn)` sets correct headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`
- `SSEStream` type with methods: `Event(name, data)`, `Data(data)`, `Comment(text)`, `Retry(ms)`
- Auto-flush after each event
- Detect client disconnect via `c.Context().Done()`
- Small: ~50 lines of code, wraps `c.Stream()` with SSE formatting
