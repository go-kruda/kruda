# Config

Configuration is set via functional options passed to `kruda.New()`.

## Config Struct

```go
type Config struct {
    ReadTimeout    time.Duration
    WriteTimeout   time.Duration
    IdleTimeout    time.Duration
    BodyLimit      int
    DevMode        bool
    SecurityHeaders bool
    Security       SecurityConfig
}
```

## Functional Options

### WithReadTimeout

```go
func WithReadTimeout(d time.Duration) Option
```

Sets the maximum duration for reading the entire request. Default: `30s`.

```go
kruda.New(kruda.WithReadTimeout(60 * time.Second))
```

### WithWriteTimeout

```go
func WithWriteTimeout(d time.Duration) Option
```

Sets the maximum duration for writing the response. Default: `30s`.

### WithIdleTimeout

```go
func WithIdleTimeout(d time.Duration) Option
```

Sets the maximum duration for idle keep-alive connections. Default: `120s`.

### WithBodyLimit

```go
func WithBodyLimit(bytes int) Option
```

Sets the maximum request body size in bytes. Default: `4194304` (4 MB). Bodies exceeding this limit receive HTTP 413.

```go
kruda.New(kruda.WithBodyLimit(10 * 1024 * 1024)) // 10 MB
```

### WithDevMode

```go
func WithDevMode(enabled bool) Option
```

Enables development mode. When enabled:
- Rich HTML error pages are rendered instead of JSON
- `X-Frame-Options` is relaxed to `SAMEORIGIN`

Default: `false`. Also auto-detected via `KRUDA_ENV=development`.

```go
kruda.New(kruda.WithDevMode(true))
```

### WithSecurityHeaders

```go
func WithSecurityHeaders(enabled bool) Option
```

Enables or disables default security headers. Default: `true`.

```go
kruda.New(kruda.WithSecurityHeaders(false)) // disable all security headers
```

### WithLegacySecurityHeaders

```go
func WithLegacySecurityHeaders() Option
```

Restores Phase 1-4 security header defaults for backward compatibility:
- `X-Frame-Options: SAMEORIGIN` (instead of `DENY`)
- `X-XSS-Protection: 1; mode=block` (instead of `0`)
- `Referrer-Policy: no-referrer` (instead of `strict-origin-when-cross-origin`)

### WithErrorHandler

```go
func WithErrorHandler(handler func(c *Ctx, err error)) Option
```

Sets a custom error handler for unhandled errors.

```go
kruda.New(kruda.WithErrorHandler(func(c *kruda.Ctx, err error) {
    c.JSON(500, map[string]string{"error": err.Error()})
}))
```

## SecurityConfig

```go
type SecurityConfig struct {
    XSSProtection         string
    ContentTypeNosniff    string
    XFrameOptions         string
    ReferrerPolicy        string
}
```

Default values:

| Field | Default |
|-------|---------|
| `XSSProtection` | `"0"` |
| `ContentTypeNosniff` | `"nosniff"` |
| `XFrameOptions` | `"DENY"` |
| `ReferrerPolicy` | `"strict-origin-when-cross-origin"` |

## Defaults Summary

| Option | Default |
|--------|---------|
| ReadTimeout | 30s |
| WriteTimeout | 30s |
| IdleTimeout | 120s |
| BodyLimit | 4 MB |
| DevMode | false |
| SecurityHeaders | true |
