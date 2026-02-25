# Error

Structured error handling with error mapping and custom error types.

## KrudaError

```go
type KrudaError struct {
    Code    int         `json:"code"`
    Message string      `json:"error"`
    Details interface{} `json:"details,omitempty"`
}
```

### NewError

```go
func NewError(code int, message string) *KrudaError
```

Creates a new HTTP error.

```go
return kruda.NewError(404, "User not found")
return kruda.NewError(400, "Invalid request")
```

### NewErrorWithDetails

```go
func NewErrorWithDetails(code int, message string, details interface{}) *KrudaError
```

Creates an error with additional details.

```go
return kruda.NewErrorWithDetails(422, "Validation failed", []map[string]string{
    {"field": "email", "message": "must be a valid email"},
})
```

### Error Response Format

```json
{
  "code": 404,
  "error": "User not found"
}
```

With details:

```json
{
  "code": 422,
  "error": "Validation failed",
  "details": [
    { "field": "email", "message": "must be a valid email" }
  ]
}
```

## Error Mapping

### MapError

```go
func (a *App) MapError(err error, code int, message string) *App
```

Maps a specific error value to an HTTP response.

```go
var ErrNotFound = errors.New("not found")
app.MapError(ErrNotFound, 404, "Resource not found")
```

### MapErrorType

```go
func (a *App) MapErrorType(t reflect.Type, code int, message string) *App
```

Maps all errors of a given type.

```go
app.MapErrorType(reflect.TypeOf(&ValidationError{}), 422, "Validation failed")
```

### MapErrorFunc

```go
func (a *App) MapErrorFunc(fn func(error) *KrudaError) *App
```

Registers a custom mapping function. Return `nil` to pass to the next mapper.

```go
app.MapErrorFunc(func(err error) *kruda.KrudaError {
    if errors.Is(err, sql.ErrNoRows) {
        return kruda.NewError(404, "Not found")
    }
    return nil
})
```

## Error Flow

When a handler returns an error:

1. Check if it's already a `*KrudaError` → use directly
2. Check `MapError` exact matches → use mapped response
3. Check `MapErrorType` type matches → use mapped response
4. Check `MapErrorFunc` functions in order → use first non-nil result
5. Fall back to HTTP 500 Internal Server Error

In dev mode (`WithDevMode(true)`), unhandled errors render a rich HTML error page instead of JSON.
