# Handler

## HandlerFunc

```go
type HandlerFunc func(c *Ctx) error
```

The base handler type. Every route handler and middleware is a `HandlerFunc`.

```go
app.Get("/hello", func(c *kruda.Ctx) error {
    return c.String(200, "Hello!")
})
```

## MiddlewareFunc

```go
type MiddlewareFunc = HandlerFunc
```

`MiddlewareFunc` is a type alias for `HandlerFunc`. They are fully interchangeable.

```go
func MyMiddleware(c *kruda.Ctx) error {
    // before handler
    err := c.Next()
    // after handler
    return err
}
```

## C[T] — Typed Context

```go
type C[T any] struct {
    *Ctx
    Input T
}
```

`C[T]` extends `Ctx` with a typed `Input` field. The request body, route parameters, and query parameters are automatically parsed and validated into `T` before the handler runs.

## TypedHandler

```go
func TypedHandler[T any](handler func(c *C[T]) error) HandlerFunc
```

Wraps a typed handler function into a standard `HandlerFunc`. The type parameter `T` defines the request struct.

```go
type CreateUserInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

app.Post("/users", kruda.TypedHandler[CreateUserInput](func(c *kruda.C[CreateUserInput]) error {
    input := c.Input
    // input.Name and input.Email are parsed and validated
    return c.JSON(201, map[string]string{"name": input.Name})
}))
```

## Struct Tag Binding

The typed handler binds request data based on struct tags:

```go
type GetUserInput struct {
    ID     string `param:"id"`         // route parameter :id
    Fields string `query:"fields"`     // query string ?fields=...
}

type UpdateUserInput struct {
    ID   string `param:"id"`           // route parameter
    Name string `json:"name"`          // JSON body field
    Age  int    `json:"age"`           // JSON body field
}
```

Binding order:
1. `param` tags — from route parameters
2. `query` tags — from query string
3. `json` tags — from request body
4. `header` tags — from request headers

## Validation

After binding, validation runs automatically using `validate` struct tags:

```go
type Input struct {
    Name string `json:"name" validate:"required,min=2,max=100"`
    Age  int    `json:"age"  validate:"min=0,max=150"`
}
```

Validation failures return HTTP 422 with structured error details.
