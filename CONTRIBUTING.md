# Contributing to Kruda

Thank you for your interest in contributing to Kruda!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/kruda.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit and push
7. Open a Pull Request

## Requirements

- **Go 1.25+** required
- All tests must pass: `go test ./...`
- Code must be formatted: `gofmt -s -w .`
- Code must pass vet: `go vet ./...`

## Project Structure

```
kruda/
├── *.go              # Core framework (minimal external deps)
├── middleware/        # Built-in middleware (Logger, Recovery, CORS, etc.)
├── transport/         # Transport implementations
│   └── wing/          # Wing transport (epoll+eventfd, Linux only)
├── contrib/           # Optional modules (JWT, WebSocket, RateLimit)
├── json/              # JSON engine abstraction
├── examples/          # Example applications
├── docs/              # VitePress documentation site
├── bench/             # Benchmarks and reproducible comparisons
└── cmd/kruda/         # CLI tool
```

## Code Guidelines

- **Minimal external deps** in core — Sonic JSON (opt-out via `kruda_stdjson` build tag), fasthttp transport
- All exported types and functions must have doc comments
- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `slog` for logging (Go 1.21+ standard)
- Functional options pattern for configuration

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Wing transport tests (Linux only)
cd transport/wing && go test ./...
```

## Contrib Modules

Each `contrib/` package has its own `go.mod` and can have external dependencies:

```bash
cd contrib/jwt && go test ./...
cd contrib/ws && go test ./...
cd contrib/ratelimit && go test ./...
```

## Commit Messages

Use clear, descriptive commit messages:

- `feat: add WebSocket broadcast support`
- `fix: prevent context reuse in concurrent handlers`
- `perf: reduce allocations in radix tree matching`
- `docs: update typed handlers guide`
- `test: add integration tests for DI container`

## Pull Request Process

1. Update documentation if your change affects the public API
2. Add or update tests for your changes
3. Ensure CI passes (tests, vet, fmt)
4. One approval required for merge

## Security Issues

**Do not open public issues for security vulnerabilities.** See [SECURITY.md](SECURITY.md) for our responsible disclosure process.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
