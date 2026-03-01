# Troubleshooting

## Common Build Errors

### `go: requires go >= 1.24`

Kruda requires Go 1.24+ for generic type aliases. Upgrade your Go installation:

```bash
go install golang.org/dl/go1.24.0@latest
go1.24.0 download
```

Or download from [go.dev/dl](https://go.dev/dl/).

### `cannot use generic type C without instantiation`

You're using `C[T]` without specifying the type parameter. Make sure your typed handler specifies the request struct:

```go
// Wrong
app.Post("/users", kruda.TypedHandler(handler))

// Correct
app.Post("/users", kruda.TypedHandler[CreateUserInput](handler))
```

### `undefined: kruda.Give` or `undefined: kruda.Use`

Ensure you're importing the correct package and using Go 1.24+:

```go
import "github.com/go-kruda/kruda"

kruda.Give(app, factory)
svc := kruda.Use[*MyService](c)
```

## Transport Selection Issues

### fasthttp not available on Windows

fasthttp uses optimized networking and is available on all platforms. Kruda automatically selects the best transport based on your configuration.

To verify which transport is active, check the startup log output.

### Port already in use

If you see `bind: address already in use`, another process is using the port. Find and kill it:

```bash
# Find process on port 3000
lsof -i :3000

# Kill it
kill -9 <PID>
```

Or use a different port:

```go
app.Listen(":3001")
```

## CGO and Sonic JSON

### `cgo: C compiler not found`

Sonic JSON requires CGO for SIMD optimizations. If you don't have a C compiler:

Option 1 — Install a C compiler:
```bash
# macOS
xcode-select --install

# Ubuntu/Debian
sudo apt install build-essential

# Windows
# Install MinGW or use WSL
```

Option 2 — Use stdlib JSON (no CGO):
```bash
go build -tags kruda_stdjson ./...
```

### Sonic fallback to encoding/json

If Sonic can't initialize (missing CPU features, CGO disabled), Kruda silently falls back to `encoding/json`. This is safe — the API is identical, only performance differs.

To force stdlib JSON explicitly:

```bash
export CGO_ENABLED=0
go build -tags kruda_stdjson ./...
```

## Windows Compatibility

### Transport tests skipped

Transport tests may be excluded on certain platforms via build tags. This is expected behavior for platform-specific optimizations.

### Path separator issues

Kruda normalizes all paths to forward slashes internally. If you're constructing paths manually, always use `/`:

```go
// Correct
app.Get("/users/:id", handler)

// Avoid OS-specific separators
// app.Get("\\users\\:id", handler) // Don't do this
```

### File watcher in `kruda dev`

The `kruda dev` hot reload uses `os.Stat` polling, which works identically on all platforms including Windows, Docker, NFS, and WSL.

## Dev Mode Error Page

### Error page not showing

Ensure dev mode is enabled:

```go
app := kruda.New(kruda.WithDevMode(true))
```

Or set the environment variable:

```bash
export KRUDA_ENV=development
```

Dev mode defaults to `false` for security — it must be explicitly enabled.

### Sensitive data in error page

The dev error page filters environment variables containing `SECRET`, `PASSWORD`, `TOKEN`, `KEY`, `CREDENTIAL`, or `AUTH` (case-insensitive). If you see sensitive data, check that your env var names include one of these keywords.

## Getting Help

- [GitHub Issues](https://github.com/go-kruda/kruda/issues) — bug reports and feature requests
- [GitHub Discussions](https://github.com/go-kruda/kruda/discussions) — questions and community help
- [FAQ](/faq) — frequently asked questions
