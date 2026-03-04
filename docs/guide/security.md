# Security

Kruda is secure by default. All security features are enabled out of the box with no configuration required.

For vulnerability reporting, see [SECURITY.md](https://github.com/go-kruda/kruda/blob/main/SECURITY.md).

## Secure Default Headers

Every response includes these security headers automatically:

| Header | Default Value | Purpose |
|--------|--------------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing |
| `X-Frame-Options` | `DENY` | Prevents clickjacking |
| `X-XSS-Protection` | `0` | Disabled per modern best practice (CSP preferred) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Controls referrer information |

No `Server` header is sent by default -- no version information is leaked.

```go
// Explicitly enable security headers
app := kruda.New(kruda.WithSecureHeaders())

// Restore Phase 1-4 defaults for backward compatibility
app := kruda.New(kruda.WithLegacySecurityHeaders())
// X-Frame-Options: SAMEORIGIN, X-XSS-Protection: 1; mode=block, Referrer-Policy: no-referrer
```

## Threat Model

Kruda assumes:

- The application is exposed to untrusted network traffic (the internet).
- Request paths, headers, query parameters, and body content are attacker-controlled.
- The framework must protect against common web vulnerabilities by default.

| Threat | Severity | Status |
|--------|----------|--------|
| Path Traversal | High | Mitigated |
| Header Injection (CRLF) | High | Mitigated |
| Denial of Service (DoS) | Medium | Mitigated |
| CORS Bypass | Medium | Mitigated |
| Cross-Site Scripting (XSS) | Medium | Mitigated (headers) |
| Cross-Site Request Forgery (CSRF) | Medium | Guidance provided |

## Path Traversal Prevention

**Source:** `router.go` -- `cleanPath()`

All request paths are normalized before route matching:

1. Decodes percent-encoded sequences (`%2e%2e%2f` -> `../`)
2. Normalizes via `path.Clean()` to resolve `.` and `..` segments
3. Ensures the result starts with `/`
4. Rejects any path that still contains `..` after cleaning

```
GET /../etc/passwd        -> 400 Bad Request
GET /%2e%2e/etc/passwd    -> 400 Bad Request
GET /a/b/../c             -> normalized to /a/c, routed normally
```

Always enabled, zero configuration required.

## Header Injection Prevention

**Source:** `context.go` -- `sanitizeHeaderValue()`, `isValidHeaderKey()`

HTTP header injection (CRLF injection) is prevented on all response header methods:

- `sanitizeHeaderValue()` strips `\r` and `\n` from header values (fast path when no CRLF present)
- `isValidHeaderKey()` validates keys contain only token characters per RFC 7230

```go
// CRLF characters are automatically stripped
c.SetHeader("X-Custom", "value\r\nInjected: header")
// Result: "X-Custom: valueInjected: header"

// Invalid header keys are silently skipped with a warning log
c.SetHeader("Invalid Key!", "value")
```

Applies to `SetHeader`, `AddHeader`, and `SetCookie`.

## DoS Protection

**Source:** `config.go`, `transport/nethttp.go`

| Setting | Default | Option |
|---------|---------|--------|
| Max body size | 4 MB | `WithBodyLimit(bytes)` / `WithMaxBodySize(bytes)` |
| Read timeout | 30s | `WithReadTimeout(d)` |
| Write timeout | 30s | `WithWriteTimeout(d)` |
| Idle timeout | 120s | `WithIdleTimeout(d)` |
| Max header size | 8 KB | config field |

Bodies exceeding the limit return HTTP 413. Timeouts are enforced at the transport level.

```go
app := kruda.New(
    kruda.WithMaxBodySize(1024 * 1024), // 1MB
    kruda.WithReadTimeout(10 * time.Second),
)
```

## CORS Configuration

**Source:** `middleware/cors.go`

```go
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com", "https://admin.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    ExposeHeaders:    []string{"X-Request-ID"},
    MaxAge:           3600,
}))
```

Key behaviors:

- **Exact origin matching** via O(1) lookup set. Non-matching origins get no `Access-Control-Allow-Origin`.
- **Credentials + wildcard rejection**: `AllowCredentials: true` with `AllowOrigins: ["*"]` panics at startup (CORS spec violation).
- **Preflight handling**: `OPTIONS` requests return HTTP 204 with appropriate `Access-Control-Allow-*` headers.
- **Vary header**: `Vary: Origin` is set automatically for non-wildcard configurations.
- **Startup validation**: Invalid origin formats panic at init so misconfigurations are caught early.

## CSRF Protection

CSRF protection is available as a separate `contrib/csrf` package (not in core).

**Recommended pattern: Double-Submit Cookie**

1. Generate a random CSRF token and set it as a cookie (`SameSite=Strict` or `SameSite=Lax`).
2. Include the token in a hidden form field or custom header (`X-CSRF-Token`).
3. Compare cookie value with header/form value on the server. Reject on mismatch.

```go
func CSRFMiddleware() kruda.HandlerFunc {
    return func(c *kruda.Ctx) error {
        if c.Method() == "GET" || c.Method() == "HEAD" || c.Method() == "OPTIONS" {
            return c.Next()
        }
        cookieToken := c.Cookie("csrf_token")
        headerToken := c.Header("X-CSRF-Token")
        if cookieToken == "" || cookieToken != headerToken {
            return c.Status(403).JSON(kruda.Map{"error": "CSRF token mismatch"})
        }
        return c.Next()
    }
}
```

> Applications using Bearer token authentication (`Authorization` header) are generally not vulnerable to CSRF.

## JWT Best Practices

Kruda's JWT middleware (`contrib/jwt`) validates `exp` and `nbf` claims automatically. Tokens presented before their `nbf` time are rejected with `ErrTokenNotYetValid`.

- Store signing keys in environment variables or a secrets manager, never in source code.
- Use short-lived tokens (15-30 minutes) with a refresh token flow.
- Set `nbf` to the token's issue time to prevent pre-dating.

```go
app.Use(jwt.New(jwt.Config{
    Secret:     []byte(os.Getenv("JWT_SECRET")),
    Expiration: 30 * time.Minute,
}))
```

## WebSocket Security

### Message Timeout (Slowloris Prevention)

Without a timeout, a client can send fragmented frames indefinitely. Set `MessageTimeout` to cap assembly time.

```go
ws.New(ws.Config{
    MessageTimeout: 30 * time.Second,
    MaxMessageSize: 1 << 20, // 1 MB
})
```

### Ping Rate Limiting

A malicious client can flood the server with ping frames. `MaxPingPerSecond` closes the connection when exceeded.

```go
ws.New(ws.Config{
    MaxPingPerSecond: 10,
})
```

## File Upload Validation

Uploaded filenames are automatically sanitized with `filepath.Base()` to strip directory components. Validate extensions in your handler:

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

In development, stack traces help debugging. In production they leak internal paths and dependencies.

```go
// Development
app.Use(middleware.Recovery())

// Production -- hide stack traces
app.Use(middleware.Recovery(middleware.RecoveryConfig{
    DisableStackTrace: true,
}))

// Production -- report to external service
app.Use(middleware.Recovery(middleware.RecoveryConfig{
    DisableStackTrace: true,
    PanicHandler: func(c *kruda.Ctx, v any) {
        sentry.CaptureException(fmt.Errorf("panic: %v", v))
        c.Status(500).JSON(map[string]string{"error": "internal error"})
    },
}))
```

## Request ID & Route Regex

### Request ID Validation

Incoming `X-Request-ID` headers are validated against `[a-zA-Z0-9_-]` with a 256-character limit. Invalid characters cause the header to be replaced with a generated UUID v4.

Custom `Generator` functions should only produce allowed characters.

### Route Regex Constraints

Patterns with regex constraints (e.g. `/:id<^\d+$>`) are checked for nested quantifiers at registration time. Patterns like `(a+)+` that could cause catastrophic backtracking (ReDoS) are rejected with a panic.

Keep regex constraints simple:

```go
app.Get("/:id<^\\d+$>", handler)         // digits only
app.Get("/:slug<^[a-z0-9-]+$>", handler) // slugs
```

## Dev Mode

In development mode (`WithDevMode(true)` or `KRUDA_ENV=development`):

- `X-Frame-Options` is relaxed to `SAMEORIGIN` for dev tools
- Rich error pages are rendered with source context
- Environment variables are filtered (no `SECRET`, `PASSWORD`, `TOKEN`, `KEY`, `CREDENTIAL`, `AUTH`)

Dev mode defaults to `false` -- must be explicitly enabled.

## Production Checklist

- [ ] Use `middleware.Recovery` with `DisableStackTrace: true`
- [ ] Configure CORS with explicit origins (never `*` with credentials)
- [ ] Set `MessageTimeout` and `MaxPingPerSecond` on WebSocket endpoints
- [ ] Validate file upload extensions in your handler
- [ ] Use strong JWT secrets from environment variables or a vault
- [ ] Add `middleware.PathTraversal()` before serving static files
- [ ] Keep route regex constraints simple to avoid ReDoS

```go
app := kruda.New()
app.Use(middleware.Recovery(middleware.RecoveryConfig{DisableStackTrace: true}))
app.Use(middleware.RequestID())
app.Use(middleware.PathTraversal())
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```
