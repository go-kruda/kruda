# Config

Configuration is set via functional options passed to `kruda.New()`.

## Functional Options

### WithReadTimeout

```go
func WithReadTimeout(d time.Duration) Option
```

Sets the maximum duration for reading the entire request. Default: `30s`.

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

### WithMaxBodySize

```go
func WithMaxBodySize(size int) Option
```

Alias for `WithBodyLimit`.

### WithDevMode

```go
func WithDevMode(enabled bool) Option
```

Enables development mode. When enabled:
- Rich HTML error pages are rendered instead of JSON
- `X-Frame-Options` is relaxed to `SAMEORIGIN`

Default: `false`. Also auto-detected via `KRUDA_ENV=development`.

### WithSecureHeaders

```go
func WithSecureHeaders() Option
```

Explicitly enables default security headers. Use this to make security enablement explicit in your code.

Security headers are opt-in. Use `WithSecureHeaders()` to enable, or `WithSecurity()` to enable all security features.

### WithLegacySecurityHeaders

```go
func WithLegacySecurityHeaders() Option
```

Restores Phase 1-4 security header defaults for backward compatibility.

### WithErrorHandler

```go
func WithErrorHandler(h func(c *Ctx, err *KrudaError)) Option
```

Sets a custom error handler. Receives `*KrudaError` (not `error`).

### WithLogger

```go
func WithLogger(l *slog.Logger) Option
```

Sets a custom structured logger.

### WithTransport

```go
func WithTransport(t transport.Transport) Option
```

Sets a custom transport implementation.

### FastHTTP

```go
func FastHTTP() Option
```

Selects the fasthttp transport. On Linux the default is **Wing** (epoll+eventfd) which is faster — use `FastHTTP()` only if you need fasthttp compatibility. On macOS, fasthttp is already the default.

### NetHTTP

```go
func NetHTTP() Option
```

Selects the net/http transport for HTTP/2, TLS, and Windows compatibility.

### WithTLS

```go
func WithTLS(certFile, keyFile string) Option
```

Enables TLS with the given certificate and key files.

### WithHTTP3

```go
func WithHTTP3(certFile, keyFile string) Option
```

Enables HTTP/3 dual-stack (QUIC + TCP) with TLS.

### WithTrustProxy

```go
func WithTrustProxy(trust bool) Option
```

When true, trusts `X-Forwarded-For` / `X-Real-IP` headers. Default: `false`.

### WithShutdownTimeout

```go
func WithShutdownTimeout(d time.Duration) Option
```

Sets the graceful shutdown timeout. Default: `10s`.

### WithJSONEncoder / WithJSONDecoder

```go
func WithJSONEncoder(enc func(v any) ([]byte, error)) Option
func WithJSONDecoder(dec func(data []byte, v any) error) Option
```

Override the JSON encoder/decoder.

### WithEnvPrefix

```go
func WithEnvPrefix(prefix string) Option
```

Loads configuration from environment variables with the given prefix.

### WithValidator

```go
func WithValidator(v *Validator) Option
```

Sets a custom validator for typed handler input validation.

### WithContainer

```go
func WithContainer(c *Container) Option
```

Sets the DI container for the app.

### WithOpenAPIInfo

```go
func WithOpenAPIInfo(title, version, description string) Option
```

Sets OpenAPI spec metadata.

### WithOpenAPIPath

```go
func WithOpenAPIPath(path string) Option
```

Sets the path to serve the OpenAPI spec (default: `/openapi.json`).

### WithOpenAPITag

```go
func WithOpenAPITag(name, description string) Option
```

Adds an OpenAPI tag definition.

## Defaults Summary

| Option | Default |
|--------|---------|
| ReadTimeout | 30s |
| WriteTimeout | 30s |
| IdleTimeout | 120s |
| BodyLimit | 4 MB |
| ShutdownTimeout | 10s |
| DevMode | false |
| SecurityHeaders | false |
| TrustProxy | false |
| TransportName | "wing" (Linux), "fasthttp" (macOS), "nethttp" (Windows) |
