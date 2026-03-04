# Security Hardening Guide

Production configuration recommendations for kruda.

## Quick Checklist

- [ ] Use `middleware.Recovery` with `DisableStackTrace: true`
- [ ] Configure CORS with explicit origins (never `*` with credentials)
- [ ] Set `MessageTimeout` and `MaxPingPerSecond` on WebSocket endpoints
- [ ] Validate file upload extensions in your handler
- [ ] Use strong JWT secrets from environment variables or a vault
- [ ] Add `middleware.PathTraversal()` before serving static files

## Recommended Production Config

```go
app := kruda.New()

// Panic recovery — hide stack traces in production
app.Use(middleware.Recovery(middleware.RecoveryConfig{
    DisableStackTrace: true,
}))

// Request tracing
app.Use(middleware.RequestID())

// Path traversal protection
app.Use(middleware.PathTraversal())

// CORS — explicit origins only
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

## WebSocket Security

### Message Timeout (Slowloris Prevention)

Without a message timeout, a client can send fragmented frames indefinitely and
hold a connection open. Set `MessageTimeout` to cap the total time allowed for
assembling a single message.

```go
ws.New(ws.Config{
    MessageTimeout: 30 * time.Second,
    MaxMessageSize: 1 << 20, // 1 MB
})
```

### Ping Rate Limiting

A malicious client can flood the server with ping frames, forcing it to
allocate pong responses. `MaxPingPerSecond` closes the connection when the
rate is exceeded.

```go
ws.New(ws.Config{
    MaxPingPerSecond: 10,
})
```

## JWT Best Practices

kruda's JWT middleware validates `exp` (expiration) and `nbf` (not-before)
claims automatically. Tokens presented before their `nbf` time are rejected
with `ErrTokenNotYetValid`.

- Store signing keys in environment variables or a secrets manager, never in
  source code.
- Use short-lived tokens (15–30 minutes) with a refresh token flow.
- Set `nbf` to the token's issue time to prevent tokens from being used
  before they were created.

```go
app.Use(jwt.New(jwt.Config{
    Secret:     []byte(os.Getenv("JWT_SECRET")),
    Expiration: 30 * time.Minute,
}))
```

## File Upload

Uploaded filenames are automatically sanitized with `filepath.Base()` to strip
directory components. You should still validate extensions in your handler:

```go
app.Post("/upload", func(c *kruda.Ctx) error {
    file := c.File("avatar")
    ext := strings.ToLower(filepath.Ext(file.Name))
    allowed := map[string]bool{".jpg": true, ".png": true, ".webp": true}
    if !allowed[ext] {
        return kruda.NewError(400, "unsupported file type")
    }
    return file.Save("./uploads/" + file.Name)
})
```

## Recovery Middleware

In development, stack traces help debugging. In production they leak internal
paths and dependencies.

```go
// Development
app.Use(middleware.Recovery())

// Production
app.Use(middleware.Recovery(middleware.RecoveryConfig{
    DisableStackTrace: true,
}))
```

A custom `PanicHandler` can send errors to an external service:

```go
app.Use(middleware.Recovery(middleware.RecoveryConfig{
    DisableStackTrace: true,
    PanicHandler: func(c *kruda.Ctx, v any) {
        sentry.CaptureException(fmt.Errorf("panic: %v", v))
        c.Status(500).JSON(map[string]string{"error": "internal error"})
    },
}))
```

## CORS Configuration

Origins are validated at startup — invalid formats cause a panic so
misconfigurations are caught early. Each origin must include a scheme and host
(e.g. `https://example.com`).

Using `AllowCredentials: true` with `AllowOrigins: ["*"]` is a CORS spec
violation and will panic.

```go
// SPA + API setup
middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com", "https://admin.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
    AllowCredentials: true,
    ExposeHeaders:    []string{"X-Request-ID"},
    MaxAge:           3600,
})
```

## Request ID

Incoming `X-Request-ID` headers are validated against `[a-zA-Z0-9_-]` with a
256-character length limit. Headers containing other characters are replaced
with a generated UUID v4.

If you use a custom `Generator`, ensure its output only contains allowed
characters — otherwise downstream request IDs will be regenerated on the next
hop.

## Route Regex Constraints

Route patterns with regex constraints (e.g. `/:id<^\d+$>`) are checked for
nested quantifiers at registration time. Patterns like `(a+)+` that could
cause catastrophic backtracking are rejected with a panic.

Keep regex constraints simple — character classes and basic quantifiers are
sufficient for most route validation:

```go
app.Get("/:id<^\\d+$>", handler)         // digits only
app.Get("/:slug<^[a-z0-9-]+$>", handler) // slugs
```
