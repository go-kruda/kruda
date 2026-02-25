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
    return c.String(200, "Hello!")
})

tc := kruda.NewTestClient(app)
```

## TestRequest Builder

Build requests with a fluent API:

### Get / Post / Put / Delete / Patch

```go
resp := tc.Get("/hello")
resp := tc.Post("/users")
resp := tc.Put("/users/123")
resp := tc.Delete("/users/123")
resp := tc.Patch("/users/123")
```

### WithHeader

```go
resp := tc.Get("/api/profile").
    WithHeader("Authorization", "Bearer token123")
```

### WithJSON

```go
resp := tc.Post("/users").
    WithJSON(map[string]string{"name": "Alice", "email": "alice@example.com"})
```

### WithBody

```go
resp := tc.Post("/upload").
    WithBody([]byte("raw body content"))
```

### WithQuery

```go
resp := tc.Get("/search").
    WithQuery("q", "kruda").
    WithQuery("page", "1")
```

## TestResponse

### StatusCode

```go
resp.StatusCode // int — HTTP status code
```

### Body

```go
body := resp.Body() // string — response body as string
```

### JSON

```go
var result map[string]interface{}
resp.JSON(&result) // parse response body as JSON
```

### Header

```go
contentType := resp.Header("Content-Type")
requestID := resp.Header("X-Request-ID")
```

## Example Test

```go
func TestCreateUser(t *testing.T) {
    app := kruda.New()
    app.Post("/users", func(c *kruda.Ctx) error {
        var user struct {
            Name string `json:"name"`
        }
        if err := c.Body(&user); err != nil {
            return kruda.NewError(400, "Invalid JSON")
        }
        return c.JSON(201, map[string]string{"name": user.Name})
    })

    tc := kruda.NewTestClient(app)

    resp := tc.Post("/users").
        WithJSON(map[string]string{"name": "Alice"})

    if resp.StatusCode != 201 {
        t.Fatalf("expected 201, got %d", resp.StatusCode)
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
        return c.String(200, "secret")
    })

    tc := kruda.NewTestClient(app)

    // Without auth — should get 401
    resp := tc.Get("/protected")
    if resp.StatusCode != 401 {
        t.Fatalf("expected 401, got %d", resp.StatusCode)
    }

    // With auth — should get 200
    resp = tc.Get("/protected").
        WithHeader("Authorization", "Bearer valid-token")
    if resp.StatusCode != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}
```
