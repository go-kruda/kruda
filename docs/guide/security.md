# Security

Kruda is secure by default. All security features are enabled out of the box and can be configured as needed.

## Secure Default Headers

Every response includes these security headers:

| Header | Default Value | Purpose |
|--------|--------------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing |
| `X-Frame-Options` | `DENY` | Prevents clickjacking |
| `X-XSS-Protection` | `0` | Disabled per modern best practice (CSP preferred) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Controls referrer information |

Disable security headers if needed:

```go
app := kruda.New(kruda.WithSecurityHeaders(false))
```

Restore Phase 1-4 defaults for backward compatibility:

```go
app := kruda.New(kruda.WithLegacySecurityHeaders())
```

## Path Traversal Prevention

The router automatically normalizes paths and rejects traversal attempts:

- `/../etc/passwd` → HTTP 400
- `/%2e%2e/etc/passwd` → HTTP 400 (decoded and validated)
- `/a/b/../c` → normalized to `/a/c`

This is always enabled with no configuration required.

## Header Injection Prevention

Response header methods strip CRLF characters to prevent header injection:

```go
// CRLF characters are automatically stripped
c.SetHeader("X-Custom", "value\r\nInjected: header")
// Result: "X-Custom: valueInjected: header"

// Invalid header keys are silently skipped
c.SetHeader("Invalid Key!", "value") // logged as warning, skipped
```

This applies to `SetHeader`, `AddHeader`, and `SetCookie`.

## DoS Protection

Built-in defaults protect against resource exhaustion:

| Setting | Default | Option |
|---------|---------|--------|
| Max body size | 4 MB | `WithBodyLimit(bytes)` |
| Read timeout | 30s | `WithReadTimeout(d)` |
| Write timeout | 30s | `WithWriteTimeout(d)` |
| Idle timeout | 120s | `WithIdleTimeout(d)` |

Bodies exceeding the limit return HTTP 413. Read timeouts return HTTP 408.

## CORS Configuration

Use the CORS middleware with explicit allowed origins:

```go
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
}))
```

Key behaviors:
- Origins are matched with exact string comparison
- Non-matching origins receive no `Access-Control-Allow-Origin` header
- `AllowCredentials: true` with wildcard `*` origin panics at init time (per CORS spec)
- Preflight `OPTIONS` requests return HTTP 204

## No Server Header

Kruda does not expose a `Server` header by default. No version information is leaked in responses.

## Dev Mode

In development mode (`WithDevMode(true)` or `KRUDA_ENV=development`):
- `X-Frame-Options` is relaxed to `SAMEORIGIN` for dev tools
- Rich error pages are rendered with source context
- Environment variables are filtered (no `SECRET`, `PASSWORD`, `TOKEN`, `KEY`, `CREDENTIAL`, `AUTH`)

Dev mode defaults to `false` — it must be explicitly enabled.

## CSRF Protection

CSRF protection is available as a separate `contrib/csrf` package (not in core). Recommended pattern: double-submit cookie.

See [SECURITY.md](https://github.com/go-kruda/kruda/blob/main/docs/SECURITY.md) for the full security policy and threat model.
