# Test Client

In-memory HTTP client for testing Kruda applications without starting a real server.

## NewTestClient

```go
func NewTestClient(app *App) *TestClient
```

Creates a test client bound to the given app. Requests are handled in-memory — no network, no port conflicts.

```go
app := kruda.New()
app.Get("/hello", func(c *kruda.Ctx) error {
    return c.Text("Hello!")
})

tc := kruda.NewTestClient(app)
```

## Quick Methods

Convenience methods that return `(*TestResponse, error)`:

```go
resp, err := tc.Get("/hello")
resp, err := tc.Post("/users", body)
resp, err := tc.Put("/users/123", body)
resp, err := tc.Delete("/users/123")
resp, err := tc.Patch("/users/123", body)
resp, err := tc.Head("/hello")
resp, err := tc.Options("/hello")
```

### Client-Level Headers/Cookies

Set default headers or cookies for all requests:

```go
tc := kruda.NewTestClient(app)
tc.WithHeader("Authorization", "Bearer token123")
tc.WithCookie("session", "abc")
```

## Request Builder

For more control, use the fluent `TestRequest` builder:

```go
resp, err := tc.Request("POST", "/users").
    Header("Authorization", "Bearer token").
    Body(map[string]string{"name": "Alice"}).
    Query("notify", "true").
    ContentType("application/json").
    Cookie("session", "abc").
    Send()
```

### Builder Methods

| Method | Description |
|--------|-------------|
| `Header(key, value)` | Set a request header |
| `Cookie(name, value)` | Set a request cookie |
| `Body(v any)` | Set request body (JSON-encoded if struct/map) |
| `Query(key, value)` | Add a query parameter |
| `ContentType(ct)` | Set Content-Type header |
| `Send()` | Execute the request, returns `(*TestResponse, error)` |

## TestResponse

### StatusCode

```go
func (tr *TestResponse) StatusCode() int
```

### Header

```go
func (tr *TestResponse) Header(key string) string
```

### Body / BodyString

```go
func (tr *TestResponse) Body() []byte
func (tr *TestResponse) BodyString() string
```

### JSON

```go
func (tr *TestResponse) JSON(v any) error
```

Parse response body as JSON.

## Example Test

```go
func TestCreateUser(t *testing.T) {
    app := kruda.New()
    app.Post("/users", func(c *kruda.Ctx) error {
        var user struct {
            Name string `json:"name"`
        }
        if err := c.Bind(&user); err != nil {
            return kruda.NewError(400, "Invalid JSON")
        }
        return c.Status(201).JSON(map[string]string{"name": user.Name})
    })

    tc := kruda.NewTestClient(app)

    resp, err := tc.Post("/users", map[string]string{"name": "Alice"})
    if err != nil {
        t.Fatal(err)
    }

    if resp.StatusCode() != 201 {
        t.Fatalf("expected 201, got %d", resp.StatusCode())
    }

    var result map[string]string
    resp.JSON(&result)

    if result["name"] != "Alice" {
        t.Fatalf("expected Alice, got %s", result["name"])
    }
}
```

## Testing Middleware

```go
func TestAuthMiddleware(t *testing.T) {
    app := kruda.New()
    app.Use(AuthMiddleware)
    app.Get("/protected", func(c *kruda.Ctx) error {
        return c.Text("secret")
    })

    tc := kruda.NewTestClient(app)

    // Without auth — should get 401
    resp, _ := tc.Get("/protected")
    if resp.StatusCode() != 401 {
        t.Fatalf("expected 401, got %d", resp.StatusCode())
    }

    // With auth — use Request builder
    resp, _ = tc.Request("GET", "/protected").
        Header("Authorization", "Bearer valid-token").
        Send()
    if resp.StatusCode() != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode())
    }
}
```
