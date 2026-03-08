# 🔥 Kruda Framework — Technical Specification

> **"Fast by default. Type-safe by design. Standard by nature."**
>
> Version: 1.0.0-draft
> Language: Go (1.25+, recommended 1.26)
> Author: Tiger
> Last Updated: February 2026

---

## Table of Contents

1. [Vision & Philosophy](#1-vision--philosophy)
2. [Architecture Overview](#2-architecture-overview)
3. [Transport Layer](#3-transport-layer)
4. [Core Engine](#4-core-engine)
5. [Router](#5-router)
6. [Context](#6-context)
7. [Type System (Generics)](#7-type-system-generics)
8. [Validation](#8-validation)
9. [Error Handling](#9-error-handling)
10. [Middleware & Lifecycle](#10-middleware--lifecycle)
11. [Plugin System](#11-plugin-system)
12. [Dependency Injection](#12-dependency-injection)
13. [Auto CRUD (Resource)](#13-auto-crud-resource)
14. [Concurrency Helpers](#14-concurrency-helpers)
15. [Security](#15-security)
16. [Auto OpenAPI / Swagger](#16-auto-openapi--swagger)
17. [Testing Utilities](#17-testing-utilities)
18. [JSON Engine](#18-json-engine)
19. [Performance Techniques](#19-performance-techniques)
20. [CLI Tool](#20-cli-tool)
21. [Project Structure](#21-project-structure)
22. [Roadmap & Phases](#22-roadmap--phases)
23. [Benchmarks & Targets](#23-benchmarks--targets)
24. [Competitive Analysis](#24-competitive-analysis)
25. [API Quick Reference](#25-api-quick-reference)
26. [Implementation Priorities](#26-implementation-priorities)
27. [Rust Secret Weapon](#27-rust-secret-weapon)
28. [Competitive Strategy](#28-competitive-strategy)
29. [AI-Friendly Developer Experience](#29-ai-friendly-developer-experience)
30. [MCP Server Plugin](#30-mcp-server-plugin)

---

## 1. Vision & Philosophy

### 1.1 What is Kruda?

Kruda (ครุฑ) is a high-performance Go web framework that combines the speed of Fiber with the safety of net/http standard, while dramatically reducing boilerplate code through Go generics.

### 1.2 Core Principles

1. **Fast by default** — Uses CloudWeGo fasthttp (epoll-based) as default transport, zero-allocation hot paths, Sonic JSON (SIMD-accelerated)
2. **Type-safe by design** — Go generics for typed handlers, compile-time type checking, zero reflection at runtime
3. **Standard by nature** — Switchable between fasthttp and net/http, compatible with Go ecosystem
4. **Secure by default** — No context reuse bugs, built-in security headers, fail-fast UUID generation
5. **Write less, do more** — 60-70% less boilerplate than Fiber/Gin while maintaining same or better performance

### 1.3 Target Positioning

- **NOT** competing with Fiber on raw benchmark numbers head-to-head
- **IS** the "Type-safe Go framework with auto-everything" — type-safe handlers, auto-validation, auto-error-mapping, auto-OpenAPI, auto-CRUD
- Think of it as: "Go version of Elysia/tRPC" — maximum DX without sacrificing performance

### 1.4 Name Origin

ครุฑ (Garuda) — Symbol of power and speed in Thai mythology. Fast, strong, memorable. Easy to pronounce in both Thai and international contexts.

```
go get github.com/go-kruda/kruda
```

---

## 2. Architecture Overview

### 2.1 Layer Diagram

```
┌──────────────────────────────────────────────────────┐
│                    User API Layer                      │
│   app.Get() / app.Post() / Typed Handlers / Resource  │
├──────────────────────────────────────────────────────┤
│                 Middleware Pipeline                    │
│   onRequest → beforeHandle → handler → afterHandle    │
├──────────────────────────────────────────────────────┤
│                Radix Tree Router                      │
│   AOT-compiled, zero-alloc matching                   │
├──────────────────────────────────────────────────────┤
│               Context (sync.Pool)                     │
│   Zero-alloc, reusable, safe copy                     │
├──────────────────────────────────────────────────────┤
│            ┌──────────────────┐                       │
│            │ Transport Layer  │  ← Pluggable          │
│            │    Interface     │                       │
│            └────────┬────────┘                        │
│         ┌───────────┴───────────┐                     │
│    ┌────┴──────┐         ┌──────┴──────┐              │
│    │  fasthttp  │         │  net/http   │              │
│    │ (default) │         │ (fallback)  │              │
│    └───────────┘         └─────────────┘              │
│    epoll/kqueue           Go standard                 │
│    zero-copy I/O          HTTP/2, TLS                 │
│    worker pool            streaming                   │
└──────────────────────────────────────────────────────┘
```

### 2.2 Key Dependencies

| Dependency | Purpose | Why chosen |
|-----------|---------|------------|
| `github.com/cloudwego/fasthttp` | Default network transport | epoll-based, used by ByteDance in 50K+ microservices |
| `github.com/bytedance/sonic` | JSON serialization | SIMD-accelerated, 2-5x faster than encoding/json |
| Go standard `net/http` | Fallback transport | HTTP/2, TLS, streaming support |
| Go standard `crypto/rand` | UUID/token generation | Secure randomness, fail-fast on error |

### 2.3 Go Version Requirement

- **Minimum: Go 1.25** (required for generic type aliases)
- **Recommended: Go 1.26** (Green Tea GC, self-referential generics, cgo overhead -30%, stack-allocated slices)
- **Tested on: Go 1.25, 1.26**

> **Why Go 1.25 minimum?** The type system relies on generic type aliases (`type T[B any] = BodyCtx[B]`) which require Go 1.25+. Older versions cannot compile the core typed handler system.
>
> **Why Go 1.26 recommended?** The Green Tea GC significantly reduces tail latency under high concurrency, self-referential generics enable more complex typed patterns, and cgo overhead reduction directly benefits Sonic JSON performance.

---

## 3. Transport Layer

### 3.1 Design: Pluggable Transport Interface

The transport layer is abstracted behind an interface, allowing Kruda to switch between network implementations without changing any user-facing code.

```go
// transport.go
package kruda

import "context"

// Transport defines the network layer interface
type Transport interface {
    // ListenAndServe starts the server
    ListenAndServe(addr string, handler Handler) error
    // Shutdown gracefully shuts down the server
    Shutdown(ctx context.Context) error
}

// Handler is the core request handler interface
type Handler interface {
    ServeKruda(c *Ctx)
}
```

### 3.2 fasthttp Transport (Default)

**Why fasthttp over fasthttp:**
- fasthttp is a pure networking library (not HTTP framework) — cleaner separation
- epoll/kqueue based, non-blocking I/O
- Used in production at ByteDance with 50,000+ microservices
- Does not have fasthttp's context reuse safety issues
- Supports switching between fasthttp and Go net on demand

**Why fasthttp over raw net/http:**
- Better scheduling strategy for small requests
- Worker pool goroutine management
- Lower latency under high concurrency
- Zero-copy I/O where possible

**Implementation notes:**
- Default for Linux/macOS
- Auto-fallback to net/http on Windows (fasthttp doesn't support Windows)
- For requests > 1MB, recommend switching to net/http with streaming
- fasthttp uses LT (Level-Triggered) mode vs Go net's ET (Edge-Triggered) mode

```go
// Usage: select transport
app := kruda.New()                           // default: fasthttp
app := kruda.New(kruda.NetHTTP())             // use net/http
app := kruda.New(kruda.WithTransport(custom)) // custom transport
```

### 3.3 net/http Transport (Fallback)

Used when:
- TLS/HTTPS is needed (fasthttp TLS not yet mature)
- HTTP/2 is required
- Large request/response streaming (> 1MB)
- Windows environment
- Maximum ecosystem compatibility needed

### 3.4 Transport Selection Logic

```
User specifies transport? → Use specified
  ↓ No
Running on Windows? → Use net/http
  ↓ No
TLS configured? → Use net/http
  ↓ No
Use fasthttp (default)
```

---

## 4. Core Engine

### 4.1 App Structure

```go
// kruda.go
package kruda

type App struct {
    // Configuration
    config     Config
    
    // Router
    router     *Router
    
    // Middleware stack
    middleware []MiddlewareFunc
    
    // Lifecycle hooks
    hooks      Hooks
    
    // Error mapping
    errorMap   map[error]ErrorMapping
    
    // Transport
    transport  Transport
    
    // Resource registry (for OpenAPI generation)
    resources  []ResourceMeta
    
    // Dependency injection container
    container  *Container
    
    // Context pool
    ctxPool    sync.Pool
}

type Config struct {
    // Server
    ReadTimeout     time.Duration  // default: 30s
    WriteTimeout    time.Duration  // default: 30s
    IdleTimeout     time.Duration  // default: 120s
    BodyLimit       int            // default: 4MB (4 * 1024 * 1024)
    HeaderLimit     int            // default: 8KB (8 * 1024)
    
    // Transport
    Transport       Transport      // default: fasthttp
    
    // JSON engine
    JSONEncoder     JSONEncoder    // default: sonic
    JSONDecoder     JSONDecoder    // default: sonic
    
    // Security (all enabled by default)
    Security        SecurityConfig
    
    // Logging
    Logger          Logger         // default: slog-based
}
```

### 4.2 App Initialization

```go
func New(opts ...Option) *App {
    app := &App{
        config: defaultConfig(),
        router: newRouter(),
        errorMap: defaultErrorMap(),
    }
    
    for _, opt := range opts {
        opt(app)
    }
    
    // Initialize context pool
    app.ctxPool = sync.Pool{
        New: func() any {
            return newCtx(app)
        },
    }
    
    // Select transport
    if app.config.Transport == nil {
        if runtime.GOOS == "windows" {
            app.transport = newNetHTTPTransport(app.config)
        } else {
            app.transport = newfasthttpTransport(app.config)
        }
    }
    
    return app
}
```

### 4.3 Method Chaining API

All route registration methods return `*App` to enable chaining:

```go
app := kruda.New().
    Use(kruda.Logger(), kruda.CORS()).
    Get("/", homeHandler).
    Post("/users", createUser).
    Group("/api/v1").
        Use(kruda.JWT(secret)).
        Resource("/products", productService).
        Done(). // return to parent
    Listen(":3000")
```

---

## 5. Router

### 5.1 Radix Tree Router

**Requirements:**
- Zero allocation during route matching
- Support: static routes, parameterized (`:id`), wildcard (`*`), regex constraints
- AOT-compiled: route tree is built at startup, not modified at runtime
- O(log n) lookup time

### 5.2 Route Patterns

```
Static:     /users
Param:      /users/:id
Multi:      /users/:id/posts/:postId
Wildcard:   /files/*filepath
Regex:      /users/:id<[0-9]+>
Optional:   /users/:id?
```

### 5.3 Router Implementation

```go
// router.go
type Router struct {
    trees map[string]*node  // method → radix tree root
    
    // Pre-compiled param parsers (built at startup)
    paramParsers map[string]ParamParser
}

type node struct {
    path     string
    children []*node
    handler  HandlerFunc
    param    string      // parameter name if parameterized
    wildcard bool
    
    // Pre-allocated for zero-alloc matching
    indices  string      // first bytes of children for quick lookup
}

type ParamParser func(value string) (any, error)
```

### 5.4 Route Registration

```go
// Standard handlers
app.Get(path string, handler HandlerFunc, hooks ...HookConfig)
app.Post(path string, handler HandlerFunc, hooks ...HookConfig)
app.Put(path string, handler HandlerFunc, hooks ...HookConfig)
app.Delete(path string, handler HandlerFunc, hooks ...HookConfig)
app.Patch(path string, handler HandlerFunc, hooks ...HookConfig)
app.Options(path string, handler HandlerFunc, hooks ...HookConfig)
app.Head(path string, handler HandlerFunc, hooks ...HookConfig)
app.All(path string, handler HandlerFunc, hooks ...HookConfig)

// Typed handlers (generic) — unified C[T] for all methods
kruda.Get[In, Out](app, path, handler func(*C[In]) (*Out, error))
kruda.Post[In, Out](app, path, handler func(*C[In]) (*Out, error))
kruda.Put[In, Out](app, path, handler func(*C[In]) (*Out, error))
kruda.Delete[In, Out](app, path, handler func(*C[In]) (*Out, error))

// Full typed (custom method)
kruda.Handle[In, Out](app, method, path, handler)
```

### 5.5 Route Groups

```go
type Group struct {
    prefix     string
    app        *App
    middleware []MiddlewareFunc
    hooks      Hooks
}

func (g *Group) Get(path string, handler HandlerFunc) *Group
func (g *Group) Post(path string, handler HandlerFunc) *Group
func (g *Group) Use(middleware ...MiddlewareFunc) *Group
func (g *Group) Guard(middleware ...MiddlewareFunc) *Group  // alias for Use — reads as "protect with"
func (g *Group) Group(prefix string) *Group                 // nested group
func (g *Group) Resource(path string, service any) *Group   // auto CRUD
```

**Usage:**

```go
app.Group("/api/v1").
    Guard(kruda.JWT(secret)).
    Resource("/products", productSvc).
    Resource("/orders", orderSvc)

// Nested groups
app.Group("/api").
    Use(kruda.Logger()).
    Group("/v1").
        Guard(kruda.JWT(secret)).
        Resource("/users", userSvc).
    Group("/v2").
        Guard(kruda.JWT(secret), kruda.RoleRequired("admin")).
        Resource("/users", userSvcV2)
```

---

## 6. Context

### 6.1 Design Principles

**CRITICAL DIFFERENCE FROM FIBER:**

Fiber's `*fiber.Ctx` uses fasthttp's context pool, which causes:
1. String values returned from `c.Params()` are unsafe `[]byte→string` conversions that become invalid after handler returns
2. Passing `*fiber.Ctx` to goroutines causes data races
3. `c.Body()` data is reused from the pool — can be corrupted

**Kruda's approach:**
1. Context is pooled via `sync.Pool` for zero allocation
2. All string values are **proper copies** — safe to use anywhere
3. Body data is **copy-on-read** — safe to keep after handler returns
4. Context itself should not be passed to goroutines, but all extracted values are safe

### 6.2 Context Structure

```go
// context.go
type Ctx struct {
    app        *App
    
    // Request data (populated on init, safe copies)
    method     string
    path       string
    params     map[string]string  // pre-allocated map, reset per request
    query      map[string]string  // lazy parsed
    headers    map[string]string  // lazy parsed
    bodyBytes  []byte             // copy-on-read from request
    bodyParsed bool
    
    // Response
    status     int
    respHeaders map[string]string
    responded  bool
    
    // Internal
    routeIndex int               // current middleware index
    handlers   []HandlerFunc     // middleware + handler chain
    locals     map[string]any    // request-scoped values
    
    // Writer (transport-specific)
    writer     ResponseWriter
    
    // Timing
    startTime  time.Time
}
```

### 6.3 Context API

```go
// Request - all return safe, owned strings
func (c *Ctx) Method() string
func (c *Ctx) Path() string
func (c *Ctx) Param(name string) string           // path parameter
func (c *Ctx) ParamInt(name string) (int, error)   // parsed int param
func (c *Ctx) Query(name string, def ...string) string
func (c *Ctx) QueryInt(name string, def ...int) int
func (c *Ctx) Header(name string) string
func (c *Ctx) Cookie(name string) string
func (c *Ctx) IP() string
func (c *Ctx) BodyBytes() []byte                   // safe copy
func (c *Ctx) BodyString() string                  // safe copy
func (c *Ctx) Bind(v any) error                    // JSON/Form/XML auto-detect

// Response
func (c *Ctx) Status(code int) *Ctx               // chainable
func (c *Ctx) JSON(v any) error                    // uses Sonic
func (c *Ctx) Text(s string) error
func (c *Ctx) HTML(template string, data any) error
func (c *Ctx) File(path string) error
func (c *Ctx) Stream(reader io.Reader) error
func (c *Ctx) SSE(events <-chan Event) error
func (c *Ctx) Redirect(url string, code ...int) error
func (c *Ctx) SetHeader(key, value string) *Ctx
func (c *Ctx) SetCookie(cookie *Cookie) *Ctx
func (c *Ctx) NoContent() error                    // 204

// Locals (request-scoped key-value store)
func (c *Ctx) Set(key string, value any)
func (c *Ctx) Get(key string) any

// Generic getter with type safety
func Get[T any](c *Ctx, key string) T

// Flow control
func (c *Ctx) Next() error                         // call next middleware
func (c *Ctx) Latency() time.Duration

// Context for stdlib compatibility
func (c *Ctx) Context() context.Context
func (c *Ctx) SetContext(ctx context.Context)
```

### 6.4 Context Pool Management

```go
func (app *App) acquireCtx(w ResponseWriter, r Request) *Ctx {
    c := app.ctxPool.Get().(*Ctx)
    c.reset(w, r)
    return c
}

func (app *App) releaseCtx(c *Ctx) {
    c.cleanup()
    app.ctxPool.Put(c)
}

func (c *Ctx) reset(w ResponseWriter, r Request) {
    c.method = r.Method()
    c.path = r.Path()
    c.status = 200
    c.responded = false
    c.bodyParsed = false
    c.startTime = time.Now()
    c.writer = w
    // Reset maps without reallocating
    clear(c.params)
    clear(c.query)
    clear(c.headers)
    clear(c.locals)
}
```

---

## 7. Type System (Generics)

### 7.1 Overview

The type system is the **core differentiator** of Kruda. It uses Go generics to provide:
- Typed request body parsing
- Typed path parameter parsing
- Typed query parameter parsing
- Typed response
- All at **compile-time** — zero reflection at runtime

### 7.2 Unified Typed Context: `C[T]`

> **Design principle:** One type, one field, struct tags decide everything.
> No need to choose between `BodyCtx`, `ParamCtx`, `QueryCtx` — just use `C[T]`.

```go
// The only typed context you need
type C[T any] struct {
    *Ctx
    In T  // parsed, validated input — body, params, query, or any mix
}
```

**Struct tags tell Kruda where data comes from:**

```go
// Body only (POST/PUT) — fields with json tags come from request body
type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
}

// Path params only (GET) — fields with param tags come from URL
type GetUserReq struct {
    ID int `param:"id" validate:"gt=0"`
}

// Query only (GET list) — fields with query tags come from query string
type ListReq struct {
    Page   int    `query:"page"   default:"1"`
    Limit  int    `query:"limit"  default:"20"`
    Search string `query:"search"`
}

// Mixed: path + query (common pattern)
type GetUserPostsReq struct {
    UserID int    `param:"userId" validate:"gt=0"`  // from /users/:userId/posts
    Page   int    `query:"page"   default:"1"`      // from ?page=2
}

// Mixed: path + body (update pattern)
type UpdateUserReq struct {
    ID   int    `param:"id"   validate:"gt=0"`      // from /users/:id
    Name string `json:"name"  validate:"required"`   // from request body
}
```

**How it works internally:**
- At startup: Kruda inspects struct tags once, builds a parser per type
- At request: parser fills `In` from the right source (param/query/body) based on tags
- Zero reflection at request time — all pre-compiled

### 7.3 Generic Handler Registration

```go
// All methods use the same C[T] pattern:
func Post[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error)) {
    parser := buildParser[In]()  // pre-compile at registration time
    
    app.addRoute("POST", path, func(c *Ctx) error {
        var in In
        if err := parser.Parse(c, &in); err != nil {  // fills from json/param/query based on tags
            return err
        }
        if err := c.validate(in); err != nil {
            return NewValidationError(err)
        }
        
        tc := &C[In]{Ctx: c, In: in}
        result, err := handler(tc)
        if err != nil {
            return err  // auto-mapped by error handler
        }
        
        c.Status(201)  // POST default
        return c.JSON(result)
    })
}

// Same pattern for all HTTP methods
func Get[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error))
func Put[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error))
func Delete[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error))
func Patch[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error))
```

**Usage reads like English:**

```go
// "POST /users with CreateReq, respond with UserRes"
kruda.Post[CreateReq, UserRes](app, "/users", func(c *kruda.C[CreateReq]) (*UserRes, error) {
    return svc.Create(c.In)  // c.In = CreateReq, already parsed + validated
})

// "GET /users/:id with GetReq, respond with UserRes"
kruda.Get[GetReq, UserRes](app, "/users/:id", func(c *kruda.C[GetReq]) (*UserRes, error) {
    return svc.Get(c.In.ID)  // c.In.ID came from :id param (via struct tag)
})

// "GET /users with ListReq, respond with []UserRes"  
kruda.Get[ListReq, []UserRes](app, "/users", func(c *kruda.C[ListReq]) (*[]UserRes, error) {
    return svc.List(c.In.Page, c.In.Search)  // from query string
})

// Mixed: param + body
kruda.Put[UpdateReq, UserRes](app, "/users/:id", func(c *kruda.C[UpdateReq]) (*UserRes, error) {
    return svc.Update(c.In.ID, c.In.Name)  // ID from param, Name from body
})
```

### 7.4 Struct Tag Reference

```go
// Tags tell Kruda where each field comes from:
type ExampleReq struct {
    // Path params — from URL like /users/:id
    ID   int    `param:"id"   validate:"gt=0"`

    // Query string — from ?page=2&search=foo
    Page   int    `query:"page"   default:"1"    validate:"gte=1"`
    Limit  int    `query:"limit"  default:"20"   validate:"gte=1,lte=100"`
    Search string `query:"search"`

    // Body — from JSON request body
    Name  string `json:"name"  validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
}

// Response (just json tags, no special treatment)
type UserRes struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

### 7.5 Pre-compiled Input Parser

At startup, Kruda inspects struct tags and creates a type-specific parser:

```go
// Built once at route registration — not at request time
type inputParser struct {
    paramFields []fieldParser   // fields with `param:` tag
    queryFields []fieldParser   // fields with `query:` tag
    hasBody     bool            // true if any field has `json:` tag
}

type fieldParser struct {
    fieldIndex int
    source     string                      // "param", "query"
    key        string                      // param/query key name
    converter  func(string) (any, error)   // string→int, string→uuid, etc.
    defaultVal string                      // from `default:` tag
}

// Pre-built converters (no reflection at runtime)
var converters = map[reflect.Kind]func(string) (any, error){
    reflect.Int:     func(s string) (any, error) { return strconv.Atoi(s) },
    reflect.Int64:   func(s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
    reflect.Float64: func(s string) (any, error) { return strconv.ParseFloat(s, 64) },
    reflect.Bool:    func(s string) (any, error) { return strconv.ParseBool(s) },
}
```

### 7.6 Performance Guarantee

Go generics compile down to specific code per type instantiation:

```go
// This generic code:
kruda.Post[CreateReq, UserRes](app, "/users", func(c *kruda.C[CreateReq]) (*UserRes, error) {
    return &UserRes{ID: 1, Name: c.In.Name}, nil
})

// Compiles to roughly equivalent of:
app.Post("/users", func(c *kruda.Ctx) error {
    var in CreateReq
    sonic.Unmarshal(c.bodyBytes, &in)
    validate(in)
    result := &UserRes{ID: 1, Name: in.Name}
    return c.JSON(result)
})
```

No reflection, no interface boxing, no virtual dispatch on the hot path.

### 7.7 Short Handler (Prototyping Mode)

For rapid prototyping, handlers can omit the `error` return:

```go
// Full handler (production)
kruda.Post[Req, Res](app, "/users", func(c *kruda.C[Req]) (*Res, error) {
    return svc.Create(c.In)
})

// Short handler (prototyping) — panics are caught by Recovery middleware
kruda.Post[Req, Res](app, "/users", func(c *kruda.C[Req]) *Res {
    return kruda.Must(svc.Create(c.In))
})
```

---

## 8. Validation

### 8.1 Design

- Struct tag-based: `validate:"required,email,min=2"`
- Pre-compiled at startup (not per-request)
- Returns structured error with field names and messages
- Compatible with existing `go-playground/validator` tags

### 8.2 Built-in Validators

```
required      — field must be present and non-zero
email         — valid email format
url           — valid URL format
min=N         — minimum length (string) or value (number)
max=N         — maximum length (string) or value (number)
gte=N         — greater than or equal
lte=N         — less than or equal
gt=N          — greater than
lt=N          — less than
oneof=a b c   — must be one of listed values
uuid          — valid UUID format
alpha         — alphabetic characters only
alphanum      — alphanumeric characters only
numeric       — numeric string
contains=X    — must contain substring
startswith=X  — must start with
endswith=X    — must end with
len=N         — exact length
```

### 8.3 Validation Error Response

```json
{
    "error": "validation_error",
    "message": "request validation failed",
    "details": [
        {
            "field": "email",
            "tag": "required",
            "message": "email is required"
        },
        {
            "field": "age",
            "tag": "gte",
            "value": "-1",
            "message": "age must be greater than or equal to 0"
        }
    ]
}
```

### 8.4 Custom Validators

```go
app.Validator().Register("thai_phone", func(value string) bool {
    return regexp.MustCompile(`^0[689]\d{8}$`).MatchString(value)
})

type UserReq struct {
    Phone string `json:"phone" validate:"required,thai_phone"`
}
```

---

## 9. Error Handling

### 9.1 KrudaError

```go
type KrudaError struct {
    Code    int    `json:"code"`              // HTTP status code
    Message string `json:"message"`           // human-readable message
    Detail  string `json:"detail,omitempty"`  // optional detail
    Err     error  `json:"-"`                 // wrapped error (not exposed)
}

func (e *KrudaError) Error() string { return e.Message }
func (e *KrudaError) Unwrap() error { return e.Err }

// Constructor helpers
func NewError(code int, message string, err ...error) *KrudaError
func BadRequest(message string) *KrudaError      // 400
func Unauthorized(message string) *KrudaError     // 401
func Forbidden(message string) *KrudaError        // 403
func NotFound(message string) *KrudaError         // 404
func Conflict(message string) *KrudaError         // 409
func TooManyRequests(message string) *KrudaError  // 429
func InternalError(message string) *KrudaError    // 500
```

### 9.2 Error Mapping

Register error-to-status-code mappings once, use everywhere:

```go
app := kruda.New()

app.MapError(gorm.ErrRecordNotFound, 404, "resource not found")
app.MapError(gorm.ErrDuplicatedKey, 409, "resource already exists")
app.MapError(ErrInsufficientBalance, 422, "insufficient balance")
app.MapError(redis.Nil, 404, "cache miss")

// Now handlers just return errors — Kruda maps them automatically
kruda.Get[IDParam, User](app, "/users/:id", func(c *kruda.C[IDParam]) (*User, error) {
    return db.First(&User{}, c.In.ID)  // gorm.ErrRecordNotFound → 404
})
```

### 9.3 Error Mapping Implementation

```go
type ErrorMapping struct {
    Status  int
    Message string
}

func (app *App) MapError(target error, status int, message string) {
    app.errorMap[target] = ErrorMapping{Status: status, Message: message}
}

func (app *App) resolveError(err error) *KrudaError {
    // 1. Already a KrudaError?
    var ke *KrudaError
    if errors.As(err, &ke) {
        return ke
    }
    
    // 2. Check error map
    for target, mapping := range app.errorMap {
        if errors.Is(err, target) {
            return &KrudaError{
                Code:    mapping.Status,
                Message: mapping.Message,
                Err:     err,
            }
        }
    }
    
    // 3. Default: 500 Internal Server Error
    return &KrudaError{
        Code:    500,
        Message: "internal server error",
        Err:     err,
    }
}
```

### 9.4 Global Error Handler

```go
app := kruda.New(kruda.WithErrorHandler(func(c *kruda.Ctx, err error) {
    ke := app.resolveError(err)
    
    // Log if 5xx
    if ke.Code >= 500 {
        slog.Error("server error",
            "method", c.Method(),
            "path", c.Path(),
            "error", err,
            "latency", c.Latency(),
        )
    }
    
    c.Status(ke.Code).JSON(ke)
}))
```

---

## 10. Middleware & Lifecycle

### 10.1 Middleware Function

```go
type MiddlewareFunc func(c *Ctx) error

// Usage
app.Use(func(c *kruda.Ctx) error {
    start := time.Now()
    err := c.Next()  // call next handler
    slog.Info("request",
        "method", c.Method(),
        "path", c.Path(),
        "status", c.Status(),
        "latency", time.Since(start),
    )
    return err
})
```

### 10.2 Lifecycle Hooks

```
Request arrives
    │
    ▼
  OnRequest          ← logging, request ID, rate limiting
    │
    ▼
  OnParse            ← body parsing, content-type detection
    │
    ▼
  Middleware chain    ← app.Use() middleware
    │
    ▼
  BeforeHandle       ← auth, permission checks
    │
    ▼
  Handler            ← your business logic
    │
    ▼
  AfterHandle        ← response transformation, caching
    │
    ▼
  OnResponse         ← logging, metrics, cleanup
    │
    ▼
  OnError (if any)   ← error handling
```

```go
type Hooks struct {
    OnRequest     []HookFunc
    OnParse       []HookFunc
    BeforeHandle  []HookFunc
    AfterHandle   []HookFunc
    OnResponse    []HookFunc
    OnError       []ErrorHookFunc
    OnShutdown    []func()
}
```

### 10.3 Per-route Hooks

```go
app.Get("/admin", adminHandler, kruda.WithHooks(kruda.HookConfig{
    BeforeHandle: []kruda.HookFunc{requireAdmin, auditLog},
    AfterHandle:  []kruda.HookFunc{cacheResponse},
}))
```

### 10.4 Provide / Need (Request-Scoped DI)

`Provide[T]` / `Need[T]` is the **request-scoped** layer of Kruda's DI system. Values are created fresh every request (e.g. current user from JWT).

For the full DI system including app-level singletons, transients, modules, and auto-cleanup, see **Section 12: Dependency Injection**.

```go
// Register: create a CurrentUser for every request
kruda.Provide[CurrentUser](app, func(c *kruda.Ctx) (*CurrentUser, error) {
    token := c.Header("Authorization")
    user, err := verifyJWT(token)
    if err != nil {
        return nil, Unauthorized("invalid token")
    }
    return user, nil
})

// Retrieve: type-safe, no string key
app.Get("/profile", func(c *kruda.Ctx) error {
    user := kruda.Need[CurrentUser](c)
    return c.JSON(user)
})
```

**Implementation:**
```go
func Provide[T any](app *App, fn func(*Ctx) (*T, error)) {
    typeKey := reflect.TypeOf((*T)(nil)).Elem()  // called once at startup
    app.Use(func(c *Ctx) error {
        val, err := fn(c)
        if err != nil {
            return err
        }
        c.locals[typeKey] = val  // O(1) set
        return c.Next()
    })
}

func Need[T any](c *Ctx) *T {
    typeKey := reflect.TypeOf((*T)(nil)).Elem()
    return c.locals[typeKey].(*T)  // O(1) get
}
```

---

## 11. Plugin System

### 11.1 Plugin Interface

```go
type Plugin interface {
    Name() string
    Install(app *App) error
}

// Or functional style
type PluginFunc func(app *App) error

// Standard middleware usage
app.Use(kruda.CORS())
app.Use(kruda.Logger())

// Guard — reads as "protect with" (alias for Use, semantic clarity for auth)
app.Guard(kruda.JWT(kruda.JWTConfig{Secret: "..."}))

// Limit — reads like English: "limit 100 per minute"
app.Limit(100).Per(time.Minute)
// equivalent to: app.Use(kruda.RateLimit(100, time.Minute))
```

### 11.2 Middleware Ecosystem Architecture

Middleware is split into two tiers: **Built-in** (zero external deps, part of core) and **Contrib** (separate modules, opt-in). This design follows the patterns of Gin (`gin-contrib/`), Fiber (`fiber/contrib/`), and Echo.

```
kruda/                          ← core framework (zero external deps)
├── middleware/
│   ├── logger.go               ← built-in
│   ├── recovery.go             ← built-in
│   ├── cors.go                 ← built-in
│   ├── requestid.go            ← built-in
│   └── static.go               ← built-in
│
kruda-contrib/                  ← official contrib org (separate Go modules)
├── jwt/                        ← github.com/go-kruda/kruda/contrib/jwt
├── ratelimit/                  ← github.com/go-kruda/kruda/contrib/ratelimit
├── session/                    ← github.com/go-kruda/kruda/contrib/session
├── csrf/                       ← github.com/go-kruda/kruda/contrib/csrf
├── compress/                   ← github.com/go-kruda/kruda/contrib/compress
├── swagger/                    ← github.com/go-kruda/kruda/contrib/swagger
├── websocket/                  ← github.com/go-kruda/kruda/contrib/websocket
├── cache/                      ← github.com/go-kruda/kruda/contrib/cache
├── oauth2/                     ← github.com/go-kruda/kruda/contrib/oauth2
├── validator/                  ← github.com/go-kruda/kruda/contrib/validator
└── timeout/                    ← github.com/go-kruda/kruda/contrib/timeout
```

### 11.3 Why Two Tiers

**Reason 1: Dependency isolation**

Built-in middleware has zero external deps. When a user runs `go get kruda`, they get only what they need. Contrib middleware may pull in external deps:

```
jwt/        → needs crypto libs (jose, jwx)
ratelimit/  → needs Redis client (optional)
session/    → needs store driver (Redis, memcached, etc.)
swagger/    → needs OpenAPI spec libs
websocket/  → needs gorilla/websocket or nhooyr/websocket
```

If all middleware were in core, every `go get kruda` would download JWT crypto, Redis clients, WebSocket libs — even for a simple REST API that uses none of them.

**Reason 2: Independent release cycles**

Core framework may be stable at v1.2.0, but JWT middleware may need a security patch urgently. With separate modules:

```
kruda          v1.2.0  (stable, no change)
contrib/jwt    v1.1.1  (security patch for CVE)
contrib/cache  v1.0.3  (Redis 8 compat fix)
```

Users only update what they use. If JWT were in core, every JWT security patch = new framework release for everyone.

**Reason 3: Community contribution**

Contrib packages are easier to contribute to — smaller scope, separate CI, lower barrier. A contributor can fork `contrib/jwt`, add a feature, and PR back without touching the core framework.

**Reason 4: Binary size**

Go linker tree-shakes unused packages, but unused deps still increase `go mod download` time and `go.sum` size. Keeping contrib separate means clean `go.sum` for simple projects.

### 11.4 Built-in Middleware (Part of Core)

These ship with `go get kruda` — zero external deps, needed by virtually every API:

| Middleware | Why Built-in | Description |
|------------|-------------|-------------|
| **Logger** | Every API needs request logging | Structured slog, latency, status, path |
| **Recovery** | Panics must not crash server | Catches panics, logs stack, returns 500 |
| **CORS** | Web APIs don't work without CORS | Configurable origins, methods, headers |
| **RequestID** | Essential for distributed tracing | UUID per request, sets X-Request-ID header |
| **Static** | Common need, zero deps | Serve files from filesystem or embed.FS |

```go
// All built-in, zero import needed:
app := kruda.New()
app.Use(kruda.Logger())            // structured logging
app.Use(kruda.Recovery())          // panic recovery
app.Use(kruda.CORS())              // CORS with sensible defaults
app.Use(kruda.RequestID())         // unique request IDs
app.Static("/assets", "./public")  // static files
```

### 11.5 Contrib Middleware (Separate Modules)

Install only what you need:

```go
import (
    "github.com/go-kruda/kruda/contrib/jwt"
    "github.com/go-kruda/kruda/contrib/ratelimit"
    "github.com/go-kruda/kruda/contrib/swagger"
)

// JWT auth
app.Guard(jwt.New(jwt.Config{
    Secret: os.Getenv("JWT_SECRET"),
}))

// Rate limiting (in-memory or Redis-backed)
app.Use(ratelimit.New(ratelimit.Config{
    Max:      100,
    Window:   time.Minute,
    // Store: ratelimit.RedisStore(redisClient),  // optional Redis
}))

// Auto Swagger from typed handlers
app.Use(swagger.New(swagger.Config{
    Title:   "My API",
    Version: "1.0.0",
}))
```

**Contrib availability at launch:**

| Package | Status at Soft Launch | Why |
|---------|----------------------|-----|
| jwt | ✅ Ready | Auth is expected by everyone |
| ratelimit | ✅ Ready | Production APIs need rate limiting |
| swagger | ✅ Ready | Key Kruda differentiator (auto OpenAPI) |
| csrf | ✅ Ready | Security expectation |
| compress | ✅ Ready | Simple, high value |
| session | 🟡 Soon after | Not every API needs sessions |
| websocket | 🟡 Soon after | Niche use case |
| cache | 🟡 Soon after | Nice-to-have |
| oauth2 | 📅 Community driven | Complex, many providers |
| timeout | 📅 Community driven | Simple to implement per-project |

> **Rule:** A framework without JWT + Rate limit + Swagger at launch feels incomplete. These three contrib packages must be ready on day one, even though they're separate modules.

### 11.6 Plugin Configuration Pattern

Every middleware (built-in and contrib) follows the same config pattern:

```go
// Zero-config with sensible defaults:
app.Use(kruda.CORS())

// Or customize:
app.Use(kruda.CORS(kruda.CORSConfig{
    AllowOrigins:     []string{"https://myapp.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    AllowCredentials: true,
    MaxAge:           3600,
}))
```

**Implementation pattern:**

```go
// Every plugin follows this exact pattern:
func CORS(config ...CORSConfig) MiddlewareFunc {
    cfg := defaultCORSConfig()
    if len(config) > 0 {
        cfg = config[0]
    }
    
    return func(c *Ctx) error {
        // CORS logic
        return c.Next()
    }
}

type CORSConfig struct {
    AllowOrigins     []string
    AllowMethods     []string
    AllowHeaders     []string
    AllowCredentials bool
    MaxAge           int
}
```

---

## 12. Dependency Injection

Kruda has a built-in DI container inspired by the best patterns from .NET `IServiceCollection`, Kotlin Koin, and FastAPI `Depends`. No external libraries needed — no Wire, no Uber FX.

### 12.1 Design Principles

1. **Type is the key** — no string-based lookup, no annotations, no code generation
2. **Fail fast at startup** — missing dependencies, circular refs, constructor errors all panic before the server starts
3. **Zero cost at runtime** — all reflection happens once at startup; after that, plain map[type]→instance lookup
4. **Three scopes** — Singleton (once), Transient (every call), Request (every request via Provide/Need)
5. **Auto-detect cleanup** — if a type implements `io.Closer`, Kruda closes it on shutdown automatically
6. **Go-idiomatic** — constructors are normal Go functions, no special interfaces to implement

### 12.2 Container

```go
type Container struct {
    mu         sync.RWMutex
    providers  map[instanceKey]provider   // registered constructors
    instances  map[instanceKey]any        // resolved singletons
    closers    []io.Closer               // auto-detected for shutdown (LIFO)
    healthers  []HealthChecker           // auto-detected for /health
    resolved   bool                      // lock after Listen()
}

type instanceKey struct {
    typ  reflect.Type
    name string        // "" = default (unnamed)
}

type provider struct {
    fn        any              // constructor function or value
    deps      []reflect.Type   // parameter types (inspected once at Give time)
    scope     Scope            // Singleton | Transient
    lazy      bool             // true = resolve on first Use, not at startup
    isValue   bool             // true = direct value, not a constructor
}

type Scope int

const (
    Singleton Scope = iota  // created once, shared everywhere (default)
    Transient               // created fresh every time Use[T] is called
)
```

### 12.3 Registration API — `Give`

All registration happens at startup before `app.Listen()`.

#### Direct values

```go
// Give a pre-built value (config, literal, external instance)
kruda.Give(app, &Config{
    DBHost: "localhost:5432",
    Secret: "my-jwt-secret",
})
```

#### Constructor functions

```go
// Kruda inspects function params → resolves them from the container
kruda.Give(app, NewDB)           // func(cfg *Config) *gorm.DB
kruda.Give(app, NewRedis)        // func(cfg *Config) *redis.Client
kruda.Give(app, NewUserRepo)     // func(db *gorm.DB) *UserRepo
kruda.Give(app, NewOrderRepo)    // func(db *gorm.DB, r *redis.Client) *OrderRepo
kruda.Give(app, NewUserService)  // func(repo *UserRepo) *UserService
kruda.Give(app, NewOrderService) // func(repo *OrderRepo, u *UserService) *OrderService
```

**Registration order doesn't matter.** Kruda topologically sorts dependencies at resolve time.

#### Constructors with error return

```go
// Both signatures supported:
kruda.Give(app, func(cfg *Config) *gorm.DB { ... })            // no error
kruda.Give(app, func(cfg *Config) (*gorm.DB, error) { ... })   // with error

// If constructor returns error → panic at startup with clear message:
// panic: kruda: failed to create *gorm.DB: dial tcp: connection refused
```

#### Constructors are plain Go functions

```go
// No magic tags, no special interfaces, no annotations
// Just write normal Go constructors:
func NewDB(cfg *Config) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(cfg.DBHost))
    if err != nil {
        return nil, err
    }
    return db, nil
}

func NewUserRepo(db *gorm.DB) *UserRepo {
    return &UserRepo{db: db}
}

func NewUserService(repo *UserRepo) *UserService {
    return &UserService{repo: repo}
}
```

### 12.4 Interface Binding — `GiveAs`

Register a concrete type as an interface — essential for testing and swapping implementations.

```go
// UserRepository is an interface, *UserRepo is the concrete implementation
kruda.GiveAs[UserRepository](app, NewUserRepo)
// NewUserRepo returns *UserRepo which implements UserRepository

// Resolve via interface
repo := kruda.Use[UserRepository](app)  // returns *UserRepo wrapped as interface

// Swap for testing
if os.Getenv("APP_ENV") == "test" {
    kruda.GiveAs[UserRepository](app, NewMockUserRepo)
} else {
    kruda.GiveAs[UserRepository](app, NewUserRepo)
}
```

**Implementation:**
```go
func GiveAs[I any](app *App, fn any) {
    ifaceType := reflect.TypeOf((*I)(nil)).Elem()
    // Register constructor with interface type as key
    // Resolve returns concrete type, stored under interface key
    app.container.register(ifaceType, "", fn, Singleton, false)
}
```

### 12.5 Transient Scope — `GiveTransient`

Create a **new instance every time** `Use[T]` is called. For objects that must not be shared.

```go
// Singleton (default): one DB pool shared everywhere
kruda.Give(app, NewDB)

// Transient: new transaction every time
kruda.GiveTransient(app, NewDBTransaction)  // func(db *gorm.DB) *Transaction

// Every call creates a fresh instance:
tx1 := kruda.Use[Transaction](app)  // new
tx2 := kruda.Use[Transaction](app)  // different instance
```

**Implementation:**
```go
func GiveTransient[T any](app *App, fn any) {
    app.container.register(reflect.TypeOf((*T)(nil)).Elem(), "", fn, Transient, false)
}

// In resolve:
if prov.scope == Transient {
    return callConstructor(prov)  // always fresh, never cached
}
```

### 12.6 Lazy Resolve — `GiveLazy`

Don't create the instance until the first `Use[T]` call. Useful for expensive connections not needed on every startup.

```go
// Eager (default): created at startup
kruda.Give(app, NewPaymentGateway)      // connects immediately

// Lazy: created on first access
kruda.GiveLazy(app, NewPaymentGateway)  // no connection until first Use

// First call: creates + caches. Subsequent calls: returns cached.
gw := kruda.Use[PaymentGateway](app)   // creates here (once)
gw2 := kruda.Use[PaymentGateway](app)  // returns cached
```

**Implementation:**
```go
func GiveLazy[T any](app *App, fn any) {
    app.container.register(reflect.TypeOf((*T)(nil)).Elem(), "", fn, Singleton, true)
}

// In resolve:
func (c *Container) resolve(key instanceKey) any {
    // Lazy singletons use sync.Once pattern
    if prov.lazy {
        c.mu.Lock()
        defer c.mu.Unlock()
        if inst, ok := c.instances[key]; ok {
            return inst  // already resolved
        }
        inst := c.doResolve(key, nil)
        c.instances[key] = inst
        return inst
    }
    // ...
}
```

**Perf:** First call pays `sync.Mutex` + constructor cost. All subsequent calls are plain map lookup — zero overhead.

### 12.7 Named Instances — `GiveNamed` / `UseNamed`

When you need multiple instances of the same type (e.g. read/write DBs, multiple caches).

```go
// Two databases of the same type
kruda.GiveNamed[*gorm.DB](app, "write", func(cfg *Config) *gorm.DB {
    return connectDB(cfg.WriteDBURL)
})
kruda.GiveNamed[*gorm.DB](app, "read", func(cfg *Config) *gorm.DB {
    return connectDB(cfg.ReadDBURL)
})

// Retrieve by name
writeDB := kruda.UseNamed[*gorm.DB](app, "write")
readDB := kruda.UseNamed[*gorm.DB](app, "read")

// Use in constructor
kruda.Give(app, func() *UserRepo {
    return &UserRepo{
        write: kruda.UseNamed[*gorm.DB](app, "write"),
        read:  kruda.UseNamed[*gorm.DB](app, "read"),
    }
})
```

**Implementation:**
```go
func GiveNamed[T any](app *App, name string, fn any) {
    key := instanceKey{
        typ:  reflect.TypeOf((*T)(nil)).Elem(),
        name: name,
    }
    app.container.register(key.typ, name, fn, Singleton, false)
}

func UseNamed[T any](app *App, name string) *T {
    key := instanceKey{typ: reflect.TypeOf((*T)(nil)).Elem(), name: name}
    return app.container.resolve(key).(*T)
}
```

### 12.8 Modules — `kruda.Module`

Group related providers for clean organization in large projects.

```go
// ─── Define modules ───
var UserModule = kruda.Module("user",
    kruda.Give(NewUserRepo),
    kruda.Give(NewUserService),
)

var OrderModule = kruda.Module("order",
    kruda.Give(NewOrderRepo),
    kruda.Give(NewOrderService),
)

var InfraModule = kruda.Module("infra",
    kruda.Give(NewDB),
    kruda.Give(NewRedis),
    kruda.Give(NewMailer),
)

// ─── Install into app ───
func main() {
    app := kruda.New()
    
    kruda.Give(app, loadConfig)
    app.Install(InfraModule, UserModule, OrderModule)
    
    app.Listen(":3000")
}
```

**Implementation:**
```go
type Module struct {
    name      string
    providers []moduleEntry
}

type moduleEntry struct {
    fn    any
    scope Scope
    lazy  bool
    name  string  // for named instances
}

func Module(name string, entries ...moduleEntry) *Module {
    return &Module{name: name, providers: entries}
}

func (app *App) Install(modules ...*Module) {
    for _, mod := range modules {
        for _, entry := range mod.providers {
            app.container.register(/* ... */)
        }
    }
}
```

**Module entries are created by Give/GiveAs/etc. returning moduleEntry when used outside app context:**
```go
// When Give receives *App → registers directly
kruda.Give(app, NewUserRepo)

// When Give receives no *App → returns moduleEntry for use in Module()
var UserModule = kruda.Module("user",
    kruda.Give(NewUserRepo),      // returns moduleEntry
    kruda.Give(NewUserService),   // returns moduleEntry
)
```

### 12.9 Auto Cleanup — `io.Closer` Detection

If a resolved instance implements `io.Closer`, Kruda automatically calls `Close()` on shutdown in **reverse order** (LIFO — DB closes last).

```go
// No cleanup code needed — Kruda detects io.Closer automatically
kruda.Give(app, func(cfg *Config) (*gorm.DB, error) {
    return gorm.Open(postgres.Open(cfg.DBHost))
})
// gorm.DB has Close() → Kruda calls db.Close() on shutdown

kruda.Give(app, func(cfg *Config) *redis.Client {
    return redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
})
// redis.Client has Close() → Kruda calls redis.Close() on shutdown

// Shutdown order (LIFO):
// 1. redis.Close()  ← registered second, closed first
// 2. db.Close()     ← registered first, closed last
```

**For types without io.Closer, use the explicit Cleanup return:**
```go
// Return kruda.Cleanup function for custom cleanup
kruda.Give(app, func(cfg *Config) (*CustomPool, kruda.Cleanup) {
    pool := NewPool(cfg)
    return pool, func() {
        pool.Drain()
        pool.Shutdown()
    }
})
```

**Implementation:**
```go
type Cleanup func()

func (c *Container) afterResolve(key instanceKey, instance any) {
    // Auto-detect io.Closer
    if closer, ok := instance.(io.Closer); ok {
        c.closers = append(c.closers, closer)
    }
}

func (c *Container) Shutdown(ctx context.Context) error {
    var errs []error
    // Close in reverse order (LIFO)
    for i := len(c.closers) - 1; i >= 0; i-- {
        if err := c.closers[i].Close(); err != nil {
            errs = append(errs, err)
        }
    }
    // Call explicit Cleanup functions (also LIFO)
    for i := len(c.cleanups) - 1; i >= 0; i-- {
        c.cleanups[i]()
    }
    return errors.Join(errs...)
}
```

### 12.10 Auto Health Check — `HealthChecker` Detection

If a resolved instance implements `HealthChecker`, Kruda automatically includes it in the health endpoint.

```go
type HealthChecker interface {
    Health(ctx context.Context) error
}

// Example: DB implements HealthChecker
func (db *PostgresDB) Health(ctx context.Context) error {
    return db.PingContext(ctx)
}

func (r *RedisClient) Health(ctx context.Context) error {
    return r.Ping(ctx).Err()
}

// Kruda auto-detects and registers health checks
app.Get("/health", kruda.HealthHandler())

// Response (all healthy):
// 200 { "status": "ok", "checks": { "*PostgresDB": "ok", "*RedisClient": "ok" } }

// Response (DB down):
// 503 { "status": "unhealthy", "checks": { "*PostgresDB": "connection refused", "*RedisClient": "ok" } }
```

**Implementation:**
```go
func (c *Container) afterResolve(key instanceKey, instance any) {
    // Auto-detect HealthChecker
    if hc, ok := instance.(HealthChecker); ok {
        c.healthers = append(c.healthers, namedHealth{
            name:    key.typ.String(),
            checker: hc,
        })
    }
}

func HealthHandler() HandlerFunc {
    return func(c *Ctx) error {
        checks := map[string]string{}
        healthy := true
        for _, h := range c.app.container.healthers {
            if err := h.checker.Health(c.Context()); err != nil {
                checks[h.name] = err.Error()
                healthy = false
            } else {
                checks[h.name] = "ok"
            }
        }
        status := 200
        statusText := "ok"
        if !healthy {
            status = 503
            statusText = "unhealthy"
        }
        return c.Status(status).JSON(Map{
            "status": statusText,
            "checks": checks,
        })
    }
}
```

### 12.11 Retrieval API — `Use`

```go
// Singleton / Lazy — returns cached instance
db := kruda.Use[*gorm.DB](app)

// Transient — returns new instance every call
tx := kruda.Use[Transaction](app)

// Named — returns by type + name
writeDB := kruda.UseNamed[*gorm.DB](app, "write")

// Request-scoped — via context (see Section 10.4)
user := kruda.Need[CurrentUser](c)
```

### 12.12 Resolve Implementation

```go
func Use[T any](app *App) *T {
    key := instanceKey{typ: reflect.TypeOf((*T)(nil)).Elem()}
    return app.container.resolve(key).(*T)
}

func (c *Container) resolve(key instanceKey, path []instanceKey) any {
    // 1. Cycle detection
    for _, p := range path {
        if p == key {
            names := make([]string, len(path)+1)
            for i, pk := range append(path, key) {
                names[i] = pk.typ.String()
            }
            panic(fmt.Sprintf("kruda: circular dependency: %s", strings.Join(names, " → ")))
        }
    }

    // 2. Already resolved? (Singleton cache)
    if inst, ok := c.instances[key]; ok {
        return inst
    }

    // 3. Find provider
    prov, ok := c.providers[key]
    if !ok {
        panic(fmt.Sprintf("kruda: no provider for %v (name=%q)", key.typ, key.name))
    }

    // 4. Direct value?
    if prov.isValue {
        c.instances[key] = prov.fn
        c.afterResolve(key, prov.fn)
        return prov.fn
    }

    // 5. Resolve dependencies recursively
    fnType := reflect.TypeOf(prov.fn)
    args := make([]reflect.Value, fnType.NumIn())
    for i := 0; i < fnType.NumIn(); i++ {
        depKey := instanceKey{typ: fnType.In(i)}
        args[i] = reflect.ValueOf(c.resolve(depKey, append(path, key)))
    }

    // 6. Call constructor
    results := reflect.ValueOf(prov.fn).Call(args)

    // 7. Handle error return: func(...) (T, error)
    if len(results) == 2 {
        if cleanup, ok := results[1].Interface().(Cleanup); ok {
            c.cleanups = append(c.cleanups, cleanup)
        } else if !results[1].IsNil() {
            err := results[1].Interface().(error)
            panic(fmt.Sprintf("kruda: failed to create %v: %v", key.typ, err))
        }
    }

    instance := results[0].Interface()

    // 8. Cache if Singleton
    if prov.scope == Singleton {
        c.instances[key] = instance
    }

    // 9. Auto-detect io.Closer and HealthChecker
    c.afterResolve(key, instance)

    return instance
}
```

### 12.13 Startup Validation

Kruda validates the entire dependency graph **before** the server starts:

```go
func (app *App) Listen(addr string) error {
    // ──── DI Validation (before accepting any request) ────

    // 1. Resolve all eager (non-lazy) singletons
    for key, prov := range app.container.providers {
        if prov.scope == Singleton && !prov.lazy {
            app.container.resolve(key, nil)
        }
    }

    // 2. Validate lazy providers have resolvable deps
    for key, prov := range app.container.providers {
        if prov.lazy {
            app.container.validateDeps(key)  // check deps exist, no cycles
        }
    }

    // ──── If we get here, all dependencies are satisfied ────
    // ──── Start server ────
    return app.transport.Listen(addr)
}
```

**Error messages are clear and actionable:**
```
panic: kruda: no provider for *gorm.DB
  required by: *UserRepo
  required by: *UserService
  hint: did you forget kruda.Give(app, NewDB)?

panic: kruda: circular dependency detected:
  *ServiceA → *ServiceB → *ServiceC → *ServiceA

panic: kruda: failed to create *gorm.DB:
  dial tcp 127.0.0.1:5432: connection refused

panic: kruda: duplicate provider for *UserRepo
  existing: registered at main.go:42
  new:      registered at main.go:58
  hint: use kruda.GiveNamed to register multiple instances of the same type
```

### 12.14 Combining App-Level and Request-Level DI

```go
func main() {
    app := kruda.New()

    // ─── App-level: created once at startup ───
    kruda.Give(app, &Config{Secret: "jwt-secret"})
    kruda.Give(app, NewDB)
    kruda.Give(app, NewUserRepo)
    kruda.Give(app, NewUserService)

    // ─── Request-level: created every request ───
    kruda.Provide[CurrentUser](app, func(c *kruda.Ctx) (*CurrentUser, error) {
        // Access app-level DI from request-level provider
        svc := kruda.Use[UserService](app)
        return svc.FromJWT(c.Header("Authorization"))
    })

    // ─── Handlers use both ───
    app.Get("/profile", func(c *kruda.Ctx) error {
        user := kruda.Need[CurrentUser](c)  // request-scoped
        return c.JSON(user)
    })

    app.Get("/admin/users", func(c *kruda.Ctx) error {
        svc := kruda.Use[UserService](app)  // app-scoped singleton
        return c.JSON(svc.ListAll())
    })

    app.Listen(":3000")
}
```

### 12.15 Full Real-World Example

```go
// ═══════════════════════════════════════════════════════
// modules/infra.go
// ═══════════════════════════════════════════════════════
var InfraModule = kruda.Module("infra",
    kruda.Give(func(cfg *Config) (*gorm.DB, error) {
        return gorm.Open(postgres.Open(cfg.DBURL))
    }),
    kruda.Give(func(cfg *Config) *redis.Client {
        return redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
    }),
    kruda.Give(func(cfg *Config) *ses.Client {
        return ses.NewFromConfig(cfg.AWS)
    }),
)

// ═══════════════════════════════════════════════════════
// modules/user.go
// ═══════════════════════════════════════════════════════
var UserModule = kruda.Module("user",
    kruda.Give(NewUserRepo),     // func(db *gorm.DB) *UserRepo
    kruda.Give(NewUserService),  // func(repo *UserRepo, mailer *ses.Client) *UserService
)

// ═══════════════════════════════════════════════════════
// modules/order.go
// ═══════════════════════════════════════════════════════
var OrderModule = kruda.Module("order",
    kruda.Give(NewOrderRepo),     // func(db *gorm.DB, cache *redis.Client) *OrderRepo
    kruda.Give(NewOrderService),  // func(repo *OrderRepo, userSvc *UserService) *OrderService
)

// Payment gateway is expensive — connect lazily
var PaymentModule = kruda.Module("payment",
    kruda.GiveLazy(NewStripeClient),   // func(cfg *Config) *StripeClient
    kruda.Give(NewPaymentService),     // func(stripe *StripeClient, orderSvc *OrderService) *PaymentService
)

// ═══════════════════════════════════════════════════════
// main.go
// ═══════════════════════════════════════════════════════
func main() {
    cfg := loadConfig()
    app := kruda.New()

    // Register config
    kruda.Give(app, cfg)

    // Install all modules
    app.Install(InfraModule, UserModule, OrderModule, PaymentModule)

    // Request-scoped auth
    kruda.Provide[CurrentUser](app, func(c *kruda.Ctx) (*CurrentUser, error) {
        return kruda.Use[UserService](app).FromJWT(c.Header("Authorization"))
    })

    // Error mapping
    app.MapError(gorm.ErrRecordNotFound, 404, "not found")
    app.MapError(gorm.ErrDuplicatedKey, 409, "already exists")

    // Routes
    app.Group("/api/v1").
        Guard(kruda.JWT(cfg.Secret)).
        Resource("/users", kruda.Use[UserService](app)).
        Resource("/orders", kruda.Use[OrderService](app))

    // Health check (auto-detects DB, Redis, etc.)
    app.Get("/health", kruda.HealthHandler())

    // Start — validates all DI, connects all eager deps, then listens
    app.Listen(":3000")

    // Shutdown — closes Redis, then DB (LIFO), automatically
}
```

### 12.16 Performance Guarantees

```
Phase          │ What happens              │ Cost
═══════════════╪═══════════════════════════╪══════════════════════════
Registration   │ Give(), GiveAs(), etc.    │ map insert — O(1) per call
(startup)      │ reflect.TypeOf() once     │ ~50ns per type
───────────────┼───────────────────────────┼──────────────────────────
Resolution     │ Topological sort + call   │ O(N) total, runs once
(startup)      │ constructors              │ N = number of providers
───────────────┼───────────────────────────┼──────────────────────────
Use[T](app)    │ Singleton: map lookup     │ O(1) — ~5ns
(runtime)      │ Transient: call fn        │ O(1) — cost of constructor
               │ Lazy (first): resolve+    │ O(1) — sync.Mutex once
               │ cache                     │
───────────────┼───────────────────────────┼──────────────────────────
Need[T](c)     │ context locals map lookup │ O(1) — ~5ns
(per-request)  │                           │
───────────────┼───────────────────────────┼──────────────────────────
Shutdown       │ io.Closer calls (LIFO)    │ O(K) — K = closeable deps
```

**vs Uber FX:** FX uses reflection on every `Invoke()` call and builds the graph lazily. Kruda resolves everything upfront, then runtime is pure map lookups.

**vs Wire:** Wire generates code at compile time (zero reflect), but requires a separate code generation step and doesn't support lifecycle hooks. Kruda trades ~50ns/type startup cost for zero build complexity.

### 12.17 DI API Summary

| Function | What | Scope | When resolved |
|----------|------|-------|---------------|
| `kruda.Give(app, val/fn)` | Register singleton | Singleton | Startup (eager) |
| `kruda.GiveAs[I](app, fn)` | Register as interface | Singleton | Startup (eager) |
| `kruda.GiveLazy(app, fn)` | Register lazy singleton | Singleton | First `Use` call |
| `kruda.GiveTransient(app, fn)` | Register transient | Transient | Every `Use` call |
| `kruda.GiveNamed[T](app, n, fn)` | Register named instance | Singleton | Startup (eager) |
| `kruda.Use[T](app)` | Retrieve app-level | — | O(1) map lookup |
| `kruda.UseNamed[T](app, n)` | Retrieve named | — | O(1) map lookup |
| `kruda.Provide[T](app, fn)` | Register request-scoped | Request | Every request |
| `kruda.Need[T](c)` | Retrieve request-scoped | — | O(1) map lookup |
| `kruda.Module(name, ...)` | Group providers | — | — |
| `app.Install(modules...)` | Install modules | — | — |
| `kruda.HealthHandler()` | Auto health endpoint | — | — |

**Auto-detected interfaces (zero config):**

| Interface | Effect |
|-----------|--------|
| `io.Closer` | Auto `Close()` on shutdown (LIFO order) |
| `HealthChecker` | Auto-included in `HealthHandler()` response |

---

## 13. Auto CRUD (Resource)

### 13.1 Overview

The `Resource` feature auto-generates 5 RESTful endpoints from a single service interface, dramatically reducing boilerplate.

### 13.2 Service Interface

```go
// A service must implement any combination of these methods.
// Kruda uses Go reflection AT STARTUP (not runtime) to detect which methods exist.

type Lister[T any] interface {
    List(q ListQuery) ([]T, int, error)     // GET /resource
}

type Getter[T any] interface {
    Get(id int) (*T, error)                  // GET /resource/:id
}

type Creator[B any, T any] interface {
    Create(body B) (*T, error)               // POST /resource
}

type Updater[B any, T any] interface {
    Update(id int, body B) (*T, error)       // PUT /resource/:id
}

type Deleter interface {
    Delete(id int) error                     // DELETE /resource/:id
}
```

### 13.3 ListQuery (Built-in)

```go
type ListQuery struct {
    Page   int    `query:"page"   default:"1"`
    Limit  int    `query:"limit"  default:"20"`
    Sort   string `query:"sort"   default:"created_at"`
    Order  string `query:"order"  default:"desc"`
    Search string `query:"search"`
}

func (q ListQuery) Offset() int {
    return (q.Page - 1) * q.Limit
}
```

### 13.4 Paginated Response (Auto-generated)

```go
type PagedResponse[T any] struct {
    Data  []T `json:"data"`
    Total int `json:"total"`
    Page  int `json:"page"`
    Limit int `json:"limit"`
    Pages int `json:"pages"`  // total / limit (ceiling)
}
```

### 13.5 Resource Registration

```go
// Registers all endpoints that the service implements
func (app *App) Resource(path string, service any) *App {
    // Detected at startup via reflection — NO runtime reflection
    // Generates specific routes equivalent to hand-written code
}

// Usage
app.Group("/api/v1").Resource("/products", &ProductService{db: db})

// Generates:
// GET    /api/v1/products          → ProductService.List()
// GET    /api/v1/products/:id      → ProductService.Get()
// POST   /api/v1/products          → ProductService.Create()
// PUT    /api/v1/products/:id      → ProductService.Update()
// DELETE /api/v1/products/:id      → ProductService.Delete()
```

### 13.6 Resource Hooks

```go
app.Resource("/products", productService, kruda.ResourceHooks{
    BeforeCreate: []HookFunc{requireAdmin},
    BeforeDelete: []HookFunc{requireAdmin, auditLog},
    AfterCreate:  []HookFunc{sendNotification},
})
```

---

## 14. Concurrency Helpers

### 14.1 Parallel (Typed Parallel Execution)

```go
// Parallel2: run 2 tasks concurrently, return typed results — no type assertion needed
user, orders, err := kruda.Parallel2(c,
    func(ctx context.Context) (*User, error) { return fetchUser(ctx, id) },
    func(ctx context.Context) ([]Order, error) { return fetchOrders(ctx, id) },
)
// user is *User, orders is []Order — compile-time safe!

// Parallel3: 3 tasks
user, orders, reviews, err := kruda.Parallel3(c,
    func(ctx context.Context) (*User, error) { return fetchUser(ctx, id) },
    func(ctx context.Context) ([]Order, error) { return fetchOrders(ctx, id) },
    func(ctx context.Context) ([]Review, error) { return fetchReviews(ctx, id) },
)

// Parallel: dynamic N tasks (returns []any, needs type assertion)
results, err := kruda.Parallel(c, tasks...)
```

### 14.2 Race (First Wins)

```go
// Return first successful result
result, err := kruda.Race(c,
    func(ctx context.Context) (*Data, error) { return cache.Get(key) },
    func(ctx context.Context) (*Data, error) { return db.Find(key) },
)
```

### 14.3 Each (Fan-Out / Fan-In)

```go
// Process items concurrently with worker pool
results, err := kruda.Each(c, items, processItem, kruda.Workers(10))
```

### 14.4 Pipeline

```go
// Sequential stages with concurrent items within each stage
result, err := kruda.Pipeline(c, input,
    stage1_validate,
    stage2_transform,
    stage3_enrich,
)
```

---

## 15. Security

### 15.1 Secure Defaults (Enabled Automatically)

```go
type SecurityConfig struct {
    // Body limits
    BodyLimit   int           // default: 4MB
    HeaderLimit int           // default: 8KB
    
    // Timeouts
    ReadTimeout  time.Duration // default: 30s
    WriteTimeout time.Duration // default: 30s
    IdleTimeout  time.Duration // default: 120s
    
    // Headers (all enabled by default)
    XContentTypeOptions string // default: "nosniff"
    XFrameOptions       string // default: "DENY"
    XXSSProtection      string // default: "1; mode=block"
    
    // HSTS (disabled by default, enable for HTTPS)
    HSTS        bool
    HSTSMaxAge  int            // default: 31536000 (1 year)
    
    // Content Security Policy
    CSP         string         // default: "" (not set)
    
    // Path traversal protection
    PathTraversal bool         // default: true
    
    // CRLF injection protection
    CRLFProtection bool        // default: true
    
    // Slowloris protection
    SlowlorisProtection bool   // default: true
}
```

### 15.2 Context Safety (vs Fiber)

| Issue | Fiber | Kruda |
|-------|-------|-------|
| String params in goroutines | ❌ unsafe, use `[]byte` copy | ✅ safe, proper string copy |
| Context in goroutines | ❌ data race | ✅ extracted values are safe |
| Body in goroutines | ❌ pool reuse corruption | ✅ copy-on-read |
| Session fixation | ❌ CVE-2024-38513 | ✅ server-only session ID |
| CSRF token reuse | ❌ CVE-2023-45141 | ✅ double submit + session binding |
| UUID zero fallback | ❌ CVE-2025-66630 | ✅ fail-fast, panic on crypto/rand failure |
| CRLF injection | ❌ CVE-2020-15111 | ✅ sanitize all header values |

### 15.3 UUID Generation (Fail-Fast)

```go
// Kruda: NEVER silently generate zero UUID
func GenerateUUID() string {
    b := make([]byte, 16)
    _, err := crypto_rand.Read(b)
    if err != nil {
        // FAIL FAST — never return predictable UUID
        panic("kruda: crypto/rand failed: " + err.Error())
    }
    // Format as UUID v4
    b[6] = (b[6] & 0x0f) | 0x40
    b[8] = (b[8] & 0x3f) | 0x80
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
        b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
```

### 15.4 CSRF Plugin Design

```go
app.Use(kruda.CSRF(kruda.CSRFConfig{
    // Double Submit Cookie method (default)
    Method:     "double_submit",
    CookieName: "_csrf",
    HeaderName: "X-CSRF-Token",
    
    // OR Synchronizer Token Pattern
    Method:     "synchronizer",
    SessionKey: "csrf_token",
    
    // Common
    SameSite:   http.SameSiteLaxMode,
    Secure:     true,
    HTTPOnly:   true,
}))
```

---

## 16. Auto OpenAPI / Swagger

### 16.1 Overview

Because every typed handler and Resource already has type information (body, params, query, response), Kruda can auto-generate OpenAPI 3.0 spec without any manual annotation.

### 16.2 How It Works

```
Typed Handler Registration
    │
    ▼
Extract type info via reflect (at startup only)
    │
    ▼
Build OpenAPI schema from struct tags
    │
    ▼
Generate /docs endpoint with Swagger UI
```

### 16.3 Usage

```go
app.Use(kruda.Swagger(kruda.SwaggerConfig{
    Title:       "My API",
    Description: "API built with Kruda",
    Version:     "1.0.0",
    Path:        "/docs",        // Swagger UI
    SpecPath:    "/docs/spec",   // JSON spec
}))
```

### 16.4 Schema Generation from Struct

```go
type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=2,max=100" doc:"User's full name"`
    Email string `json:"email" validate:"required,email"         doc:"Email address"`
    Age   int    `json:"age"   validate:"gte=0,lte=150"          doc:"User's age"`
}

// Auto-generates:
// {
//   "type": "object",
//   "required": ["name", "email"],
//   "properties": {
//     "name":  { "type": "string", "minLength": 2, "maxLength": 100, "description": "User's full name" },
//     "email": { "type": "string", "format": "email", "description": "Email address" },
//     "age":   { "type": "integer", "minimum": 0, "maximum": 150, "description": "User's age" }
//   }
// }
```

---

## 17. Testing Utilities

### 17.1 Built-in Test Client

```go
// No need to start an actual server
func TestCreateUser(t *testing.T) {
    app := setupApp()
    
    resp := app.Test(kruda.TestReq{
        Method:  "POST",
        Path:    "/api/v1/users",
        Body:    CreateUserReq{Name: "Tiger", Email: "tiger@test.com"},
        Headers: map[string]string{"Authorization": "Bearer " + token},
    })
    
    assert.Equal(t, 201, resp.Status)
    
    var user UserRes
    resp.JSON(&user)
    assert.Equal(t, "Tiger", user.Name)
}
```

### 17.2 TestResponse

```go
type TestResponse struct {
    Status  int
    Headers map[string]string
    Body    []byte
}

func (r *TestResponse) JSON(v any) error        // parse body as JSON
func (r *TestResponse) Text() string             // body as string
func (r *TestResponse) Header(key string) string // get header
```

### 17.3 Benchmark Helper

```go
func BenchmarkCreateUser(b *testing.B) {
    app := setupApp()
    req := kruda.TestReq{
        Method: "POST",
        Path:   "/api/v1/users",
        Body:   CreateUserReq{Name: "Tiger", Email: "tiger@test.com"},
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            app.Test(req)
        }
    })
}
```

---

## 18. JSON Engine

### 18.1 Default: Sonic (ByteDance)

Sonic uses SIMD (Single Instruction, Multiple Data) and JIT (Just-In-Time) compilation for JSON serialization — 2-5x faster than `encoding/json`.

### 18.2 Pluggable JSON Interface

```go
type JSONEncoder interface {
    Marshal(v any) ([]byte, error)
}

type JSONDecoder interface {
    Unmarshal(data []byte, v any) error
}

// Default
app := kruda.New()  // uses Sonic

// Custom
app := kruda.New(kruda.WithJSON(customEncoder, customDecoder))

// Fallback to stdlib
app := kruda.New(kruda.WithStdJSON())
```

### 18.3 Sonic Requirements

- Linux/macOS on amd64 or arm64
- Fallback to `encoding/json` on unsupported platforms
- Go 1.25+ (cgo overhead -30% on Go 1.26)

---

## 19. Performance Techniques

### 19.1 Zero-Allocation Hot Path

| Technique | Where | Impact |
|-----------|-------|--------|
| `sync.Pool` for Context | Per-request context reuse | Eliminate GC pressure |
| `sync.Pool` for buffers | JSON encoding/response buffers | Reduce allocations |
| Pre-allocated maps | Params, query, headers maps in Context | No map creation per request |
| `[]byte`-first internal API | All internal data as `[]byte`, lazy string conversion | Reduce string allocations |
| Radix tree router | Route matching without allocations | O(log n) zero-alloc lookup |
| Pre-compiled validators | Validation functions built at startup | No reflection at runtime |
| Pre-compiled param parsers | Type converters built at startup | No strconv per-request overhead |

### 19.2 Goroutine Pool

```go
// Worker pool for handler execution
type workerPool struct {
    workers    int
    taskCh     chan task
    workerPool sync.Pool
}
```

### 19.3 Header Optimization

Instead of `map[string]string` for headers, use a fixed-size array for common headers:

```go
type headerStore struct {
    // Fast path: common headers in fixed slots
    contentType   string
    contentLength int
    authorization string
    userAgent     string
    // Slow path: uncommon headers in map
    extra map[string]string
}
```

### 19.4 String Interning

For frequently used strings (method names, common header names), use pre-allocated constants:

```go
var (
    methodGET    = "GET"
    methodPOST   = "POST"
    methodPUT    = "PUT"
    methodDELETE = "DELETE"
    // etc.
)
```

### 19.5 Response Buffer Pool

JSON response encoding uses pooled buffers — zero allocation per response:

```go
var bufPool = sync.Pool{
    New: func() any { return bytes.NewBuffer(make([]byte, 0, 4096)) },
}

func (c *Ctx) JSON(v any) error {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)
    
    if err := sonic.NewEncoder(buf).Encode(v); err != nil {
        return err
    }
    c.SetHeader("Content-Type", "application/json")
    c.writer.Write(buf.Bytes())  // zero-copy write
    return nil
}
```

### 19.6 Lazy Parsing (Zero Cost If Unused)

Query string and headers are NOT parsed until first access:

```go
type Ctx struct {
    rawQuery    string
    queryParsed bool
    queryCache  map[string]string  // pre-allocated from pool, lazy filled
}

func (c *Ctx) Query(key string) string {
    if !c.queryParsed {
        c.queryCache = parseQuery(c.rawQuery)  // parse once, cache
        c.queryParsed = true
    }
    return c.queryCache[key]
}
```

Endpoints that don't use query strings pay zero cost.

### 19.7 PGO (Profile-Guided Optimization)

Ship a `default.pgo` CPU profile with the framework — Go compiler uses it to optimize hot paths:

```bash
# Generate profile from benchmarks
go test -bench=. -cpuprofile=default.pgo ./benchmarks/

# Build with PGO (Go 1.21+, automatic if default.pgo exists in module root)
go build ./...
```

**Free 2-7% improvement** with no code changes. Kruda ships `default.pgo` in the repo.

### 19.8 GC Tuning (Go 1.26 Green Tea GC)

Go 1.26's Green Tea GC reduces tail latency. Kruda exposes a tuning option:

```go
// For API servers with plenty of memory: reduce GC frequency
app := kruda.New(kruda.WithGCPercent(200))

// Or set via environment
// GOGC=200 ./my-server
```

**Effect:** Fewer GC pauses → lower p99 latency at the cost of ~2x memory usage.

### 19.9 Why Pure Go v1.0 Wins Without Rust

Kruda v1.0 is pure Go — no Rust, no CGO. Yet it beats Gin, Echo, and matches Fiber in benchmarks. This section explains exactly why, with code-level comparisons showing where competitors waste performance.

Rust acceleration (Section 27) comes in v2.0 as a bonus. But v1.0 must already be the fastest type-safe Go framework on its own. The following design decisions make this possible.

#### 19.9.1 vs Gin — Unnecessary Allocations

Gin allocates on every request in multiple places that Kruda avoids:

```go
// Gin: allocates render.JSON struct every JSON response
func (c *Context) JSON(code int, obj any) {
    c.Render(code, render.JSON{Data: obj})  // new struct every call
}

// Gin: no sync.Pool for response buffers
// Gin: uses encoding/json by default (slowest JSON option)
// Gin: parses query string eagerly even if handler doesn't use it
```

```go
// Kruda: zero-alloc JSON response via pooled buffers + Sonic
var bufPool = sync.Pool{
    New: func() any { return bytes.NewBuffer(make([]byte, 0, 4096)) },
}

func (c *Ctx) JSON(v any) error {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)
    
    if err := c.app.json.Encode(buf, v); err != nil {  // Sonic by default
        return err
    }
    c.SetHeader("Content-Type", "application/json")
    c.writer.Write(buf.Bytes())
    return nil
}
// Result: 0 alloc/op vs Gin's ~350 B/op
```

Gin can't fix this without breaking its API — `render.JSON{Data: obj}` is a public interface that thousands of projects depend on. Kruda starts clean.

#### 19.9.2 vs Fiber — Safety Tax

Fiber achieves speed by reusing fasthttp's context, but this creates a "safety tax" that makes it dangerous for production:

```go
// Fiber: context is reused across requests
app.Get("/user", func(c *fiber.Ctx) error {
    name := c.Query("name")
    go func() {
        fmt.Println(name)    // ✅ safe (string is immutable)
        fmt.Println(c.Body()) // 💀 DATA RACE — c is already reused!
    }()
    return c.JSON(user)
})

// Fiber's "solution": copy the context manually
app.Get("/user", func(c *fiber.Ctx) error {
    cc := c.Copy()  // allocation! defeats the purpose of reusing context
    go processAsync(cc)
    return c.JSON(user)
})
```

Fiber has accumulated 4 CVEs related to this context reuse design. Developers must remember `c.Copy()` every time they pass context to a goroutine — forget once and you have a data race.

```go
// Kruda: context values are always safe
// Strings are immutable in Go — extracted values are safe in goroutines
// Context itself is pooled but Reset() clears all state before reuse
app.Get("/user", func(c *kruda.Ctx) error {
    name := c.Query("name")
    body := c.BodyBytes()     // returns a copy, safe to keep
    go func() {
        fmt.Println(name)     // ✅ safe
        fmt.Println(body)     // ✅ safe
    }()
    return c.JSON(user)
})
// No c.Copy() needed. No footgun. Same performance.
```

#### 19.9.3 Sonic JSON — 3.3x Faster Than stdlib (No Rust Needed)

ByteDance's Sonic uses JIT compilation + partial SIMD in pure Go (with assembly):

```
encoding/json  →  ~5,000 ns/op   ← Gin default
Sonic (Go)     →  ~1,500 ns/op   ← Kruda v1.0 default
Rust simd-json →  ~800 ns/op     ← Kruda v2.0 (future)
```

Sonic alone gives Kruda a 3.3x advantage over any framework using `encoding/json`. This is the single biggest performance win in v1.0, and it requires zero Rust — just importing a Go package.

#### 19.9.4 Static Route O(1) Map — Faster Than Radix Tree

Most real-world applications have more static routes than parameterized routes. Kruda optimizes for this:

```go
type Router struct {
    // Fast path: static routes in O(1) map
    static map[string]map[string]*Route  // method → exact_path → route
    
    // Slow path: parameterized routes in radix tree
    tree   map[string]*node              // method → radix tree
}

func (r *Router) Find(method, path string, c *Ctx) {
    // Try static first — O(1) map lookup (~5ns)
    if routes, ok := r.static[method]; ok {
        if route, ok := routes[path]; ok {
            c.route = route
            return
        }
    }
    // Not found in static → walk radix tree for /user/:id style routes
    r.tree[method].find(path, c)
}
```

In a typical app with 50 routes where 40 are static (`/api/users`, `/api/health`, etc.) and 10 have params (`/api/users/:id`), 80% of requests hit the O(1) map path. Gin, Echo, and Fiber walk a radix tree for every request regardless.

#### 19.9.5 Lazy Parsing — Zero Cost If Unused

```go
// Gin/Echo: parse query string eagerly on every request
func (c *Context) handleRequest() {
    c.queryCache = parseQuery(c.Request.URL.RawQuery)  // always runs
    // even if handler never calls c.Query()
}

// Kruda: parse only when first accessed
func (c *Ctx) Query(key string) string {
    if !c.queryParsed {
        c.queryCache = parseQuery(c.rawQuery)
        c.queryParsed = true
    }
    return c.queryCache[key]
}
```

A JSON API handler that reads body but never touches query string saves ~200ns per request from not parsing the query. Multiply by 100K req/sec = significant.

#### 19.9.6 Generics — Compile-time vs Runtime Reflection

```go
// Gin: reflect at runtime for every request
func (c *Context) ShouldBindJSON(obj any) error {
    // reflect.TypeOf(obj) called every time
    return json.NewDecoder(c.Request.Body).Decode(obj)
}

// Kruda: generics — type known at compile time
func Bind[T any](c *Ctx) (T, error) {
    var v T
    err := c.app.json.Unmarshal(c.BodyBytes(), &v)
    return v, err
}
// Compiler generates specialized code per type — no reflect at request time
```

#### 19.9.7 Benchmark Projection — v1.0 Pure Go

```
Framework        req/sec      latency     alloc/op    Design advantage
─────────────────────────────────────────────────────────────────────────
Kruda v1.0       520K         1.9 μs      0 B         All of the above combined
Fiber            505K         2.0 μs      0 B         Fast but unsafe context
Echo             410K         2.4 μs      200 B       Decent but allocates
Gin              380K         2.6 μs      350 B       Legacy design, allocates
─────────────────────────────────────────────────────────────────────────
Kruda v2.0       680K+        1.4 μs      0 B         + Rust simd-json (v2.0)
```

**Why Kruda v1.0 pure Go beats Gin/Echo:**
- Zero alloc (sync.Pool everywhere) → saves ~350 B/op vs Gin
- Sonic JSON (3.3x vs encoding/json) → saves ~3,500 ns on JSON
- Static route O(1) → saves ~100ns on 80% of requests
- Lazy parsing → saves ~200ns when query/form unused
- Generics → saves reflect overhead

**Why Kruda v1.0 matches Fiber without Fiber's safety issues:**
- Same zero-alloc approach
- Same radix tree + net/http (or fasthttp)
- But context is safe in goroutines (no c.Copy() footgun)
- No CVE history from context reuse bugs

**Conclusion:** Rust is a v2.0 bonus that widens the lead. v1.0 pure Go wins on _design_, not _language_.

---

## 20. CLI Tool

### 20.1 Installation

```bash
go install github.com/go-kruda/kruda/cmd/kruda@latest
```

### 20.2 Commands

```bash
# Create new project
kruda new my-app
kruda new my-app --template api      # REST API starter
kruda new my-app --template fullstack # with frontend

# Generate
kruda generate resource users         # generate CRUD for users
kruda generate middleware auth         # generate middleware template
kruda generate plugin myplugin        # generate plugin template

# Development
kruda dev                              # hot reload dev server
kruda build                            # production build
kruda test                             # run tests with benchmarks
```

### 20.3 Project Template

```
my-app/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handler/
│   │   └── user.go
│   ├── model/
│   │   └── user.go
│   ├── service/
│   │   └── user.go
│   └── middleware/
│       └── auth.go
├── config/
│   └── config.go
├── .cursor/
│   └── rules                    ← Cursor AI rules (auto-generated)
├── .claude/
│   └── INSTRUCTIONS.md          ← Claude Code instructions (auto-generated)
├── .github/
│   └── copilot-instructions.md  ← GitHub Copilot instructions (auto-generated)
├── go.mod
├── go.sum
├── Dockerfile
├── docker-compose.yml
└── README.md
```

> **AI-Ready by default:** Every `kruda new` project includes AI assistant configuration files. Developers using Cursor, Claude Code, or GitHub Copilot get correct code generation from minute one. See Section 29 for details.

---

## 21. Project Structure

### 21.1 Framework Source Structure

```
kruda/                            ← core framework (github.com/go-kruda/kruda)
├── kruda.go              # App struct, New(), Listen()
├── config.go             # Config struct, defaults, options
├── context.go            # Ctx struct, request/response API
├── router.go             # Radix tree router (static O(1) + radix tree)
├── group.go              # Route groups
├── handler.go            # Generic typed handlers
├── resource.go           # Auto CRUD resource
├── middleware.go          # Middleware chain
├── lifecycle.go          # Lifecycle hooks
├── error.go              # KrudaError, error mapping
├── container.go          # DI container (Give/Use/Module/auto-cleanup/health)
├── validation.go         # Validation engine
├── bind.go               # Body/param/query binding
├── concurrent.go         # Parallel, Race, Each helpers
├── map.go                # Map type alias (map[string]any)
├── test.go               # Test helpers
│
├── middleware/            # ← BUILT-IN middleware (zero external deps)
│   ├── logger.go         #   Structured logging (slog)
│   ├── recovery.go       #   Panic recovery
│   ├── cors.go           #   CORS
│   ├── requestid.go      #   Request ID (X-Request-ID)
│   └── static.go         #   Static file serving
│
├── transport/
│   ├── transport.go      # Transport interface
│   ├── fasthttp.go        # fasthttp implementation
│   └── nethttp.go        # net/http implementation
│
├── json/
│   ├── json.go           # JSON interface
│   ├── rust_json.go      # 🦀 Rust simd-json (v2.0, build tag: !pure_go && !no_rust)
│   ├── sonic.go          # Sonic adapter (v1.0 default)
│   └── std.go            # encoding/json fallback (CGO_ENABLED=0)
│
├── rust/                          # 🦀 Rust Secret Weapons (v2.0)
│   ├── kruda-json/                # simd-json accelerated JSON engine
│   │   ├── Cargo.toml
│   │   ├── src/lib.rs
│   │   └── benches/bench.rs
│   ├── kruda-validator/           # Fast regex-based validation
│   │   ├── Cargo.toml
│   │   └── src/lib.rs
│   └── kruda-router/              # Zero-alloc radix tree router
│       ├── Cargo.toml
│       └── src/lib.rs
│
├── internal/
│   ├── bytesconv/        # unsafe bytes↔string (internal only)
│   ├── radix/
│   │   ├── rust_router.go   # 🦀 Rust FFI (v2.0, build tag: !pure_go)
│   │   └── go_router.go     # Pure Go (v1.0 default)
│   ├── validator/
│   │   ├── rust_validator.go # 🦀 Rust FFI (v2.0, build tag: !pure_go)
│   │   └── go_validator.go   # Pure Go (v1.0 default)
│   └── pool/             # object pools
│
├── cmd/
│   └── kruda/            # CLI tool
│       └── main.go
│
├── examples/
│   ├── hello/
│   ├── rest-api/
│   ├── auth-jwt/
│   ├── crud-resource/
│   ├── websocket-chat/
│   └── fullstack/
│
├── benchmarks/
│   ├── bench_test.go
│   ├── compare_fiber_test.go
│   ├── compare_gin_test.go
│   ├── compare_echo_test.go
│   └── compare_fuego_test.go     # ⚠️ Direct competitor benchmark
│
├── docs/
│   ├── getting-started.md
│   ├── routing.md
│   ├── typed-handlers.md
│   ├── middleware.md
│   ├── plugins.md
│   ├── security.md
│   ├── performance.md
│   └── migration-from-fuego.md   # ⚠️ Migration guide
│
├── Makefile              # Build Rust libs + Go
├── go.mod
├── go.sum
├── LICENSE                # MIT
└── README.md
```

```
kruda-contrib/                    ← official contrib (github.com/go-kruda/kruda/contrib)
├── go.mod                # separate Go module (own dependency tree)
├── jwt/                  # JWT authentication
│   ├── jwt.go
│   ├── jwt_test.go
│   └── go.mod            # depends on kruda core + jwt libs
├── ratelimit/            # Rate limiting (memory + optional Redis)
│   ├── ratelimit.go
│   ├── memory_store.go
│   ├── redis_store.go
│   └── go.mod
├── swagger/              # Auto OpenAPI docs
│   ├── swagger.go
│   ├── schema.go         # struct → OpenAPI schema
│   ├── ui.go             # embedded Swagger UI
│   └── go.mod
├── csrf/                 # CSRF protection
│   └── csrf.go
├── compress/             # gzip/brotli compression
│   └── compress.go
├── session/              # Session management
│   ├── session.go
│   ├── memory_store.go
│   └── redis_store.go
├── websocket/            # WebSocket support
│   └── websocket.go
├── cache/                # Response caching
│   └── cache.go
├── oauth2/               # OAuth2 providers
│   └── oauth2.go
├── mcp/                  # MCP Server plugin (auto tool generation)
│   ├── mcp.go            # Main plugin: New(), Config
│   ├── scanner.go        # Route scanning + tool generation
│   ├── schema.go         # Go types → JSON Schema
│   ├── transport_streamable.go  # Streamable HTTP transport
│   ├── transport_sse.go  # SSE transport (legacy)
│   ├── executor.go       # Tool execution → internal HTTP
│   └── naming.go         # Route → tool name conventions
└── timeout/              # Request timeout
    └── timeout.go
```

**Why two repos:**
- `kruda/` has zero external deps → clean `go.sum`
- `kruda-contrib/` has per-package deps → only pulled when needed
- Independent release cycles → JWT security patch ≠ framework release
- Lower contributor barrier → submit PR to small contrib package
- See Section 11.3 for full rationale

**Build modes:**
```bash
# Default: Rust → Sonic → stdlib (auto-detected)
make build

# Pure Go only (no CGO, no Rust)
CGO_ENABLED=0 go build -tags pure_go ./...

# Sonic only (no Rust, but with CGO)
go build -tags no_rust ./...
```

---

## 22. Roadmap & Phases

> **Strategy:** Ship pure Go v1.0 first (Phase 1-6) that beats competitors on design alone (see Section 19.9), establish community, then add Rust acceleration (Phase 7) as a v2.0 differentiator that no other Go framework has.

### Phase 1 — Foundation (Week 1-3)

| Task | Description | Est. Days | Priority |
|------|-------------|-----------|----------|
| `kruda.go` | App struct, New(), Listen(), method chaining | 1 | 🔴 |
| `config.go` | Config struct, defaults, functional options | 1 | 🔴 |
| `context.go` | Ctx struct, sync.Pool, request/response API | 3 | 🔴 |
| `router.go` | Radix tree router, zero-alloc matching | 4 | 🔴 |
| `group.go` | Route groups, prefix, scoped middleware | 2 | 🔴 |
| `middleware.go` | Middleware chain, Next(), ordered execution | 2 | 🔴 |
| `error.go` | KrudaError, error mapping, global handler | 1 | 🔴 |
| `transport/nethttp.go` | net/http transport (start simple) | 2 | 🔴 |
| Graceful shutdown | Signal handling, connection draining | 1 | 🔴 |

**Milestone:** Basic API working on net/http

```go
app := kruda.New()
app.Get("/ping", func(c *kruda.Ctx) error { return c.JSON(Map{"pong": true}) })
app.Listen(":3000")
```

### Phase 2 — Type System & Validation (Week 4-6)

> **Core goal:** Typed handlers must feel magical — parse, validate, and deliver typed data in one step. No boilerplate. This is the #1 reason developers will choose Kruda over any other Go framework.

| Task | Description | Est. Days | Priority |
|------|-------------|-----------|----------|
| `handler.go` | Generic typed handlers (Post[B], Get[P], etc.) | 4 | 🔴 |
| `bind.go` | Body/param/query binding with struct tags | 3 | 🔴 |
| `validation.go` | Pre-compiled validators from struct tags | 4 | 🔴 |
| `json/sonic.go` | Sonic JSON integration | 1 | 🔴 |
| `lifecycle.go` | Lifecycle hooks (onRequest, beforeHandle, etc.) | 2 | 🟡 |
| `concurrent.go` | Parallel, Race helpers | 2 | 🟡 |
| `map.go` | Map type alias | 0.5 | 🟢 |

**Milestone:** Typed handlers with validation — cleanest API in the Go ecosystem

### Phase 3 — fasthttp & Performance (Week 7-8)

> **Core goal:** Be the fastest type-safe Go framework. Pluggable transport gives Kruda an edge no other typed framework has.

| Task | Description | Est. Days | Priority |
|------|-------------|-----------|----------|
| `transport/fasthttp.go` | fasthttp integration | 4 | 🔴 |
| Transport auto-selection | OS/config-based selection | 1 | 🔴 |
| Zero-alloc context | sync.Pool optimization, pre-alloc maps | 3 | 🔴 |
| Header optimization | Fixed-slot headers | 2 | 🟡 |
| Benchmark suite | vs Fiber, Gin, Echo, Fuego, Hertz | 2 | 🟡 |

**Milestone:** Fastest type-safe Go framework

### Phase 4 — Ecosystem (Week 9-12)

> **Core goal:** Auto CRUD, DI, error mapping, and plugins make Kruda a complete platform — not just a router with generics.

| Task | Description | Est. Days | Priority |
|------|-------------|-----------|----------|
| `container.go` | DI container: Give/Use/GiveAs/GiveLazy/GiveTransient/GiveNamed | 4 | 🔴 |
| `module.go` | DI modules: Module(), Install() | 1 | 🔴 |
| `health.go` | Auto HealthChecker detection + HealthHandler() | 1 | 🔴 |
| `resource.go` | Auto CRUD from service interface | 4 | 🔴 |
| **Built-in middleware (core, zero deps):** | | | |
| `middleware/logger.go` | Structured logging (slog) | 2 | 🔴 |
| `middleware/recovery.go` | Panic recovery | 1 | 🔴 |
| `middleware/cors.go` | CORS middleware | 1 | 🔴 |
| `middleware/requestid.go` | Unique request ID per request | 0.5 | 🔴 |
| `middleware/static.go` | Static file serving | 1 | 🟡 |
| **Contrib middleware (separate modules, must ship at launch):** | | | |
| `contrib/jwt/` | JWT authentication | 2 | 🔴 |
| `contrib/swagger/` | Auto OpenAPI generation | 5 | 🔴 |
| `contrib/ratelimit/` | Rate limiting (memory + Redis) | 2 | 🔴 |
| `contrib/csrf/` | CSRF protection | 2 | 🟡 |
| `contrib/compress/` | gzip/brotli compression | 1 | 🟡 |
| **Post-launch contrib (v1.x):** | | | |
| `contrib/session/` | Session management | 2 | 🟡 |
| `contrib/websocket/` | WebSocket support | 3 | 🟡 |
| `contrib/cache/` | Response caching | 2 | 🟡 |
| `test.go` | Built-in test helpers | 2 | 🔴 |

### Phase 5 — Production Ready & Launch (Week 13-16)

| Task | Description | Priority |
|------|-------------|----------|
| Security audit | Path traversal, header injection, DoS | 🔴 |
| 100% test coverage | Core + plugins | 🔴 |
| Documentation site | VitePress/Starlight | 🔴 |
| CLI tool | kruda new, kruda generate | 🟡 |
| Examples | 10+ complete examples (kruda-examples repo) | 🟡 |
| CI/CD | GitHub Actions, automated benchmarks | 🟡 |
| Migration guides | "Switching from Fiber/Gin/Echo/Fuego to Kruda" | 🟡 |
| **AI-Friendly DX** | | |
| `llms.txt` | Machine-readable framework description for AI assistants | 🔴 |
| `.cursor/rules` | Cursor AI rules (shipped in kruda new template) | 🔴 |
| `.claude/INSTRUCTIONS.md` | Claude Code instructions (shipped in template) | 🔴 |
| `.github/copilot-instructions.md` | GitHub Copilot instructions (shipped in template) | 🔴 |
| Rich godoc examples | Every exported function has runnable examples | 🟡 |

### Phase 6 — Launch & Community (Week 16-18)

| Action | Details |
|--------|---------|
| GitHub | `github.com/go-kruda/kruda` |
| go pkg | `go get github.com/go-kruda/kruda` |
| Blog post #1 | "Introducing Kruda — Type-Safe Go with Auto-Everything" |
| Blog post #2 | "Go Framework Benchmark 2026 — Honest Numbers" |
| Blog post #3 | "From 85 Lines to 5 — CRUD APIs in Kruda" |
| **Blog post #4** | **"The First AI-Native Go Framework — MCP, llms.txt, and AI-Ready DX"** |
| Benchmark post | Performance comparison with charts (vs all major frameworks) |
| Discord/Telegram | Community server |
| Reddit/HN | Launch post on r/golang and Hacker News |

### Phase 7 — 🦀 Rust Acceleration (Week 19-22)

> **This is the secret weapon phase.** No other Go framework has Rust-accelerated internals.
> Must maintain pure Go fallback for every Rust component.

| Task | Description | Est. Days | Priority |
|------|-------------|-----------|----------|
| `rust/kruda-json/` | simd-json Rust library + CGO bridge | 5 | 🔴 |
| `json/rust_json.go` | Go wrapper with build tags | 2 | 🔴 |
| `rust/kruda-validator/` | Fast regex validation in Rust | 4 | 🟡 |
| `internal/validator/rust_validator.go` | Go wrapper | 2 | 🟡 |
| `rust/kruda-router/` | Zero-alloc radix tree in Rust | 5 | 🟡 |
| `internal/radix/rust_router.go` | Go wrapper | 2 | 🟡 |
| Makefile | Rust build + Go build pipeline | 1 | 🔴 |
| Benchmark comparison | Rust vs Sonic vs stdlib numbers | 2 | 🔴 |
| Blog post | "How We Made Go 6x Faster with Rust" | 1 | 🟡 |

**Milestone:** Rust-accelerated JSON, validator, router — with pure Go fallback

**Expected performance gains:**
```
Component        Pure Go       Rust          Improvement
───────────────────────────────────────────────────────
JSON parse       ~5 μs         ~0.8 μs       6.2x faster
Regex validate   ~1.2 μs       ~0.15 μs      8x faster
Route matching   ~120 ns       ~45 ns        2.7x faster
```

**Build tag system:**
```bash
go build ./...                      # Default: auto-detect (Rust→Sonic→stdlib)
go build -tags pure_go ./...        # Force: Pure Go, no CGO
go build -tags no_rust ./...        # Force: Sonic only, no Rust
CGO_ENABLED=0 go build ./...        # Force: No CGO at all
```

### Future — AI Integration & Growth

| Task | Description | Priority |
|------|-------------|----------|
| **MCP Server Plugin** | `contrib/mcp/` — auto-generate MCP tools from typed handlers | 🔴 |
| MCP + Resource() | Auto CRUD → auto MCP tools (1 line = 5 endpoints + 5 tools) | 🔴 |
| MCP auth passthrough | Forward auth tokens from AI agents to middleware chain | 🔴 |
| MCP blog post | "The First Web Framework with Built-in AI Agent Support" | 🔴 |
| TypeScript client SDK codegen | `kruda generate client --output ./sdk/client.ts` | 🟡 |
| gRPC integration | gRPC alongside REST | 🟡 |
| GraphQL plugin | Auto GraphQL from same types | 🟡 |
| Response caching | In-memory + Redis | 🟢 |
| Admin dashboard | Auto-generated admin UI | 🟢 |
| VS Code extension | Snippet + auto-complete | 🟢 |

### Roadmap Visual Timeline

```
Week  1-3:  ████████░░░░░░░░░░░░░░░░  Phase 1: Foundation (pure Go)
Week  4-6:  ░░░░░░░░████████░░░░░░░░  Phase 2: Type System (cleanest DX in Go)
Week  7-8:  ░░░░░░░░░░░░░░░░████░░░░  Phase 3: fasthttp (fastest typed framework)
Week  9-12: ░░░░░░░░░░░░░░░░░░░░████  Phase 4: Ecosystem (auto CRUD, DI, plugins)
Week 13-16: ████████████████░░░░░░░░  Phase 5: Production ready + AI-Friendly DX
Week 16-18: ░░░░░░░░░░░░░░░░████████  Phase 6: Launch 🚀 (llms.txt, AI blog post)
Week 19-22: ░░░░████████████░░░░░░░░  Phase 7: 🦀 Rust Secret Weapon
Week 23+:   ░░░░░░░░░░░░░░░░████████  Future: 🤖 MCP + Growth

Key milestones:
  Week 6:  ✅ Typed handlers working (demo-able)
  Week 8:  ✅ Fastest type-safe Go framework (benchmark proof)
  Week 12: ✅ Auto CRUD + plugins (feature complete)
  Week 15: 🤖 AI-Friendly DX ready (llms.txt, cursor/claude/copilot rules)
  Week 16: 🚀 Public launch ("first AI-native Go framework")
  Week 22: 🦀 Rust acceleration live
  Week 25: 🤖 MCP plugin live ("1 line = API + AI agent support")
```

---

## 23. Benchmarks & Targets

### 23.1 Hello World (10M requests, 100 concurrent)

```
Framework       RPS Target    Latency    Memory    Alloc/op
─────────────────────────────────────────────────────────
Fiber           ~280K         ~0.35ms    ~8MB      0 B/op
Echo            ~300K         ~0.33ms    ~10MB     0-1 B/op
Hertz           ~310K         ~0.32ms    ~9MB      0 B/op
Kruda (target)  ~300-320K     ~0.31ms    ~8MB      0 B/op
```

### 23.2 JSON API (parse + validate + respond)

```
Framework       Code Lines   RPS Target   Alloc/op
───────────────────────────────────────────────────
Fiber           45 lines     ~250K        3 B/op
Gin             40 lines     ~240K        4 B/op
Kruda (target)  5 lines      ~245K        2 B/op
```

### 23.3 Concurrency (10K concurrent connections)

```
Framework       RPS Target    p99 Latency   Context Safety
────────────────────────────────────────────────────────
Fiber           ~220K         ~2.1ms        ⚠️ unsafe
Kruda (target)  ~240K         ~1.8ms        ✅ safe
```

---

## 24. Competitive Analysis

### 24.1 vs Fiber

| Aspect | Fiber | Kruda | Winner |
|--------|-------|-------|--------|
| Raw performance | fasthttp-based | fasthttp-based | ~tie |
| Context safety | ⚠️ pool reuse bugs | ✅ safe by default | Kruda |
| Type safety | ❌ none | ✅ Go generics | Kruda |
| Validation | ❌ external lib | ✅ built-in auto | Kruda |
| Auto OpenAPI | ❌ manual | ✅ auto from types | Kruda |
| Auto CRUD | ❌ none | ✅ Resource() | Kruda |
| Error mapping | ❌ manual | ✅ auto | Kruda |
| Ecosystem | ✅ 35K+ stars | ❌ new | Fiber |
| net/http compat | ❌ fasthttp | ✅ yes | Kruda |
| HTTP/2 | ⚠️ limited | ✅ via net/http | Kruda |
| CVE history | 🔴 multiple | 🟢 new, secure design | Kruda |
| Boilerplate | 45 lines/endpoint | 5 lines/endpoint | Kruda |
| Learning curve | Easy (Express-like) | Easy (similar API) | Tie |

### 24.2 vs Gin

| Aspect | Gin | Kruda | Winner |
|--------|-----|-------|--------|
| Market share | 48% of Go devs | New | Gin |
| Performance | Very fast | Similar or better | ~tie |
| Type safety | Limited (binding) | Full generics | Kruda |
| Auto OpenAPI | ❌ swag comments | ✅ auto from types | Kruda |
| Auto CRUD | ❌ none | ✅ Resource() | Kruda |
| Maturity | 10+ years | New | Gin |

### 24.3 vs Echo

| Aspect | Echo | Kruda | Winner |
|--------|------|-------|--------|
| Performance | Excellent (often wins benchmarks) | Target: match | ~tie |
| Type safety | Limited | Full generics | Kruda |
| net/http compat | ✅ yes | ✅ yes | Tie |
| Auto features | Minimal | Comprehensive | Kruda |

### 24.4 vs Hertz (CloudWeGo)

| Aspect | Hertz | Kruda | Winner |
|--------|-------|-------|--------|
| Performance | Excellent (fasthttp) | Uses same fasthttp | ~tie |
| DX | Moderate | High (auto-everything) | Kruda |
| Generics | Limited | Full | Kruda |
| Auto CRUD | ❌ none | ✅ Resource() | Kruda |
| ByteDance backing | ✅ yes | ❌ no | Hertz |
| Documentation | Chinese-primary | English + Thai | Context-dependent |

### 24.5 vs Fuego (Typed Go Framework)

> Fuego (github.com/go-fuego/fuego) shares a similar philosophy: Go generics for typed handlers + auto OpenAPI.
> It validated the concept. Kruda extends it into a complete platform.

**Fuego code example:**
```go
fuego.Post(s, "/user/{user}", func(c fuego.ContextWithBody[MyInput]) (*MyOutput, error) {
    body, err := c.Body()
    if err != nil {
        return nil, err
    }
    return &MyOutput{Message: "Hello, " + body.Name}, nil
})
```

**Kruda equivalent:**
```go
kruda.Post[CreateUserReq, UserRes](app, "/users", func(c *kruda.C[CreateUserReq]) (*UserRes, error) {
    return svc.Create(c.In)  // In is already parsed — no c.Body() call needed
})
```

| Aspect | Fuego | Kruda | Winner |
|--------|-------|-------|--------|
| Typed handlers (generics) | ✅ yes | ✅ yes | Tie |
| Auto OpenAPI from code | ✅ yes | ✅ yes | Tie |
| net/http compatible | ✅ yes | ✅ yes (+ fasthttp) | Kruda |
| Validation | ✅ go-playground | ✅ pre-compiled | Kruda (faster) |
| Auto CRUD / Resource | ❌ none | ✅ `app.Resource()` | **Kruda** |
| Error mapping | ❌ none | ✅ `app.MapError()` | **Kruda** |
| High-perf transport | ❌ net/http only | ✅ fasthttp switchable | **Kruda** |
| Rust acceleration | ❌ none | ✅ simd-json, validator, router | **Kruda** |
| Concurrency helpers | ❌ none | ✅ Parallel, Race, Each | **Kruda** |
| Lifecycle hooks | ❌ basic middleware | ✅ full lifecycle | **Kruda** |
| Security defaults | ❌ none built-in | ✅ headers, limits, CRLF | **Kruda** |
| Context safety focus | ⚠️ not emphasized | ✅ core design principle | **Kruda** |
| Method chaining | ❌ none | ✅ fluent API | **Kruda** |
| CLI tool | ❌ none | ✅ `kruda new/generate` | **Kruda** |
| Body access DX | ⚠️ `c.Body()` returns err | ✅ `c.In` direct field | **Kruda** |
| Plug into Gin/Echo | ✅ adaptors | ❌ standalone | Fuego |
| Community/Stars | ~1000+ stars | New | Fuego |
| Maturity | Growing (pre-1.0) | New | Fuego |

**DX difference — Input access:**
```go
// Fuego: requires c.Body() call + error check
body, err := c.Body() // extra step

// Kruda: In already parsed and validated
c.In // direct field, ready to use
```

**Summary:** Fuego validated the concept. Kruda extends it into a complete platform with auto CRUD, error mapping, performance transport, Rust acceleration, and security defaults.

### 24.6 Cross-Language Comparison

| Aspect | Kruda (Go) | FastAPI (Python) | Elysia (Bun/TS) | Laravel (PHP) | Actix (Rust) |
|--------|-----------|-----------------|-----------------|--------------|-------------|
| Performance | ★★★★★ | ★★☆☆☆ | ★★★☆☆ | ★☆☆☆☆ | ★★★★★ |
| DX / Boilerplate | ★★★★☆ | ★★★★★ | ★★★★★ | ★★★★★ | ★★☆☆☆ |
| Type Safety | ★★★★★ | ★★★☆☆ (hints only) | ★★★★☆ | ★★☆☆☆ | ★★★★★ |
| Security | ★★★★★ | ★★★☆☆ | ★★★☆☆ | ★★★★☆ | ★★★★☆ |
| Ecosystem | ★★★☆☆ | ★★★★★ | ★★★☆☆ | ★★★★★ | ★★☆☆☆ |
| Deploy simplicity | ★★★★★ | ★★☆☆☆ | ★★★☆☆ | ★★☆☆☆ | ★★★★★ |
| Learning curve | ★★★★☆ | ★★★★★ | ★★★★☆ | ★★★★★ | ★★☆☆☆ |
| Concurrency | ★★★★★ | ★★☆☆☆ | ★★★☆☆ | ★☆☆☆☆ | ★★★★★ |

**Kruda wins against:**
- FastAPI/Elysia → on performance (5-15x faster), deploy simplicity, true type safety
- Laravel → on performance (10-30x), concurrency, type safety, deploy simplicity
- Fiber/Gin/Echo → on DX, type safety, security, auto-everything

**Kruda loses to:**
- FastAPI/Elysia/Laravel → on DX (Python/TS/PHP inherently less verbose than Go)
- Actix (Rust) → on raw performance (~10-20%, but irrelevant in real workloads)
- Laravel → on ecosystem size and full-stack features

**Kruda's unique position:** No framework in any language combines all of: type-safe generics + auto CRUD + auto OpenAPI + fasthttp performance + Rust acceleration + security defaults

---

## 25. API Quick Reference

### 25.1 App

```go
// Create
app := kruda.New(opts ...Option)

// Routes (untyped)
app.Get(path, handler, hooks?)
app.Post(path, handler, hooks?)
app.Put(path, handler, hooks?)
app.Delete(path, handler, hooks?)

// Typed routes — C[T] unified context
kruda.Get[In, Out](app, path, handler)
kruda.Post[In, Out](app, path, handler)
kruda.Put[In, Out](app, path, handler)
kruda.Delete[In, Out](app, path, handler)

// Route groups
app.Group(prefix) → *Group

// Auto CRUD
app.Resource(path, service, hooks?)

// Middleware
app.Use(middleware...)
app.Guard(middleware...)              // alias for Use — reads as "protect with"

// Rate limiting
app.Limit(max).Per(duration)

// Error mapping
app.MapError(err, status, message)

// Typed DI — App-level (Singleton)
kruda.Give(app, val/fn)
kruda.GiveAs[I](app, fn)
kruda.GiveLazy(app, fn)
kruda.GiveTransient(app, fn)
kruda.GiveNamed[T](app, name, fn)
kruda.Use[T](app) → *T
kruda.UseNamed[T](app, name) → *T

// Typed DI — Request-level
kruda.Provide[T](app, fn)
kruda.Need[T](c) → *T

// DI Modules
mod := kruda.Module(name, entries...)
app.Install(modules...)

// Health check (auto-detect)
app.Get("/health", kruda.HealthHandler())

// Lifecycle
app.OnRequest(hook)
app.OnResponse(hook)
app.OnShutdown(fn)

// Server
app.Listen(addr)
app.Shutdown(ctx)

// Testing
app.Test(req) → *TestResponse
```

### 25.2 Context

```go
// Request
c.Method() string
c.Path() string
c.Param(name) string
c.ParamInt(name) (int, error)
c.Query(name, default?) string
c.QueryInt(name, default?) int
c.Header(name) string
c.Cookie(name) string
c.IP() string
c.Bind(v) error
c.BodyBytes() []byte
c.BodyString() string

// Typed context
c.In                                 // parsed + validated input (C[T] only)

// Response
c.Status(code) *Ctx
c.JSON(v) error
c.Text(s) error
c.HTML(tmpl, data) error
c.File(path) error
c.Stream(reader) error
c.SSE(events) error
c.Redirect(url, code?) error
c.SetHeader(k, v) *Ctx
c.SetCookie(cookie) *Ctx
c.NoContent() error

// Typed DI retrieval
kruda.Need[T](c) → *T

// Flow
c.Next() error
c.Latency() time.Duration
c.Context() context.Context
```

### 25.3 Concurrency

```go
kruda.Parallel2(c, f1, f2) → (A, B, error)
kruda.Parallel3(c, f1, f2, f3) → (A, B, C, error)
kruda.Parallel(c, tasks...) → ([]any, error)
kruda.Race(c, f1, f2) → (T, error)
kruda.Each(c, items, fn, opts?) → ([]T, error)
kruda.Pipeline(c, input, stages...) → (T, error)
kruda.Must[T](val T, err error) → T           // panic on error (prototyping)
```

### 25.4 Errors

```go
kruda.NewError(code, message, err?)
kruda.BadRequest(msg)
kruda.Unauthorized(msg)
kruda.Forbidden(msg)
kruda.NotFound(msg)
kruda.Conflict(msg)
kruda.TooManyRequests(msg)
kruda.InternalError(msg)
```

### 25.5 Plugins

```go
kruda.Logger(config?)
kruda.CORS(config?)
kruda.JWT(config)
kruda.CSRF(config?)
kruda.RateLimit(max, window, config?)
kruda.Compress(config?)
kruda.Static(root, config?)
kruda.Recovery()
kruda.RequestID()
kruda.Swagger(config)
```

---

## 26. Implementation Priorities

### 26.1 Must-Have for MVP (Week 1-6)

These are non-negotiable for the first public release:

1. ✅ Core App (New, Listen, Shutdown)
2. ✅ Radix Tree Router (zero-alloc, static route O(1) map)
3. ✅ Context (sync.Pool, safe strings, lazy parsing)
4. ✅ Method Chaining API
5. ✅ Route Groups
6. ✅ Middleware Chain
7. ✅ Typed Handlers (generics)
8. ✅ Body/Param/Query Binding
9. ✅ Validation (struct tags)
10. ✅ Error Handling + Mapping
11. ✅ Sonic JSON
12. ✅ net/http Transport
13. ✅ Basic Test Helper

### 26.2 Should-Have for v1.0 (Week 7-12)

14. fasthttp Transport
15. Auto CRUD (Resource)
16. Auto OpenAPI / Swagger
17. Dependency Injection (Give/Use/Module)
18. Concurrency helpers
19. Security defaults
20. CLI Tool

**Built-in middleware (part of core, zero external deps):**
21. Logger (slog structured logging)
22. Recovery (panic handler)
23. CORS (cross-origin)
24. RequestID (distributed tracing)
25. Static file serving

**Contrib middleware (separate modules, must be ready at launch):**
26. JWT auth (`kruda-contrib/jwt`)
27. Rate limiting (`kruda-contrib/ratelimit`)
28. Swagger (`kruda-contrib/swagger`)
29. CSRF protection (`kruda-contrib/csrf`)
30. Compression (`kruda-contrib/compress`)

### 26.3 Nice-to-Have for v1.x

Contrib packages released after initial launch:
31. Session management (`kruda-contrib/session`)
32. WebSocket support (`kruda-contrib/websocket`)
33. Response caching (`kruda-contrib/cache`)
34. OAuth2 (`kruda-contrib/oauth2`)
35. gRPC integration
36. GraphQL plugin

### 26.4 AI-Friendly DX (v1.0 launch requirement)

Must ship alongside v1.0 — zero cost, massive impact:
37. `llms.txt` / `llms-full.txt` on documentation site
38. `.cursor/rules` in CLI template
39. `.claude/INSTRUCTIONS.md` in CLI template
40. `.github/copilot-instructions.md` in CLI template
41. Rich godoc examples on every exported function
42. `kruda-examples/` repo with 10+ examples

### 26.5 MCP Server Plugin for v1.x (post-launch)

After v1.0 launch, highest priority contrib:
43. `contrib/mcp/` — auto MCP tool generation from typed handlers
44. MCP + Resource() integration (1 line = 5 REST + 5 MCP tools)
45. MCP auth passthrough and security filtering
46. MCP example in `kruda-examples/09-mcp-server/`
47. Blog post: "The First Web Framework with Built-in AI Agent Support"

### 26.6 Rust Acceleration for v2.0

After v1.0 is stable with proven pure Go benchmarks:
37. Rust simd-json engine (replace Sonic for JSON hot path)
38. Rust regex validator (replace Go regexp for validation)
39. Rust radix tree router (replace Go radix for param routes)
40. Benchmark proof: "6x faster JSON than stdlib"
41. Blog post: "How We Made Go 6x Faster with Rust"

See Section 19.9 for why v1.0 pure Go already beats competitors.
See Section 27 for full Rust architecture and build tag system.

---

## 27. Rust Secret Weapon

### 27.1 Overview

Kruda uses Rust as a hidden performance layer — users write Go, but performance-critical hot paths run Rust code via CGO/FFI. Every Rust component has a pure Go fallback.

> **Key principle:** Rust is NOT a middleware. It is NOT a separate package. It replaces the _engine internals_ of the framework itself. Users never import, configure, or even know about Rust.

```
┌─────────────────────────────────────────┐
│              Kruda (Go)                  │
│   Router, Context, Middleware, Plugins   │
│   ↑ everything users see = Go           │
├─────────────────────────────────────────┤
│           CGO Bridge (FFI)               │
├─────────────────────────────────────────┤
│          Rust Libraries (.so/.a)         │
│   JSON parser, Validator, Radix tree     │
│   ↑ hot path internals = Rust            │
└─────────────────────────────────────────┘
```

### 27.2 Positioning: Built-in Engine, Not a Plugin

Rust components are **built into the framework core**, controlled by Go build tags. This is fundamentally different from middleware or contrib packages:

```
Request arrives
  → Router matches path         ← Rust replaces this engine (v2.0)
  → Parse JSON body             ← Rust replaces this engine (v2.0)
  → Validate input              ← Rust replaces this engine (v2.0)
  → Middleware chain             ← stays Go (orchestration)
  → Handler (user code)         ← stays Go (user logic)
  → Serialize JSON response     ← Rust replaces this engine (v2.0)
```

Users don't know Rust is running. They write identical Go code whether Rust is enabled or not:

```go
// This code works identically with or without Rust:
app.Post("/users", func(c *kruda.Ctx) error {
    var req CreateUserRequest
    if err := c.Bind(&req); err != nil {    // Rust JSON or Sonic — auto-selected
        return err
    }
    if err := c.Validate(req); err != nil {  // Rust regex or Go — auto-selected
        return err
    }
    return c.JSON(user)                      // Rust JSON or Sonic — auto-selected
})
```

### 27.3 Phase & Timeline

Rust is a **v2.0 feature** — after v1.0 is stable with a proven pure Go baseline:

```
v1.0  →  Pure Go (Sonic JSON, Go radix, Go regexp)
         Already beats Gin/Echo on design alone
         See Section 19.9 for detailed analysis

v2.0  →  Rust acceleration layer (optional, auto-detected)
         go build              → auto-detect Rust libs
         go build -tags pure_go → force pure Go fallback
```

**Why not v1.0:**

1. **Debug complexity** — If core is buggy + Rust is buggy + CGO bridge is buggy, you can't tell which layer caused the issue. v1.0 pure Go = one layer to debug.

2. **Benchmark baseline** — Must know exactly how fast pure Go is before measuring Rust improvement. Otherwise Rust gains are unmeasurable.

3. **Contributor barrier** — v1.0 contributors only need Go. Adding Rust means contributors need both languages, shrinking the contributor pool during the critical early phase.

4. **Build complexity** — Users need Rust toolchain installed to build from source. Pure Go = `go build` and done. Rust = `cargo` + `rustup` + cross-compilation nightmares.

5. **Marketing moment** — v1.0 launch = "fastest type-safe Go framework." v2.0 launch = "now with Rust-powered internals, 6x faster JSON." Two news cycles instead of one.

### 27.4 Build Tag Architecture

Rust is controlled entirely by Go build tags — no user configuration needed:

```go
// json_rust.go — used when Rust libs are available
//go:build !pure_go && !no_rust && cgo

package json

/*
#cgo LDFLAGS: -L${SRCDIR}/../rust/target/release -lkruda_json -lm -ldl -lpthread
#include <stdint.h>
extern int kruda_json_deserialize(const uint8_t*, size_t, uint8_t**, size_t*);
extern void kruda_free(uint8_t*, size_t);
*/
import "C"

type Engine struct{}

func (e *Engine) Unmarshal(data []byte, v any) error {
    // CGO call to Rust simd-json
}

func (e *Engine) Marshal(v any) ([]byte, error) {
    // CGO call to Rust simd-json
}
```

```go
// json_sonic.go — Sonic fallback (no Rust but still fast)
//go:build (pure_go || no_rust || !cgo) && (amd64 || arm64)

package json

import "github.com/bytedance/sonic"

type Engine struct{}

func (e *Engine) Unmarshal(data []byte, v any) error {
    return sonic.Unmarshal(data, v)
}
```

```go
// json_std.go — stdlib fallback (works everywhere)
//go:build pure_go || (!amd64 && !arm64)

package json

import "encoding/json"

type Engine struct{}

func (e *Engine) Unmarshal(data []byte, v any) error {
    return json.Unmarshal(data, v)
}
```

**User build options:**

```bash
go build ./...                        # Auto: Rust → Sonic → stdlib
go build -tags pure_go ./...          # Force pure Go only
go build -tags no_rust ./...          # Sonic only, no Rust
CGO_ENABLED=0 go build ./...         # No CGO at all (stdlib JSON)
```

**Selection priority at build time:**
1. Rust simd-json (if Rust libs compiled + CGO enabled + no `pure_go`/`no_rust` tag)
2. Sonic (if CGO enabled, amd64/arm64, Linux/macOS/Windows)
3. encoding/json (always works, any platform, any arch)

### 27.5 Which Parts Should Be Rust

**Rust (hot path, CPU-bound, runs every request):**
- JSON serialization/deserialization (simd-json)
- Radix tree route matching
- Validation engine (Rust regex is 8x faster than Go regexp)
- String sanitization (CRLF, path traversal)
- Header parsing

**Go (I/O-bound, complex logic, orchestration):**
- Context management, sync.Pool
- Middleware chain, lifecycle hooks
- Plugin system
- Auto CRUD / Resource
- OpenAPI generation
- HTTP transport (fasthttp/net/http)

### 27.6 Rust JSON Engine (simd-json)

```rust
// rust/kruda-json/src/lib.rs
use simd_json;
use std::slice;

#[no_mangle]
pub extern "C" fn kruda_json_deserialize(
    json: *const u8,
    json_len: usize,
    output: *mut *mut u8,
    output_len: *mut usize,
) -> i32 {
    let data = unsafe { slice::from_raw_parts(json, json_len) };
    match simd_json::to_owned_value(&mut data.to_vec()) {
        Ok(val) => { /* serialize to output */ 0 }
        Err(_) => -1
    }
}

#[no_mangle]
pub extern "C" fn kruda_free(ptr: *mut u8, len: usize) {
    unsafe { Vec::from_raw_parts(ptr, len, len); }
}
```

**Go wrapper with build tags:**

```go
//go:build !pure_go && cgo

package json

/*
#cgo LDFLAGS: -L${SRCDIR}/../rust/kruda-json/target/release -lkruda_json -lm -ldl -lpthread
#include <stdint.h>
extern int kruda_json_deserialize(const uint8_t*, size_t, uint8_t**, size_t*);
extern void kruda_free(uint8_t*, size_t);
*/
import "C"

type RustJSON struct{}

func (r *RustJSON) Unmarshal(data []byte, v any) error {
    // CGO call to Rust simd-json
}
```

### 27.7 Rust Validator Engine

```rust
// rust/kruda-validator/src/lib.rs
use regex::Regex;
use once_cell::sync::Lazy;

// Pre-compiled regex — Rust regex is ~8x faster than Go regexp
static EMAIL_RE: Lazy<Regex> = Lazy::new(|| {
    Regex::new(r"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$").unwrap()
});

#[no_mangle]
pub extern "C" fn kruda_validate_email(input: *const u8, len: usize) -> bool {
    let s = unsafe { std::str::from_utf8_unchecked(std::slice::from_raw_parts(input, len)) };
    EMAIL_RE.is_match(s)
}
```

### 27.8 Rust Radix Router

```rust
// rust/kruda-router/src/lib.rs

#[repr(C)]
pub struct MatchResult {
    handler_id: i32,
    params_keys: *const *const u8,
    params_values: *const *const u8,
    params_count: usize,
}

#[no_mangle]
pub extern "C" fn kruda_router_match(
    router: *const Router,
    method: *const u8, method_len: usize,
    path: *const u8, path_len: usize,
    result: *mut MatchResult,
) -> i32 {
    // O(log n) zero-alloc radix tree matching in Rust
    0
}
```

### 27.9 Build System

```makefile
# Makefile
.PHONY: build build-rust build-go

build: build-rust build-go

build-rust:
	cd rust/kruda-json && cargo build --release
	cd rust/kruda-validator && cargo build --release
	cd rust/kruda-router && cargo build --release
	mkdir -p lib/
	cp rust/*/target/release/*.a lib/

build-go:
	CGO_ENABLED=1 go build ./...

build-pure:  # No Rust, no CGO
	CGO_ENABLED=0 go build -tags pure_go ./...

test:
	cargo test --manifest-path rust/kruda-json/Cargo.toml
	cargo test --manifest-path rust/kruda-validator/Cargo.toml
	CGO_ENABLED=1 go test -race ./...

bench:
	CGO_ENABLED=1 go test -bench=. -benchmem ./benchmarks/
```

### 27.10 Performance Targets (Rust vs Go)

```
Component        stdlib (Go)   Sonic (Go)    Rust          Improvement
──────────────────────────────────────────────────────────────────────
JSON parse       ~5 μs         ~1.5 μs       ~0.8 μs       6.2x vs stdlib
Regex validate   ~1.2 μs       N/A           ~0.15 μs      8x faster
Route matching   ~120 ns       N/A           ~45 ns        2.7x faster
```

### 27.11 CGO Overhead Consideration

CGO has ~70ns overhead per call. This is acceptable because:
- JSON parse takes 800-5000ns → 70ns overhead = 1.4-8.7% (negligible)
- Route matching takes 45-120ns → 70ns overhead = significant → batch calls or use rustgo technique
- Go 1.26 reduced CGO overhead by ~30% → now ~50ns

For route matching where CGO overhead matters, consider using the `rustgo` technique (direct assembly call, ~2% overhead) or batching multiple operations per CGO call.

### 27.12 Critical Rule: Always Have Go Fallback

Every Rust component MUST have a pure Go equivalent. This is non-negotiable:

| Rust Component | Go Fallback | Last Resort |
|---------------|-------------|-------------|
| simd-json (Rust) | Sonic (Go+asm) | encoding/json |
| regex crate (Rust) | regexp (Go) | string matching |
| radix tree (Rust) | radix tree (Go) | — (same algorithm) |

**Build tag architecture is detailed in Section 27.4.**

Reasons this rule exists:
1. **Cross-compilation** — `GOOS=windows GOARCH=arm64` must work without Rust toolchain
2. **CI simplicity** — GitHub Actions can test with just Go installed
3. **Docker** — Minimal Alpine images don't have Rust/glibc
4. **Contributor experience** — 90% of PRs don't touch Rust, shouldn't need Rust installed
5. **Debugging** — Users can `go build -tags pure_go` to isolate whether a bug is in Rust or Go

---

## 28. Competitive Strategy

### 28.1 Positioning Statement

> **Kruda is the first Go framework that combines type-safe generics, auto CRUD, auto OpenAPI, fasthttp performance, Rust acceleration, AI-native DX, automatic MCP server, and security-by-default in one package.**
>
> No framework in Go — or any language — offers all of these together.

### 28.2 v1.0 Pure Go Advantage

A common misconception is that Kruda needs Rust to compete. The truth: **v1.0 pure Go already beats every major Go framework** through superior design decisions alone.

| Design Decision | What Kruda Does | What Competitors Do | Performance Gain |
|----------------|----------------|--------------------|-----------------:|
| JSON engine | Sonic (JIT+asm) | encoding/json (Gin) | +3.3x |
| Response buffers | sync.Pool'd | Allocated per request (Gin) | -350 B/op |
| Context | Pool + Reset | New struct (Gin) or unsafe reuse (Fiber) | -200 B/op |
| Static routes | O(1) map first | Radix tree for everything | +100ns on 80% routes |
| Query parsing | Lazy (on access) | Eager (on every request) | +200ns if unused |
| Type binding | Generics (compile) | reflect (runtime) | less overhead |
| Context safety | Safe in goroutines | c.Copy() required (Fiber) | no footgun |

**v1.0 vs competitors (projected):**
```
Framework      req/sec    latency    alloc/op   safe?
──────────────────────────────────────────────────────
Kruda v1.0     520K       1.9 μs     0 B        ✅
Fiber          505K       2.0 μs     0 B        ⚠️ (CVEs)
Echo           410K       2.4 μs     200 B      ✅
Gin            380K       2.6 μs     350 B      ✅
```

**v2.0 Rust widens the gap, not creates it:**
```
Kruda v2.0     680K+      1.4 μs     0 B        ✅ (+Rust)
```

Marketing phases:
- **v1.0 launch:** "The fastest _type-safe_ Go framework"
- **v2.0 launch:** "Now with Rust-powered internals — 6x faster JSON" ← second news cycle

### 28.3 How Kruda Wins Against Each Competitor

**vs Fiber (35K+ stars) — Win on: Safety + DX**
- Fiber's context reuse bugs and 4 CVEs are real production risks
- Kruda's typed handlers reduce 70% boilerplate while being safer
- Marketing angle: "Fiber speed without Fiber's security risks"

**vs Gin (80K+ stars) — Win on: Modern DX + Auto-everything**
- Gin has no generics, no auto OpenAPI, no auto CRUD — it's a 2014 design
- Kruda is what Gin would look like if designed in 2026 with Go 1.25+ generics
- Marketing angle: "Gin was great. Kruda is next."

**vs Echo (30K+ stars) — Win on: Type safety + Auto CRUD**
- Echo is solid but lacks generics integration and auto-generation
- Kruda does everything Echo does + typed handlers + Resource()
- Marketing angle: "Echo's reliability with zero boilerplate"

**vs Hertz (CloudWeGo) — Win on: DX + International docs**
- Same fasthttp performance, but Hertz DX is verbose and docs are primarily Chinese
- Marketing angle: "fasthttp performance with world-class DX"

**vs Fuego (~1K stars) — Win on: Complete platform**
- Fuego proved typed handlers + auto OpenAPI works in Go
- Kruda adds: auto CRUD, error mapping, fasthttp, Rust acceleration, security, concurrency helpers, CLI
- Marketing angle: "The full platform, not just typed handlers"

**vs FastAPI (Python) — Win on: Performance + True type safety**
- 5-15x faster, single binary deploy, compile-time type checking
- Marketing angle: "FastAPI developer experience at Go speed"

**vs Elysia (Bun/TS) — Win on: Performance + Stability**
- 2-3x faster, lower memory, Go's stability vs Bun's immaturity
- Marketing angle: "Elysia's elegance, Go's reliability"

**vs Actix/Axum (Rust) — Win on: DX + Learning curve**
- 95% of Rust performance with 10% of Rust complexity
- Marketing angle: "Why fight the borrow checker when Go is fast enough?"

### 28.4 Core Differentiation Matrix

```
What ONLY Kruda has (no other Go framework):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Auto CRUD (Resource)            — One line, five endpoints
2. Rust acceleration               — 6x faster JSON via simd-json FFI
3. Error mapping                   — app.MapError() auto HTTP status
4. fasthttp + net/http switchable   — Best of both worlds
5. Concurrency helpers             — Parallel, Race, Each built-in
6. Security by default             — Headers, limits, CRLF, UUID fail-fast
7. Complete lifecycle hooks        — onRequest → beforeHandle → afterHandle → onResponse
8. Context safety guarantee        — All values safe in goroutines (unlike Fiber)
9. Built-in type-safe DI           — Give/Use/Module, auto-cleanup, health check, no external libs
10. AI-Friendly DX                 — llms.txt, .cursor/rules, .claude/, copilot-instructions
11. Auto MCP Server                — One line turns API into AI agent tool server
12. 1 line = 5 endpoints + 5 MCP   — Resource() + WithMCP() = REST + AI in one shot
    tools + OpenAPI docs
```

> Items 10-12 are **unique across ALL languages**, not just Go. No web framework — Python, TypeScript, Rust, Java — ships with built-in AI assistant configuration or automatic MCP server generation.

### 28.5 Launch Content Strategy

| # | Blog Post | Target Audience | Platform |
|---|-----------|-----------------|----------|
| 1 | "Introducing Kruda — Type-Safe Go with Auto-Everything" | All Go devs | dev.to, reddit, HN |
| 2 | "85 Lines to 5 Lines — CRUD APIs in Go" | Fiber, Gin, Echo users | dev.to |
| 3 | "Fiber's 4 CVEs and Why Context Safety Matters" | Fiber users | blog |
| 4 | "How We Made Go JSON 6x Faster with Rust" | Performance enthusiasts | HN, reddit |
| 5 | "Go Framework Benchmark 2026 — Honest Numbers" | All Go devs | blog + GitHub |
| 6 | "FastAPI Dev? Here's Your Go Equivalent" | Python devs | dev.to |
| 7 | "From Gin/Echo/Fiber to Kruda — Migration Guide" | Existing Go devs | docs |
| **8** | **"The First AI-Native Go Framework — Cursor, Claude Code, and MCP Built In"** | **AI-assisted devs** | **HN, reddit, dev.to** |
| **9** | **"One Line Turns Your Go API into an AI Agent Tool (MCP)"** | **AI/agent community** | **HN, reddit** |
| **10** | **"llms.txt for Frameworks: Teaching AI to Write Your Code"** | **Framework authors** | **blog** |

### 28.6 Success Metrics

| Metric | 3 months | 6 months | 12 months |
|--------|----------|----------|-----------|
| GitHub stars | 500 | 2,000 | 5,000 |
| Contributors | 5 | 15 | 30 |
| Production users | 3 | 20 | 100 |
| Discord members | 50 | 200 | 500 |
| "best Go framework" search | Mentioned | Top 10 | Top 5 |

### 28.7 Long-term Vision

```
Year 1:  Establish as "the type-safe Go framework"
         — Win the niche of developers who want Elysia/tRPC DX in Go

Year 2:  Become a top-5 Go framework by stars
         — Rust acceleration as unique selling point
         — TS client SDK for full-stack type safety

Year 3:  Production standard for new Go API projects
         — Companies choose Kruda over Gin/Echo for new projects
         — "Kruda" becomes synonymous with "type-safe Go API"
```

---

## 29. AI-Friendly Developer Experience

### 29.1 Overview

In 2025-2026, a majority of developers use AI coding assistants (Cursor, GitHub Copilot, Claude Code, Windsurf) daily. A framework that AI can't understand is a framework developers won't adopt — because the AI will generate wrong code, developers get frustrated, and switch to a framework the AI knows (Gin, Echo).

Kruda solves this by being **AI-native from day one**: structured documentation, machine-readable specs, and IDE-specific configuration files that teach AI assistants how to write correct Kruda code.

> **Principle:** If a developer asks Cursor "create a REST API with Kruda", the generated code should compile and run on the first try.

### 29.2 `llms.txt` — Machine-Readable Framework Description

Following the emerging `llms.txt` standard, Kruda publishes concise, AI-optimized documentation at well-known URLs:

```
https://kruda.dev/llms.txt          ← compact summary (~2K tokens)
https://kruda.dev/llms-full.txt     ← complete reference (~15K tokens)
```

**`llms.txt` format:**

```markdown
# Kruda — Type-Safe Go Web Framework

> Kruda is a high-performance Go web framework built for the generics era.
> It features typed handlers, auto CRUD, auto OpenAPI, built-in DI,
> zero-allocation performance, and optional Rust acceleration.

## Install
go get github.com/go-kruda/kruda

## Quick Start
package main

import "github.com/go-kruda/kruda"

func main() {
    app := kruda.New()
    app.Get("/", func(c *kruda.Ctx) error {
        return c.JSON(kruda.Map{"hello": "world"})
    })
    app.Listen(":3000")
}

## Core Concepts

### Routes
app.Get("/path", handler)
app.Post("/path", handler)
app.Put("/path", handler)
app.Delete("/path", handler)
app.Group("/api").Get("/users", handler)

### Typed Handlers (generics)
type CreateUserReq struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

app.Post("/users", kruda.Typed(func(c *kruda.Ctx, in *CreateUserReq) error {
    // in is already parsed + validated
    return c.JSON(user)
}))

### Auto CRUD
app.Resource("/users", userService)
// Creates: GET /users, GET /users/:id, POST /users, PUT /users/:id, DELETE /users/:id

### Dependency Injection
kruda.Give(app, func() (*DB, error) { return NewDB() })  // register
db := kruda.Use[*DB](app)                                 // retrieve

### Request-Scoped DI
kruda.Provide[CurrentUser](app, func(c *kruda.Ctx) (*CurrentUser, error) {
    return verifyJWT(c.Header("Authorization"))
})
user := kruda.Need[CurrentUser](c)  // in handler

### Context API
c.JSON(data)           // send JSON
c.Status(201).JSON(d)  // with status code
c.Query("key")         // query parameter
c.Param("id")          // path parameter
c.Header("X-Key")      // request header
c.BodyBytes()          // raw body

### Middleware
app.Use(kruda.Logger())
app.Use(kruda.Recovery())
app.Use(kruda.CORS())
app.Use(kruda.RequestID())
app.Guard(jwt.New(jwt.Config{Secret: "..."}))  // from contrib/jwt

### Error Handling
app.MapError(ErrNotFound, 404, "not found")
app.MapError(ErrForbidden, 403, "forbidden")
// Handlers just return errors — framework maps to HTTP status

### Concurrency
r1, r2, err := kruda.Parallel2(c, fetchUser, fetchOrders)

## Common Patterns

### REST API with auth
api := app.Group("/api")
api.Guard(jwt.New(jwt.Config{Secret: secret}))
api.Resource("/users", userService)
api.Resource("/products", productService)

### Custom middleware
func RateLimit(max int) kruda.MiddlewareFunc {
    return func(c *kruda.Ctx) error {
        // rate limit logic
        return c.Next()
    }
}

## Do NOT confuse with
- Gin: c.JSON(200, data) → Kruda: c.JSON(data) (status via c.Status())
- Gin: c.ShouldBindJSON(&req) → Kruda: kruda.Bind[T](c) or typed handler
- Fiber: c.Copy() needed → Kruda: context values always safe in goroutines
- Echo: e.GET() uppercase → Kruda: app.Get() lowercase
```

### 29.3 IDE & AI Assistant Configuration Files

Kruda ships ready-made configuration files in every `kruda new` scaffolded project:

**`.cursor/rules` — Cursor AI rules:**

```yaml
# .cursor/rules
description: Kruda Go web framework conventions

rules:
  - Import path is "github.com/go-kruda/kruda"
  - Use kruda.Typed() for handlers that parse request body
  - Use kruda.Bind[T](c) for manual binding, NOT c.ShouldBindJSON (that's Gin)
  - Use kruda.Give/Use for DI, NOT wire or dig
  - Error handling: return error from handler, map with app.MapError()
  - JSON response: c.JSON(data), NOT c.JSON(200, data) (that's Gin)
  - Status code: c.Status(201).JSON(data), method chaining
  - Groups: app.Group("/api").Use(middleware).Get("/users", handler)
  - Auto CRUD: app.Resource("/path", service) creates 5 endpoints
  - Contrib imports: "github.com/go-kruda/kruda/contrib/jwt"
  - Built-in middleware: kruda.Logger(), kruda.Recovery(), kruda.CORS()
  - Context is safe in goroutines — no c.Copy() needed (unlike Fiber)
  - Middleware order: Logger → Recovery → CORS → RequestID → Auth → Handler
```

**`.claude/INSTRUCTIONS.md` — Claude Code instructions:**

```markdown
# Kruda Framework — Claude Code Instructions

## Import
import "github.com/go-kruda/kruda"

## Route Registration
app.Get("/path", handler)
app.Post("/path", handler)  // simple handler
app.Post("/path", kruda.Typed(typedHandler))  // typed handler

## Handler Signatures
// Simple handler:
func(c *kruda.Ctx) error

// Typed handler (auto-parse + validate body):
func(c *kruda.Ctx, in *RequestStruct) error

## Response
c.JSON(data)              // 200 + JSON
c.Status(201).JSON(data)  // 201 + JSON
c.String("hello")         // 200 + text
c.NoContent()             // 204

## DO NOT use Gin/Echo/Fiber patterns:
- ❌ c.JSON(200, data)       → ✅ c.JSON(data)
- ❌ c.ShouldBindJSON(&req)  → ✅ kruda.Bind[T](c)
- ❌ e.GET("/path", handler) → ✅ app.Get("/path", handler)
- ❌ c.Copy() in goroutines  → ✅ just use values directly
```

**`.github/copilot-instructions.md` — GitHub Copilot:**

```markdown
When writing code for this Kruda project:
- This is a Kruda web framework project, not Gin/Echo/Fiber
- Use kruda.Typed() for request body parsing with generics
- Use c.JSON(data) not c.JSON(statusCode, data)
- Use app.Resource() for CRUD endpoints
- Use kruda.Give/Use for dependency injection
- Built-in middleware: kruda.Logger(), kruda.Recovery(), kruda.CORS()
```

### 29.4 Rich godoc with Examples

Every public function includes runnable examples that AI assistants can reference:

```go
// Get registers a new GET route with the given path and handler.
//
// The handler receives a [Ctx] with request/response methods.
// Return nil for success, or an error that will be mapped via [App.MapError].
//
// Example:
//
//	app.Get("/users", func(c *kruda.Ctx) error {
//	    users, err := db.ListUsers(c.Context())
//	    if err != nil {
//	        return err
//	    }
//	    return c.JSON(users)
//	})
//
//	// With path parameter:
//	app.Get("/users/:id", func(c *kruda.Ctx) error {
//	    id := c.Param("id")
//	    user, err := db.GetUser(c.Context(), id)
//	    if err != nil {
//	        return err  // auto-mapped to 404 if registered
//	    }
//	    return c.JSON(user)
//	})
func (app *App) Get(path string, handler HandlerFunc, opts ...RouteOption) *Route
```

**Rule:** Every exported function, type, and method MUST have at least one `Example` in godoc. AI assistants scrape godoc for context — missing examples = wrong code generation.

### 29.5 Examples Repository

A comprehensive examples repo that AI assistants can reference for full patterns:

```
kruda-examples/                          ← github.com/go-kruda/kruda/examples
├── README.md                            ← AI reads this first (index + descriptions)
├── 01-hello-world/
│   └── main.go                          ← minimal 10-line example
├── 02-rest-api/
│   ├── main.go                          ← routes, handlers, JSON
│   └── README.md
├── 03-typed-handlers/
│   ├── main.go                          ← generics, validation, typed request/response
│   └── README.md
├── 04-auth-jwt/
│   ├── main.go                          ← JWT auth with contrib/jwt
│   └── README.md
├── 05-crud-resource/
│   ├── main.go                          ← app.Resource() auto CRUD
│   └── README.md
├── 06-dependency-injection/
│   ├── main.go                          ← Give/Use/Module patterns
│   └── README.md
├── 07-middleware-custom/
│   ├── main.go                          ← writing custom middleware
│   └── README.md
├── 08-error-handling/
│   ├── main.go                          ← MapError, custom errors
│   └── README.md
├── 09-mcp-server/
│   ├── main.go                          ← expose app as MCP server
│   └── README.md
├── 10-fullstack/
│   ├── main.go                          ← API + static + WebSocket
│   └── README.md
└── 11-production/
    ├── main.go                          ← graceful shutdown, health check, logging
    ├── Dockerfile
    └── README.md
```

**README.md format (AI-optimized):**

```markdown
# Kruda Examples

| Example | Description | Key Concepts |
|---------|-------------|-------------|
| 01-hello-world | Minimal server | New, Get, Listen |
| 02-rest-api | REST CRUD API | Routes, JSON, Status codes |
| 03-typed-handlers | Generic typed handlers | Bind[T], validation, typed response |
| 04-auth-jwt | JWT authentication | Guard, contrib/jwt, protected routes |
| 05-crud-resource | Auto CRUD | Resource(), service interface |
| 06-dependency-injection | DI patterns | Give, Use, Module, Provide, Need |
| 07-middleware-custom | Custom middleware | Use, Next(), middleware chain |
| 08-error-handling | Error mapping | MapError, KrudaError, error chain |
| 09-mcp-server | MCP AI integration | contrib/mcp, auto tool generation |
| 10-fullstack | Full application | Static, WebSocket, API, auth |
| 11-production | Production setup | Graceful shutdown, Docker, health |
```

### 29.6 CLI Scaffolding with AI Config

`kruda new` generates projects with all AI config files pre-configured:

```bash
$ kruda new my-api

Creating my-api/
├── main.go
├── go.mod
├── .cursor/
│   └── rules                    ← Cursor AI rules
├── .claude/
│   └── INSTRUCTIONS.md          ← Claude Code instructions
├── .github/
│   └── copilot-instructions.md  ← GitHub Copilot instructions
├── .vscode/
│   └── settings.json            ← Go extension config
├── Dockerfile
├── Makefile
└── README.md

✅ Project created! Run:
  cd my-api && go run .
```

No other Go framework ships AI configuration out of the box. Developers who use Cursor/Copilot/Claude Code get correct code generation from minute one.

---

## 30. MCP Server Plugin

### 30.1 Overview

The Model Context Protocol (MCP) is an open standard (created by Anthropic, adopted by OpenAI, Google, and the Linux Foundation) that allows AI agents to interact with external tools and data sources. Kruda's MCP plugin auto-generates MCP tools from typed handlers, making any Kruda app instantly accessible to AI agents.

> **One line of code turns a Kruda app into an MCP server.**
> No other web framework in any language offers this level of AI integration.

```go
import "github.com/go-kruda/kruda/contrib/mcp"

app := kruda.New()
app.Resource("/users", userService)
app.Resource("/products", productService)

// One line → app becomes an MCP server
app.Use(mcp.New())

app.Listen(":3000")
// REST API at :3000
// MCP server at :3000/mcp (Streamable HTTP)
// MCP SSE at :3000/mcp/sse (legacy SSE transport)
```

### 30.2 Why Kruda Can Auto-Generate MCP (Others Can't)

MCP tools require structured metadata: name, description, input schema (JSON Schema), output schema. Most frameworks can't provide this because they use `any` types everywhere:

```go
// Gin — handler accepts any. What's the input type? Unknown at runtime.
r.POST("/users", func(c *gin.Context) {
    var req CreateUserReq
    c.ShouldBindJSON(&req)  // req type invisible to framework
})

// Kruda — handler declares types via generics. Framework knows everything.
app.Post("/users", kruda.Typed(func(c *kruda.Ctx, in *CreateUserReq) error {
    return c.JSON(user)
}))
// Kruda knows: input=CreateUserReq, output=User, method=POST, path=/users
```

**What Kruda knows at startup (that other frameworks don't):**

| Information | Gin/Echo/Fiber | Kruda |
|------------|:-:|:-:|
| Route path + method | ✅ | ✅ |
| Request body type | ❌ | ✅ (generics) |
| Response body type | ❌ | ✅ (generics) |
| Validation rules | ❌ | ✅ (struct tags) |
| Field descriptions | ❌ | ✅ (struct tags) |
| OpenAPI schema | ❌ manual | ✅ auto |
| MCP tool schema | ❌ manual | ✅ auto |

This means Kruda can automatically generate both OpenAPI docs AND MCP tools from the same type information — zero annotation, zero configuration.

### 30.3 How It Works

#### Step 1: Route Scanning

At startup, the MCP plugin scans all registered routes:

```go
func (p *MCPPlugin) Install(app *kruda.App) error {
    routes := app.Routes()  // returns []RouteInfo with type metadata

    for _, r := range routes {
        if !p.shouldExpose(r) {
            continue
        }
        tool := p.routeToTool(r)
        p.tools = append(p.tools, tool)
    }

    // Register MCP endpoints
    app.Post("/mcp", p.handleStreamableHTTP)
    app.Get("/mcp/sse", p.handleSSE)

    return nil
}
```

#### Step 2: Route → MCP Tool Conversion

```go
func (p *MCPPlugin) routeToTool(r kruda.RouteInfo) Tool {
    return Tool{
        Name:        p.toolName(r),                  // POST /users → "create_user"
        Description: r.Description,                   // from OpenAPI description
        InputSchema: p.typeToJSONSchema(r.InputType), // Go struct → JSON Schema
        Annotations: ToolAnnotations{
            Title:           r.Summary,
            ReadOnlyHint:    r.Method == "GET",
            DestructiveHint: r.Method == "DELETE",
            OpenWorldHint:   false,
        },
    }
}

// Naming convention:
// GET    /users       → "list_users"
// GET    /users/:id   → "get_user"
// POST   /users       → "create_user"
// PUT    /users/:id   → "update_user"
// DELETE /users/:id   → "delete_user"
// GET    /products    → "list_products"
// POST   /orders/:id/cancel → "cancel_order"
```

#### Step 3: Go Struct → JSON Schema (automatic)

```go
type CreateUserReq struct {
    Name  string `json:"name"  validate:"required"     desc:"User's full name"`
    Email string `json:"email" validate:"required,email" desc:"User's email address"`
    Age   int    `json:"age"   validate:"min=0,max=150"  desc:"User's age"`
}

// Auto-generates:
{
    "type": "object",
    "properties": {
        "name":  {"type": "string", "description": "User's full name"},
        "email": {"type": "string", "format": "email", "description": "User's email address"},
        "age":   {"type": "integer", "minimum": 0, "maximum": 150, "description": "User's age"}
    },
    "required": ["name", "email"]
}
```

#### Step 4: Tool Execution

When an AI agent calls a tool, the MCP plugin routes it to the corresponding handler:

```go
func (p *MCPPlugin) executeTool(name string, args json.RawMessage) (any, error) {
    tool := p.findTool(name)
    if tool == nil {
        return nil, fmt.Errorf("tool not found: %s", name)
    }

    // Create an internal request to the route
    req := buildHTTPRequest(tool.Route, args)
    resp := captureResponse()

    // Execute through the full middleware chain (auth, validation, etc.)
    p.app.ServeHTTP(resp, req)

    return resp.Body(), nil
}
```

**Critical:** Tool execution goes through the full middleware chain. This means auth, validation, rate limiting, logging — everything works exactly as if a normal HTTP request came in. No bypassing security.

### 30.4 Configuration

```go
// Zero-config (expose all routes):
app.Use(mcp.New())

// Custom config:
app.Use(mcp.New(mcp.Config{
    // Server identity
    Name:        "my-app",
    Version:     "1.0.0",
    Description: "User and Product management API",

    // Transport
    Path:        "/mcp",           // default: "/mcp"
    EnableSSE:   true,             // default: true (legacy transport)

    // Security
    RequireAuth:  true,            // require auth for MCP calls
    AuthHeader:   "Authorization", // forward this header from AI agent

    // Filtering
    Include:     []string{"/api/*"},          // only expose /api/* routes
    Exclude:     []string{"/admin/*"},        // never expose /admin/*
    OnlyTagged:  false,                       // if true, only expose routes with WithMCP()
}))
```

### 30.5 Selective Exposure with `WithMCP()`

For fine-grained control, tag individual routes:

```go
// Exposed to AI agents (with custom description):
app.Get("/users", listUsers, kruda.WithMCP("List all users with optional pagination"))
app.Post("/users", createUser, kruda.WithMCP("Create a new user account"))

// NOT exposed (no WithMCP tag):
app.Delete("/users/:id", deleteUser)
app.Post("/admin/reset", resetSystem)
```

With `mcp.Config{OnlyTagged: true}`, only routes with `WithMCP()` are exposed.

### 30.6 Resource() + MCP — The Ultimate Combo

```go
// 1 line of Go code:
app.Resource("/users", userService, kruda.WithMCP("User management"))

// Generates:
//
// REST Endpoints:          MCP Tools:                    OpenAPI:
// GET    /users         →  list_users                 →  GET /users documented
// GET    /users/:id     →  get_user                   →  GET /users/{id} documented
// POST   /users         →  create_user                →  POST /users documented
// PUT    /users/:id     →  update_user                →  PUT /users/{id} documented
// DELETE /users/:id     →  delete_user                →  DELETE /users/{id} documented
//
// Total: 5 REST endpoints + 5 MCP tools + 5 OpenAPI operations
// From: 1 line of code
```

### 30.7 MCP Protocol Implementation

Kruda implements MCP using the **Streamable HTTP** transport (current standard) with **SSE** fallback:

```
AI Agent (Claude, ChatGPT, Cursor)
  │
  │  POST /mcp  (JSON-RPC 2.0 over HTTP)
  │
  ▼
Kruda MCP Endpoint
  │
  ├─ initialize        → return server info + capabilities
  ├─ tools/list         → return all exposed tools with schemas
  ├─ tools/call         → execute tool → internal HTTP request → return result
  ├─ resources/list     → return available resources (optional)
  └─ resources/read     → read resource content (optional)
```

**Supported MCP features:**

| MCP Feature | Kruda Support | Details |
|------------|:---:|---------|
| Tools | ✅ | Auto-generated from routes |
| Resources | ✅ | Static files, config, DB schemas |
| Prompts | 🟡 | Optional, manual registration |
| Sampling | ❌ | Not applicable (server-side) |
| Streamable HTTP | ✅ | Primary transport |
| SSE (legacy) | ✅ | Backward compatibility |
| STDIO | ❌ | Not needed for web framework |
| Auth (OAuth 2.0) | ✅ | Via existing auth middleware |

### 30.8 Real-World Example: AI Agent Using Kruda App

**Scenario:** A developer has a Kruda API for managing a todo app. They connect it to Claude Desktop as an MCP server.

```go
// server.go
package main

import (
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/contrib/mcp"
)

type Todo struct {
    ID    int    `json:"id"`
    Title string `json:"title" validate:"required" desc:"Todo item title"`
    Done  bool   `json:"done" desc:"Whether the todo is completed"`
}

type CreateTodoReq struct {
    Title string `json:"title" validate:"required" desc:"Title for the new todo"`
}

func main() {
    app := kruda.New()

    // Auto CRUD
    app.Resource("/todos", &TodoService{}, kruda.WithMCP("Todo list management"))

    // Enable MCP
    app.Use(mcp.New(mcp.Config{
        Name:        "todo-app",
        Description: "A simple todo list manager",
    }))

    app.Listen(":3000")
}
```

**Claude Desktop config (`claude_desktop_config.json`):**

```json
{
    "mcpServers": {
        "todo-app": {
            "url": "http://localhost:3000/mcp"
        }
    }
}
```

**User conversation with Claude:**

```
User: "Show me all my todos"
Claude: [calls list_todos tool] → You have 3 todos:
  1. ✅ Buy groceries
  2. ❌ Write blog post
  3. ❌ Review PR #42

User: "Mark the blog post as done"
Claude: [calls update_todo tool {id: 2, done: true}] → Done! "Write blog post" is now marked as complete.

User: "Add a new todo: deploy v2.0"
Claude: [calls create_todo tool {title: "deploy v2.0"}] → Created! New todo "deploy v2.0" added with ID 4.
```

### 30.9 Security Considerations

MCP exposes application functionality to AI agents. Security is critical:

**1. Auth passthrough:**
```go
// AI agent sends auth token → MCP plugin forwards to internal request
// Existing auth middleware validates as normal
app.Guard(jwt.New(jwt.Config{Secret: secret}))
app.Use(mcp.New(mcp.Config{RequireAuth: true}))
```

**2. Read-only mode:**
```go
app.Use(mcp.New(mcp.Config{
    ReadOnly: true,  // only GET routes exposed as tools
}))
```

**3. Rate limiting:**
```go
// MCP requests go through same middleware chain
app.Use(ratelimit.New(ratelimit.Config{Max: 100, Window: time.Minute}))
app.Use(mcp.New())
// AI agents are rate-limited same as any client
```

**4. Audit logging:**
```go
app.Use(mcp.New(mcp.Config{
    OnToolCall: func(toolName string, args json.RawMessage, agentID string) {
        slog.Info("MCP tool call",
            "tool", toolName,
            "agent", agentID,
            "args", string(args),
        )
    },
}))
```

**5. Explicit exclusion of dangerous routes:**
```go
app.Use(mcp.New(mcp.Config{
    Exclude: []string{
        "/admin/*",
        "/internal/*",
        "DELETE /users/*",  // allow read/create/update but not delete
    },
}))
```

### 30.10 Package Structure

```
kruda-contrib/
└── mcp/                              ← github.com/go-kruda/kruda/contrib/mcp
    ├── go.mod                         ← depends on kruda core + MCP SDK
    ├── mcp.go                         ← main plugin: New(), Config
    ├── scanner.go                     ← route scanning + tool generation
    ├── schema.go                      ← Go types → JSON Schema conversion
    ├── transport_streamable.go        ← Streamable HTTP handler
    ├── transport_sse.go               ← SSE transport handler (legacy)
    ├── executor.go                    ← tool execution → internal HTTP
    ├── naming.go                      ← route → tool name conventions
    ├── security.go                    ← auth passthrough, filtering
    └── mcp_test.go                    ← tests
```

### 30.11 Competitive Advantage

**No web framework in any language offers automatic MCP generation:**

| Framework | Language | Auto OpenAPI | Auto MCP |
|-----------|----------|:---:|:---:|
| Gin | Go | ❌ | ❌ |
| Echo | Go | ❌ | ❌ |
| Fiber | Go | ❌ | ❌ |
| Fuego | Go | ✅ | ❌ |
| **Kruda** | **Go** | **✅** | **✅** |
| FastAPI | Python | ✅ | ❌ |
| Elysia | TS/Bun | ✅ | ❌ |
| Express | JS/Node | ❌ | ❌ |
| Actix | Rust | ❌ | ❌ |
| Spring Boot | Java | ✅ | ❌ |

**Marketing angle:** "The first web framework with built-in AI agent support. One line turns your API into an MCP server."

This is a unique selling point that no competitor can easily replicate — it requires typed handlers (generics) + auto schema generation as prerequisites, which only Kruda has in the Go ecosystem.

---

## Appendix A: Example Application

```go
package main

import (
    "github.com/go-kruda/kruda"
    "gorm.io/gorm"
)

// Models
type User struct {
    ID    int    `json:"id"    gorm:"primarykey"`
    Name  string `json:"name"`
    Email string `json:"email" gorm:"uniqueIndex"`
}

type CreateUserReq struct {
    Name  string `json:"name"  validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
}

type UpdateUserReq struct {
    Name  string `json:"name"  validate:"omitempty,min=2"`
    Email string `json:"email" validate:"omitempty,email"`
}

// Service
type UserService struct{ db *gorm.DB }

func (s *UserService) List(q kruda.ListQuery) ([]User, int, error) {
    var users []User
    var total int64
    query := s.db.Model(&User{})
    if q.Search != "" {
        query = query.Where("name ILIKE ? OR email ILIKE ?", "%"+q.Search+"%", "%"+q.Search+"%")
    }
    query.Count(&total)
    query.Order(q.Sort + " " + q.Order).Offset(q.Offset()).Limit(q.Limit).Find(&users)
    return users, int(total), nil
}

func (s *UserService) Get(id int) (*User, error) {
    var u User
    return &u, s.db.First(&u, id).Error
}

func (s *UserService) Create(body CreateUserReq) (*User, error) {
    u := User{Name: body.Name, Email: body.Email}
    return &u, s.db.Create(&u).Error
}

func (s *UserService) Update(id int, body UpdateUserReq) (*User, error) {
    var u User
    if err := s.db.First(&u, id).Error; err != nil {
        return nil, err
    }
    return &u, s.db.Model(&u).Updates(body).Error
}

func (s *UserService) Delete(id int) error {
    return s.db.Delete(&User{}, id).Error
}

// Main
func main() {
    db := connectDB()
    
    app := kruda.New()
    
    // Error mapping
    app.MapError(gorm.ErrRecordNotFound, 404, "not found")
    app.MapError(gorm.ErrDuplicatedKey, 409, "already exists")
    
    // Global middleware
    app.Use(
        kruda.Recovery(),
        kruda.RequestID(),
        kruda.Logger(),
        kruda.CORS(),
    )
    
    // Health check
    app.Get("/health", func(c *kruda.Ctx) error {
        return c.JSON(kruda.Map{"status": "ok"})
    })
    
    // API routes
    app.Group("/api/v1").
        Guard(kruda.JWT(jwtSecret)).
        Resource("/users", &UserService{db: db}).
        Get("/me", func(c *kruda.Ctx) error {
            user := kruda.Need[User](c)
            return c.JSON(user)
        })
    
    // Auto docs
    app.Use(kruda.Swagger(kruda.SwaggerConfig{
        Title:   "Kruda Example API",
        Version: "1.0.0",
        Path:    "/docs",
    }))
    
    app.Listen(":3000")
}
```

---

## Appendix B: Performance Comparison Code

```go
// benchmarks/bench_test.go
package benchmarks

import (
    "testing"
    "github.com/go-kruda/kruda"
)

func BenchmarkKrudaHelloWorld(b *testing.B) {
    app := kruda.New()
    app.Get("/hello", func(c *kruda.Ctx) error {
        return c.Text("Hello, World!")
    })
    
    req := kruda.TestReq{Method: "GET", Path: "/hello"}
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            app.Test(req)
        }
    })
}

func BenchmarkKrudaJSON(b *testing.B) {
    type Resp struct {
        Message string `json:"message"`
    }
    
    app := kruda.New()
    app.Get("/json", func(c *kruda.Ctx) error {
        return c.JSON(Resp{Message: "Hello, World!"})
    })
    
    req := kruda.TestReq{Method: "GET", Path: "/json"}
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            app.Test(req)
        }
    })
}

func BenchmarkKrudaTypedHandler(b *testing.B) {
    type Body struct {
        Name  string `json:"name"  validate:"required"`
        Email string `json:"email" validate:"required,email"`
    }
    type Res struct {
        ID    int    `json:"id"`
        Name  string `json:"name"`
    }
    
    app := kruda.New()
    kruda.Post[Body, Res](app, "/users", func(c *kruda.C[Body]) (*Res, error) {
        return &Res{ID: 1, Name: c.In.Name}, nil
    })
    
    req := kruda.TestReq{
        Method: "POST",
        Path:   "/users",
        Body:   Body{Name: "test", Email: "test@test.com"},
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            app.Test(req)
        }
    })
}
```

---

## Appendix C: Claude Code Instructions

When developing Kruda with Claude Code, follow these guidelines:

### C.1 Development Order

1. Start with `kruda.go`, `config.go`, `context.go` — these are the foundation
2. Then `router.go` — the radix tree router is critical for performance
3. Then `handler.go` — generic typed handlers
4. Then `bind.go` + `validation.go` — the DX magic
5. Then `error.go` — error mapping system
6. Then `container.go` — DI container (Give/Use/Module/auto-cleanup/health)
7. Then `resource.go` — auto CRUD
8. Then built-in middleware: `middleware/logger.go`, `recovery.go`, `cors.go`, `requestid.go`
9. Then contrib packages: `contrib/jwt/`, `contrib/ratelimit/`, `contrib/swagger/`
10. Then `transport/fasthttp.go` — can start with net/http and add fasthttp later
11. **Rust components come last (Phase 7/v2.0)** — everything must work in pure Go first

### C.2 DX Design Principles

> **The #1 goal of Kruda is developer experience.** Every API decision should be measured by: "Can this be simpler?"

Key DX targets to maintain:
- **Input access:** `c.In` as a direct field — already parsed, validated, ready to use. No extra function calls.
- **Error handling:** `app.MapError()` — register once, auto-map everywhere. Handlers just return errors.
- **CRUD:** `app.Resource("/users", svc)` — one line creates 5 endpoints.
- **Chaining:** `app.Group().Guard().Resource()` — fluent, readable, composable.
- **Concurrency:** `Parallel2(c, f1, f2)` — typed results, no type assertion.
- **DI:** `Give(app, fn)` + `Use[T](app)` for app-level singletons, `Provide[T]` + `Need[T]` for request-scoped. Modules, auto io.Closer cleanup, auto health check.
- **Zero config security:** Body limits, timeouts, headers all enabled by default.

When in doubt about API design, ask: "Is this the absolute minimum code a developer needs to write?"

### C.3 Testing Requirements

- Every file must have corresponding `_test.go`
- Benchmark tests for all hot-path functions
- Use `go test -race` to verify concurrency safety
- Context safety tests: verify string values survive after handler returns
- Include comparison benchmarks vs Fiber, Gin, Echo, Fuego, Hertz in benchmarks/
- All tests must pass with both `CGO_ENABLED=1` and `CGO_ENABLED=0`

### C.4 Code Style

- Follow standard Go conventions
- Use `slog` for structured logging
- Keep interfaces small (1-3 methods)
- Prefer functions over methods where possible
- Use functional options pattern for configuration
- Comments in English, technical terms in English
- Error messages in English

### C.5 Performance Rules

- Zero allocations on hot paths (verify with `go test -benchmem`)
- Use `sync.Pool` for frequently allocated objects
- Pre-compile validators and param parsers at startup
- Use Sonic for JSON (with stdlib fallback), Rust simd-json when available
- Profile with `pprof` before and after changes
- Never use `reflect` at request-handling time

### C.6 Safety Rules

- All string values from context must be proper copies
- Never expose internal `[]byte` buffers to users
- UUID generation must panic on crypto/rand failure
- Sanitize all header values for CRLF injection
- Validate path for traversal attacks
- Default body limit: 4MB
- Default timeouts: 30s read, 30s write

### C.7 Rust Integration Rules (Phase 7+)

- **Every Rust component MUST have a pure Go fallback**
- Use Go build tags to select implementation: `!pure_go && cgo` for Rust, `pure_go || !cgo` for Go
- Rust libraries compile to static `.a` files (static linking preferred over dynamic)
- CGO bridge must handle memory safely: Rust allocates → Go copies → Rust frees
- Run benchmarks for both Rust and Go paths to verify improvement
- Minimum improvement threshold: 2x faster to justify CGO overhead
- Test on Linux amd64 (primary), macOS arm64 (secondary), Windows (pure Go only)

### C.8 Build Commands

```bash
# Development (pure Go, fast compile)
go run ./cmd/server/

# Production (with Rust, if available)
make build

# Pure Go (no CGO, any platform)
CGO_ENABLED=0 go build -tags pure_go -o kruda-server ./cmd/server/

# Run all tests
make test

# Benchmarks with comparison
go test -bench=. -benchmem -count=5 ./benchmarks/ | tee bench.txt
```

---

*End of Kruda Framework Technical Specification*
