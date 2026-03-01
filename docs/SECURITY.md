# Security Policy

> **Version:** 1.0.0-rc1
> **Last Updated:** 2025
> **Contact:** security@go-kruda.dev
> **Module:** `github.com/go-kruda/kruda`

Kruda takes security seriously. This document describes the framework's threat model, built-in mitigations, secure defaults, and responsible disclosure process.

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.0.x   | ✅ Active |
| < 1.0   | ❌ No     |

Security patches are applied to the latest minor release only.

---

## Threat Model

Kruda is a server-side HTTP framework. The threat model assumes:

- The application is exposed to untrusted network traffic (the internet).
- Request paths, headers, query parameters, and body content are attacker-controlled.
- The framework must protect against common web vulnerabilities by default, without requiring developer configuration.

The following attack vectors are addressed:

| Threat | Severity | Status |
|--------|----------|--------|
| Path Traversal | High | Mitigated |
| Header Injection (CRLF) | High | Mitigated |
| Denial of Service (DoS) | Medium | Mitigated |
| CORS Bypass | Medium | Mitigated |
| Cross-Site Scripting (XSS) | Medium | Mitigated (headers) |
| Cross-Site Request Forgery (CSRF) | Medium | Guidance provided |

---

## Mitigations

### 1. Path Traversal Prevention

**Source:** `router.go` — `cleanPath()` function

All request paths are normalized before route matching. The `cleanPath` function:

1. Decodes percent-encoded sequences (e.g. `%2e%2e%2f` → `../`)
2. Normalizes the path via `path.Clean()` to resolve `.` and `..` segments
3. Ensures the result starts with `/`
4. Rejects any path that still contains `..` after cleaning

Traversal attempts return HTTP 400 (Bad Request) before reaching any handler.

```
GET /../etc/passwd        → 400 Bad Request
GET /%2e%2e/etc/passwd    → 400 Bad Request
GET /a/b/../c             → normalized to /a/c, routed normally
```

This protection is always enabled with zero configuration required.

### 2. Header Injection Prevention

**Source:** `context.go` — `sanitizeHeaderValue()`, `isValidHeaderKey()`

HTTP header injection (CRLF injection) is prevented on all response header methods:

- `sanitizeHeaderValue(value)` strips `\r` and `\n` characters from header values. A fast path skips processing when no CRLF characters are present.
- `isValidHeaderKey(key)` validates that header keys contain only token characters per RFC 7230. Invalid keys are silently skipped with a warning log.

These checks apply to `SetHeader`, `AddHeader`, and `SetCookie`.

```go
// CRLF in value is stripped automatically
c.SetHeader("X-Custom", "value\r\nInjected: header")
// Result: X-Custom: valueInjected: header

// Invalid key is silently skipped
c.SetHeader("Invalid Key!", "value")
// Result: warning logged, header not set
```

### 3. Denial of Service (DoS) Protection

**Source:** `config.go` — default configuration, `transport/nethttp.go` — body limit enforcement

Kruda enforces resource limits by default to prevent resource exhaustion:

| Setting | Default | Option |
|---------|---------|--------|
| Max Body Size | 4 MB | `WithBodyLimit(n)` / `WithMaxBodySize(n)` |
| Read Timeout | 30 seconds | `WithReadTimeout(d)` |
| Write Timeout | 30 seconds | `WithWriteTimeout(d)` |
| Idle Timeout | 120 seconds | `WithIdleTimeout(d)` |
| Max Header Size | 8 KB | (config field) |

When a request body exceeds the configured limit, the framework responds with HTTP 413 (Request Entity Too Large). The transport layer enforces this via `io.LimitReader` wrapping.

Timeouts are enforced at the transport level (`net/http` server or fasthttp) and apply to all connections.

```go
// Custom limits
app := kruda.New(
    kruda.WithMaxBodySize(1024 * 1024), // 1MB
    kruda.WithReadTimeout(10 * time.Second),
)
```

### 4. CORS Bypass Prevention

**Source:** `middleware/cors.go` — `CORS()` middleware

The CORS middleware prevents origin bypass attacks through:

- **Exact origin matching:** Origins are validated against an allow-list using exact string comparison via an O(1) lookup set. No wildcard subdomain matching unless explicitly configured.
- **Credentials + wildcard rejection:** Configuring `AllowCredentials: true` with `AllowOrigins: ["*"]` causes a panic at initialization time, preventing a common CORS misconfiguration that violates the CORS specification.
- **Non-matching origin handling:** When the `Origin` header does not match any allowed origin, the `Access-Control-Allow-Origin` header is omitted entirely (the origin is never echoed back).
- **Preflight handling:** `OPTIONS` requests with `Access-Control-Request-Method` are handled as preflight requests, returning HTTP 204 with appropriate `Access-Control-Allow-*` headers.
- **Vary header:** `Vary: Origin` is set for non-wildcard configurations to ensure correct caching behavior.

### 5. XSS Mitigation (Security Headers)

**Source:** `config.go` — `SecurityConfig`, `context.go` — `writeHeaders()`

Kruda sets security headers on all responses by default to mitigate XSS and other client-side attacks. See the [Secure Defaults](#secure-defaults) section below for the full list.

The `X-XSS-Protection` header is set to `0` (disabled) following modern best practice — the legacy XSS auditor in older browsers could itself be exploited. Content Security Policy (CSP) is the recommended XSS mitigation and should be configured by the application developer based on their specific needs.

### 6. CSRF Guidance

CSRF protection is not included in the Kruda core framework. It will be provided as a separate `contrib/csrf` package in Phase 6.

**Recommended pattern: Double-Submit Cookie**

Until the `contrib/csrf` package is available, applications that use cookie-based authentication should implement CSRF protection using the double-submit cookie pattern:

1. Generate a random CSRF token and set it as a cookie (`SameSite=Strict` or `SameSite=Lax`).
2. Include the token in a hidden form field or custom request header (e.g. `X-CSRF-Token`).
3. On the server, compare the cookie value with the header/form value. Reject the request if they don't match.

```go
// Example: custom CSRF middleware (simplified)
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

**Note:** Applications using token-based authentication (e.g. Bearer tokens in the `Authorization` header) are generally not vulnerable to CSRF, as browsers do not automatically attach these tokens to cross-origin requests.

---

## Secure Defaults

All security configurations are enabled by default. No configuration is required for a secure baseline.

### Security Headers

| Header | Default Value | Purpose |
|--------|---------------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing — browsers must respect the declared `Content-Type` |
| `X-Frame-Options` | `DENY` | Prevents clickjacking by disallowing the page from being embedded in frames |
| `X-XSS-Protection` | `0` | Disables the legacy browser XSS auditor (can be exploited; CSP is preferred) |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits referrer information sent to other origins |

### DevMode Relaxation

When `Config.DevMode` is `true`, `X-Frame-Options` is relaxed to `SAMEORIGIN` to allow development tools that use iframes.

DevMode is never enabled by default — it must be explicitly opted into via `WithDevMode(true)` or the `KRUDA_ENV=development` environment variable.

### No Server Header

Kruda does not set a `Server` response header by default. Server version information is never exposed to clients unless explicitly configured by the application developer.

### Configuration Options

```go
// Enable security headers (recommended)
app := kruda.New(kruda.WithSecureHeaders())

// Restore Phase 1-4 defaults for backward compatibility
app := kruda.New(kruda.WithLegacySecurityHeaders())
// X-Frame-Options: SAMEORIGIN
// X-XSS-Protection: 1; mode=block
// Referrer-Policy: no-referrer

// Enable dev mode (relaxes X-Frame-Options to SAMEORIGIN)
app := kruda.New(kruda.WithDevMode(true))
```

---

## CORS Best Practices

1. **Never use `AllowOrigins: ["*"]` with `AllowCredentials: true`.** Kruda panics at startup if this misconfiguration is detected.

2. **Specify exact origins** rather than wildcards when your application uses cookies or credentials:
   ```go
   middleware.CORS(middleware.CORSConfig{
       AllowOrigins:     []string{"https://app.example.com"},
       AllowCredentials: true,
   })
   ```

3. **Limit allowed methods and headers** to only what your API requires:
   ```go
   middleware.CORS(middleware.CORSConfig{
       AllowOrigins: []string{"https://app.example.com"},
       AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
       AllowHeaders: []string{"Content-Type", "Authorization"},
   })
   ```

4. **Set a reasonable `MaxAge`** for preflight caching (default: 86400 seconds / 24 hours).

5. **Use `Vary: Origin`** — Kruda sets this automatically for non-wildcard configurations to ensure proxies and CDNs cache CORS responses correctly.

---

## Responsible Disclosure

If you discover a security vulnerability in Kruda, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

### How to Report

1. Email **security@go-kruda.dev** with:
   - Description of the vulnerability
   - Steps to reproduce
   - Affected version(s)
   - Impact assessment (if known)

2. You will receive an acknowledgment within **48 hours**.

3. We aim to provide a fix or mitigation within **7 days** for critical issues and **30 days** for non-critical issues.

### What to Expect

| Timeline | Action |
|----------|--------|
| 0-48 hours | Acknowledgment of report |
| 1-7 days | Initial assessment and severity classification |
| 7-30 days | Fix developed, tested, and released |
| Post-fix | Credit in CHANGELOG and security advisory (if desired) |

### Scope

The following are in scope for security reports:

- Kruda core framework (`github.com/go-kruda/kruda`)
- Transport implementations (`transport/`)
- Built-in middleware (`middleware/`)
- CLI tool (`cmd/kruda/`)

The following are out of scope:

- Vulnerabilities in user application code
- Vulnerabilities in third-party dependencies (report to the dependency maintainer)
- Denial of service via legitimate high traffic (this is an infrastructure concern)

### Recognition

We appreciate responsible disclosure and will credit reporters in our security advisories and CHANGELOG unless anonymity is requested.
