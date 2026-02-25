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
    return c.String(200, "User: "+id)
})
```

### Query

```go
func (c *Ctx) Query(key string) string
```

Returns a query string parameter value.

```go
// GET /search?q=kruda&page=1
q := c.Query("q")       // "kruda"
page := c.Query("page") // "1"
```

### Header

```go
func (c *Ctx) Header(key string) string
```

Returns a request header value.

```go
token := c.Header("Authorization")
contentType := c.Header("Content-Type")
```

### Body

```go
func (c *Ctx) Body(v interface{}) error
```

Parses the request body as JSON into the given value.

```go
var user User
if err := c.Body(&user); err != nil {
    return kruda.NewError(400, "Invalid JSON")
}
```

### BodyBytes

```go
func (c *Ctx) BodyBytes() ([]byte, error)
```

Returns the raw request body as bytes.

## Response Methods

All response methods return `*Ctx` for method chaining unless otherwise noted.

### JSON

```go
func (c *Ctx) JSON(code int, v interface{}) error
```

Sends a JSON response with the given status code.

```go
return c.JSON(200, map[string]string{"message": "ok"})
return c.JSON(201, user)
```

### String

```go
func (c *Ctx) String(code int, s string) error
```

Sends a plain text response.

```go
return c.String(200, "Hello, World!")
```

### Status

```go
func (c *Ctx) Status(code int) *Ctx
```

Sets the response status code. Chainable.

```go
return c.Status(204).JSON(204, nil)
```

### SetHeader

```go
func (c *Ctx) SetHeader(key, value string) *Ctx
```

Sets a response header. CRLF characters in values are automatically stripped. Invalid header keys are silently skipped.

```go
c.SetHeader("X-Custom", "value")
```

### AddHeader

```go
func (c *Ctx) AddHeader(key, value string) *Ctx
```

Adds a response header (allows multiple values for the same key).

### SetCookie

```go
func (c *Ctx) SetCookie(cookie *http.Cookie) *Ctx
```

Sets a response cookie. Cookie values are sanitized.

### Redirect

```go
func (c *Ctx) Redirect(code int, url string) error
```

Sends a redirect response.

```go
return c.Redirect(302, "/login")
```

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

## Method Chaining

Response methods return `*Ctx` for fluent chaining:

```go
return c.Status(200).
    SetHeader("X-Request-ID", reqID).
    SetHeader("Cache-Control", "no-cache").
    JSON(200, data)
```
