# Security

Kruda provides security hardening features that can be enabled with a single option. Use `WithSecureHeaders()` to enable all default security headers.

For vulnerability reporting, see [SECURITY.md](https://github.com/go-kruda/kruda/blob/main/SECURITY.md).

## Security Headers

When enabled via `WithSecureHeaders()`, every response includes these headers:

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
- The framework provides layered protections. Core protections (path normalization, header injection prevention, body size limits) are always active. Security response headers are opt-in via `WithSecureHeaders()`.

| Threat | Severity | Status |
|--------|----------|--------|
| Path Traversal | High | Mitigated |
| Header Injection (CRLF) | High | Mitigated |
| Denial of Service (DoS) | Medium | Mitigated |
| CORS Bypass | Medium | Mitigated |
| Cross-Site Scripting (XSS) | Medium | Mitigated (headers) |
| Cross-Site Request Forgery (CSRF) | Medium | Mitigated (middleware) |

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

**Source:** `config.go`, `transport/nethttp.go`, `transport/wing/transport.go`

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

**Source:** `middleware/csrf.go`

Built-in CSRF middleware using the double-submit cookie pattern.

```go
app.Use(middleware.CSRF())
```

**How it works:**

1. **Safe methods** (GET, HEAD, OPTIONS, TRACE): Generates a random 32-byte token (crypto/rand), sets a `_csrf` cookie, and stores the token in `c.Set("csrf_token", token)` for template rendering.
2. **Unsafe methods** (POST, PUT, DELETE, PATCH): Validates the `X-CSRF-Token` header against the `_csrf` cookie using `crypto/subtle.ConstantTimeCompare`. Rejects with 403 on mismatch.
3. **After validation**, a new token is generated for the next request (one-time use).

**SPA/AJAX pattern:** Read the `_csrf` cookie with JavaScript and send it as the `X-CSRF-Token` header on every mutation request.

```javascript
// Frontend: read cookie and send as header
const csrfToken = document.cookie.match(/(?:^|;\s*)_csrf=([^;]*)/)?.[1];
fetch('/api/data', {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrfToken },
    body: JSON.stringify(data),
});
```

**Server-rendered form pattern:** Access the token from the request context for hidden fields.

```go
app.Get("/form", func(c *kruda.Ctx) error {
    token := c.Get("csrf_token").(string)
    // Render form with hidden <input name="_csrf" value="token">
    return c.HTML(renderForm(token))
})
```

**Configuration:**

```go
app.Use(middleware.CSRF(middleware.CSRFConfig{
    CookieName:   "_csrf",         // cookie name (default)
    HeaderName:   "X-CSRF-Token",  // header to check (default)
    CookiePath:   "/",             // cookie path (default)
    CookieSecure: true,            // set for HTTPS-only
    SameSite:     http.SameSiteStrictMode, // default
    MaxAge:       3600,            // 1 hour (default)
    TokenLength:  32,              // 32 bytes = 64 hex chars (default)
    Skip: func(c *kruda.Ctx) bool {
        return strings.HasPrefix(c.Path(), "/api/webhook")
    },
}))
```

Key security properties:

- **Constant-time comparison** prevents timing attacks
- **SameSite=Strict** prevents cross-site cookie sending
- **HttpOnly=false** on CSRF cookie (JS must read it for double-submit pattern)
- **Token refresh** on every request prevents replay
- **Minimum 16-byte tokens** enforced at init (panics if shorter)

> Applications using Bearer token authentication (`Authorization` header) are generally not vulnerable to CSRF.

## Session Management

**Source:** `contrib/session/`

Session middleware with a pluggable store interface. Default in-memory store included.

```go
import "github.com/go-kruda/kruda/contrib/session"

app.Use(session.New())
```

**Usage in handlers:**

```go
app.Post("/login", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    sess.Set("user_id", 42)
    sess.Set("role", "admin")
    return c.JSON(kruda.Map{"ok": true})
})

app.Get("/profile", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    userID := sess.GetInt("user_id")
    role := sess.GetString("role")
    return c.JSON(kruda.Map{"user_id": userID, "role": role})
})

app.Post("/logout", func(c *kruda.Ctx) error {
    sess := session.GetSession(c)
    sess.Destroy() // removes from store + expires cookie
    return c.JSON(kruda.Map{"ok": true})
})
```

**Session API:**

| Method | Description |
|--------|-------------|
| `Get(key)` | Get any value |
| `GetString(key, default...)` | Get string with optional default |
| `GetInt(key, default...)` | Get int with optional default |
| `Set(key, value)` | Store a value |
| `Delete(key)` | Remove a key |
| `Clear()` | Remove all values |
| `Destroy()` | Delete session from store + expire cookie |
| `ID()` | Get session ID |
| `IsNew()` | True if session was just created |

**Configuration:**

```go
app.Use(session.New(session.Config{
    CookieName:     "_session",              // default
    CookiePath:     "/",                     // default
    CookieSecure:   true,                    // for HTTPS
    CookieHTTPOnly: true,                    // default (prevents JS access)
    CookieSameSite: http.SameSiteLaxMode,    // default
    MaxAge:         86400,                   // 24h cookie (default)
    IdleTimeout:    30 * time.Minute,        // server-side expiry (default)
    Store:          session.NewMemoryStore(), // default
    Skip: func(method, path string) bool {
        return strings.HasPrefix(path, "/static/")
    },
}))
```

**Custom store:** Implement the `Store` interface for Redis, database, or other backends:

```go
type Store interface {
    Get(id string) (*SessionData, error)
    Save(id string, data *SessionData, ttl time.Duration) error
    Delete(id string) error
}
```

Session security properties:

- **32-byte session IDs** from crypto/rand (64 hex chars)
- **HttpOnly=true** by default (prevents XSS session theft)
- **SameSite=Lax** by default
- **Server-side expiration** via IdleTimeout (independent of cookie MaxAge)
- **Automatic cleanup** of expired sessions in MemoryStore

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
- [ ] Enable `middleware.CSRF()` for form-based or cookie-authenticated routes
- [ ] Configure CORS with explicit origins (never `*` with credentials)
- [ ] Use `session.New()` with `CookieSecure: true` for HTTPS
- [ ] Set `MessageTimeout` and `MaxPingPerSecond` on WebSocket endpoints
- [ ] Validate file upload extensions in your handler
- [ ] Use strong JWT secrets from environment variables or a vault
- [ ] Add `middleware.PathTraversal()` before serving static files
- [ ] Keep route regex constraints simple to avoid ReDoS

```go
app := kruda.New()
app.Use(middleware.Recovery(middleware.RecoveryConfig{DisableStackTrace: true}))
app.Use(middleware.RequestID())
app.Use(middleware.CSRF())
app.Use(session.New(session.Config{CookieSecure: true}))
app.Use(middleware.PathTraversal())
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```
