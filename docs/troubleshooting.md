# Troubleshooting

## Common Build Errors

### `go: requires go >= 1.25.11`

Kruda requires Go 1.25.11+ on the Go 1.25 release line or Go 1.26.4+ on the Go 1.26 release line for generic type aliases and current standard-library security fixes. Upgrade your Go installation:

```bash
go install golang.org/dl/go1.25.11@latest
go1.25.11 download
```

Or download from [go.dev/dl](https://go.dev/dl/).

### `cannot use generic type C without instantiation`

You're using `C[T]` without specifying the type parameter. Make sure your typed handler specifies both input and output types:

```go
// Wrong — missing type parameters
app.Post("/users", handler)

// Correct — use the generic Post function
kruda.Post[CreateUserInput, User](app, "/users", handler)
```

### `undefined: kruda.Give` or `undefined: kruda.Use`

The DI API uses `Container` methods. Ensure you're using the correct API:

```go
import "github.com/go-kruda/kruda"

// Register a dependency
container := kruda.NewContainer()
container.Give(&MyService{})

// Resolve in a handler (c is *kruda.Ctx)
svc := kruda.MustResolve[*MyService](c)
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

## Choosing the JSON engine

### Sonic does not need CGO

Earlier versions of this page said Sonic required a C compiler. That was wrong:
Sonic is pure Go plus assembly. If you hit `cgo: C compiler not found`, something
else in your build needs CGO — a database driver, `-race` on some platforms — not
Kruda's JSON engine.

### Selecting the engine

The engine is chosen at build time by the `kruda_stdjson` tag and nothing else.
Kruda never switches engines at runtime.

```bash
go build ./...                        # Sonic (default)
go build -tags kruda_stdjson ./...    # encoding/json
```

`CGO_ENABLED=0` is **not** a way to select `encoding/json`. It was, accidentally,
before v1.7.0 — `json/sonic.go` carried a `cgo` build constraint, so every
CGO-disabled build silently got the standard library while still reporting a
default build. Since v1.7.0 a `CGO_ENABLED=0` build gets Sonic like any other.

To confirm what a running binary actually got, read the `listening` startup line:

```
INFO listening addr=:3000 json=sonic
```

### Upgrading and finding your JSON got faster and your start-up slower

Expected, if you build with `CGO_ENABLED=0`. That build used to get
`encoding/json` and now gets Sonic: decoding request bodies gets several times
cheaper, at a fixed cost of roughly +3 ms start-up and +7 MB RSS per process for
Sonic's JIT warm-up. If you spawn a process per request or scale to zero, set
`kruda_stdjson` to keep the faster start.

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
