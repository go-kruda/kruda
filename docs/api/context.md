# Context

`Ctx` is the request/response context passed to every handler. It provides methods for reading request data and writing responses. Contexts are pooled and reused for zero-allocation on the hot path.

## Request Methods

### Path

```go
func (c *Ctx) Path() string
```

Returns the request path.

### Method

```go
func (c *Ctx) Method() string
```

Returns the HTTP method (GET, POST, etc.).

### Param

```go
func (c *Ctx) Param(key string) string
```

Returns a route parameter value.

```go
// Route: /users/:id
app.Get("/users/:id", func(c *kruda.Ctx) error {
    id := c.Param("id")
    return c.Text("User: " + id)
})
```

### ParamInt

```go
func (c *Ctx) ParamInt(name string) (int, error)
```

Returns a route parameter as an integer.

### Query

```go
func (c *Ctx) Query(name string, def ...string) string
```

Returns a query string parameter value. Optional default value.

```go
// GET /search?q=kruda&page=1
q := c.Query("q")              // "kruda"
page := c.Query("page")        // "1"
sort := c.Query("sort", "asc") // "asc" (default)
```

### QueryInt

```go
func (c *Ctx) QueryInt(name string, def ...int) int
```

Returns a query parameter as an integer with optional default.

### Header

```go
func (c *Ctx) Header(key string) string
```

Returns a request header value.

```go
token := c.Header("Authorization")
contentType := c.Header("Content-Type")
```

### Cookie

```go
func (c *Ctx) Cookie(name string) string
```

Returns a request cookie value by name.

### IP

```go
func (c *Ctx) IP() string
```

Returns the client IP address. Respects `X-Forwarded-For` / `X-Real-IP` only when `WithTrustProxy(true)` is set.

### Bind

```go
func (c *Ctx) Bind(v any) error
```

Parses the request body as JSON into the given value.

```go
var user User
if err := c.Bind(&user); err != nil {
    return kruda.NewError(400, "Invalid JSON")
}
```

### BodyBytes

```go
func (c *Ctx) BodyBytes() ([]byte, error)
```

Returns the raw request body as bytes.

### BodyString

```go
func (c *Ctx) BodyString() string
```

Returns the request body as a string.

## Response Methods

### JSON

```go
func (c *Ctx) JSON(v any) error
```

Sends a JSON response. Set status code with `Status()` before calling.

```go
return c.JSON(user)                    // 200 by default
return c.Status(201).JSON(user)        // 201 Created
```

### Text

```go
func (c *Ctx) Text(s string) error
```

Sends a plain text response.

```go
return c.Text("Hello, World!")
return c.Status(200).Text("OK")
```

### HTML

```go
func (c *Ctx) HTML(html string) error
```

Sends an HTML response.

### Status

```go
func (c *Ctx) Status(code int) *Ctx
```

Sets the response status code. Chainable.

```go
return c.Status(201).JSON(user)
```

### StatusCode

```go
func (c *Ctx) StatusCode() int
```

Returns the current response status code.

### NoContent

```go
func (c *Ctx) NoContent() error
```

Sends a 204 No Content response.

### File

```go
func (c *Ctx) File(path string) error
```

Serves a file. Requires net/http transport.

### Stream

```go
func (c *Ctx) Stream(reader io.Reader) error
```

Streams a response from a reader.

### SetHeader

```go
func (c *Ctx) SetHeader(key, value string) *Ctx
```

Sets a response header. CRLF characters in values are automatically stripped. Invalid header keys are silently skipped. Chainable.

```go
c.SetHeader("X-Custom", "value")
```

### AddHeader

```go
func (c *Ctx) AddHeader(key, value string) *Ctx
```

Adds a response header (allows multiple values for the same key). Chainable.

### SetCookie

```go
func (c *Ctx) SetCookie(cookie *Cookie) *Ctx
```

Sets a response cookie. Uses `kruda.Cookie` (not `http.Cookie`). Cookie values are sanitized. Chainable.

```go
c.SetCookie(&kruda.Cookie{
    Name:  "session",
    Value: "abc123",
    Path:  "/",
})
```

### Redirect

```go
func (c *Ctx) Redirect(url string, code ...int) error
```

Sends a redirect response. Default status is 302.

```go
return c.Redirect("/login")          // 302
return c.Redirect("/new", 301)       // 301
```

## Request-Scoped Storage

### Set / Get

```go
func (c *Ctx) Set(key string, value any)
func (c *Ctx) Get(key string) any
```

Store and retrieve request-scoped values.

### Provide

```go
func (c *Ctx) Provide(key string, value any)
```

Provides a value for request-scoped DI resolution.

## Middleware Control

### Next

```go
func (c *Ctx) Next() error
```

Calls the next handler in the middleware chain.

```go
func LogMiddleware(c *kruda.Ctx) error {
    start := time.Now()
    err := c.Next()
    slog.Info("request", "path", c.Path(), "duration", time.Since(start))
    return err
}
```

## Context & Logging

### Context

```go
func (c *Ctx) Context() context.Context
func (c *Ctx) SetContext(ctx context.Context)
```

Access or replace the underlying `context.Context`.

### Log

```go
func (c *Ctx) Log() *slog.Logger
```

Returns a request-scoped logger with request metadata.

## Timing

### MarkStart / Latency

```go
func (c *Ctx) MarkStart()
func (c *Ctx) Latency() time.Duration
```

Track request latency (used by Logger middleware).

## SSE

### SSE

```go
func (c *Ctx) SSE(fn func(*SSEStream) error) error
```

Starts a Server-Sent Events stream.

## Method Chaining

Response methods return `*Ctx` for fluent chaining:

```go
return c.Status(200).
    SetHeader("X-Request-ID", reqID).
    SetHeader("Cache-Control", "no-cache").
    JSON(data)
```
