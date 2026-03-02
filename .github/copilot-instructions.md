# Kruda Framework — GitHub Copilot Instructions

Kruda is a type-safe Go web framework (github.com/go-kruda/kruda). Go 1.25+, zero external deps in core.

## Handler Patterns

```go
// Standard handler
app.Get("/path", func(c *kruda.Ctx) error {
    return c.JSON(kruda.Map{"key": "value"})
})

// Typed handler with auto-parse and validation
kruda.Post[CreateUserInput, User](app, "/users", func(c *kruda.C[CreateUserInput]) (*User, error) {
    return &User{Name: c.In.Name}, nil
})

// Short handler (no error return)
kruda.GetX[GetInput, Output](app, "/items/:id", func(c *kruda.C[GetInput]) *Output {
    return &Output{ID: c.In.ID}
})
```

## Input Binding Tags

```go
type Input struct {
    ID    int    `param:"id"`
    Page  int    `query:"page"`
    Name  string `json:"name" validate:"required,min=2"`
}
```

## Key Conventions

- Transport: `kruda.New()` (default fasthttp) or `kruda.New(kruda.NetHTTP())` (HTTP/2, TLS)
- Functional options: `kruda.New(kruda.WithReadTimeout(30*time.Second))`
- Error returns: use `kruda.BadRequest("msg")`, `kruda.NotFound("msg")`, etc.
- Middleware: `type MiddlewareFunc = HandlerFunc`
- DI: `kruda.Give(container, value)` / `kruda.Use[T](container)`
- Testing: `tc := kruda.NewTestClient(app)` for in-memory tests
- Build: `go test -tags kruda_stdjson ./...`

## References

- Full spec: docs/kruda-spec.md
- API reference: docs/api/
- Guides: docs/guide/
