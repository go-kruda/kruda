# Error Handling

Kruda provides structured error handling with error mapping, custom error types, and dev mode error pages.

## KrudaError

The built-in error type for HTTP errors:

```go
err := kruda.NewError(404, "User not found")
err := kruda.NewErrorWithDetails(422, "Validation failed", details)
```

Return errors from handlers:

```go
app.Get("/users/:id", func(c *kruda.Ctx) error {
    user, err := findUser(c.Param("id"))
    if err != nil {
        return kruda.NewError(404, "User not found")
    }
    return c.JSON(200, user)
})
```

Error response format:

```json
{
  "error": "User not found",
  "code": 404
}
```

## Error Mapping

Map domain errors to HTTP responses without cluttering handler logic:

### MapError — Map Specific Errors

```go
var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

app.MapError(ErrNotFound, 404, "Resource not found")
app.MapError(ErrForbidden, 403, "Access denied")
```

Now handlers can return domain errors directly:

```go
app.Get("/users/:id", func(c *kruda.Ctx) error {
    user, err := repo.FindByID(c.Param("id"))
    if err != nil {
        return err // ErrNotFound → 404 automatically
    }
    return c.JSON(200, user)
})
```

### MapErrorType — Map Error Types

```go
app.MapErrorType(reflect.TypeOf(&ValidationError{}), 422, "Validation failed")
```

### MapErrorFunc — Custom Error Mapping

```go
app.MapErrorFunc(func(err error) *kruda.KrudaError {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        if pgErr.Code == "23505" { // unique violation
            return kruda.NewError(409, "Resource already exists")
        }
    }
    return nil // not handled, pass to next mapper
})
```

## Custom Error Handler

Override the default error handler:

```go
app := kruda.New(kruda.WithErrorHandler(func(c *kruda.Ctx, err error) {
    code := 500
    message := "Internal Server Error"

    var kErr *kruda.KrudaError
    if errors.As(err, &kErr) {
        code = kErr.Code
        message = kErr.Message
    }

    c.JSON(code, map[string]interface{}{
        "error":     message,
        "requestId": c.Header("X-Request-ID"),
    })
}))
```

## Dev Mode Error Page

In development mode, errors render as a rich HTML page with source code context, stack trace, and request details:

```go
app := kruda.New(kruda.WithDevMode(true))
```

The dev error page includes:
- Error message and type
- Source code ±10 lines around the error
- Stack trace with file paths
- Request details (method, path, headers, query)
- Available routes table
- Filtered environment variables

Dev mode is never active in production. Enable it explicitly or via `KRUDA_ENV=development`.

See [Error API](/api/error) for the full reference.
