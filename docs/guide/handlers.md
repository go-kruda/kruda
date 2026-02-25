# Handlers

## Basic Handlers

A handler receives a `*Ctx` and returns an error:

```go
app.Get("/hello", func(c *kruda.Ctx) error {
    return c.String(200, "Hello, World!")
})
```

The `HandlerFunc` type:

```go
type HandlerFunc func(c *Ctx) error
```

## Typed Handlers with C[T]

Typed handlers use Go generics to auto-parse request data into a struct:

```go
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}

app.Post("/users", kruda.TypedHandler[CreateUserRequest](func(c *kruda.C[CreateUserRequest]) error {
    input := c.Input // auto-parsed and validated
    // input.Name, input.Email, input.Age are ready to use
    return c.JSON(201, map[string]string{"name": input.Name})
}))
```

The `C[T]` type extends `Ctx` with a typed `Input` field:

```go
type C[T any] struct {
    *Ctx
    Input T
}
```

## Struct Tags

Control how request data is bound to your struct:

```go
type GetUserRequest struct {
    ID     string `param:"id"`              // from route parameter :id
    Fields string `query:"fields"`          // from query string ?fields=name,email
}

type UpdateUserRequest struct {
    ID   string `param:"id"`               // from route parameter
    Name string `json:"name"`              // from JSON body
    Age  int    `json:"age"`               // from JSON body
}
```

Supported tag types:

| Tag | Source | Example |
|-----|--------|---------|
| `param` | Route parameters (`:id`) | `param:"id"` |
| `query` | Query string (`?key=val`) | `query:"fields"` |
| `json` | JSON request body | `json:"name"` |
| `header` | Request headers | `header:"X-Request-ID"` |

## Validation

Validation runs automatically after binding. Use `validate` struct tags:

```go
type Input struct {
    Name  string `json:"name"  validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}
```

When validation fails, Kruda returns a structured error response:

```json
{
  "error": "Validation failed",
  "code": 422,
  "details": [
    { "field": "email", "message": "must be a valid email" }
  ]
}
```

## Response Methods

Handlers use `Ctx` methods to send responses:

```go
// JSON response
c.JSON(200, user)

// String response
c.String(200, "OK")

// Status only
c.Status(204)

// With headers (method chaining)
c.Status(200).SetHeader("X-Custom", "value").JSON(200, data)

// Redirect
c.Redirect(302, "/new-location")
```

See [Context API](/api/context) for all response methods.
