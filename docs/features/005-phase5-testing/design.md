# Phase 5 — Production Ready: Design

> Detailed design to be written when Phase 4 is complete.
> See spec Sections 17-20 for full implementation details.

## Key Design Points (from spec)

### Security Audit (`docs/SECURITY.md`)

- **Path Traversal:** Validate file paths, reject `..` segments
- **Header Injection:** Validate header names/values, reject CRLF
- **DoS Protection:** Request size limits, timeout enforcement
- **CORS:** Validate origin, handle preflight correctly
- **Secure Defaults:** SecurityConfig enabled by default
- **Dependency Security:** Monitor for CVEs in dependencies

### Test Coverage Strategy

```
├── unit/
│   ├── router_test.go
│   ├── context_test.go
│   ├── handler_test.go
│   ├── bind_test.go
│   ├── validation_test.go
│   ├── container_test.go
│   ├── resource_test.go
│   ├── health_test.go
│   └── middleware/*_test.go
├── integration/
│   ├── end_to_end_test.go
│   ├── transport_compat_test.go
│   └── di_integration_test.go
└── bench/
    └── *_test.go
```

- Minimum: 90% coverage, target: 100% on core paths
- Branch coverage: test both success and error paths
- Edge cases: empty input, nil values, timeouts, panics

### Documentation Site (VitePress)

```
docs/
├── .vitepress/
│   └── config.ts           # VitePress configuration
├── index.md                # Homepage
├── guide/
│   ├── getting-started.md
│   ├── installation.md
│   ├── quick-start.md
│   ├── routing.md
│   ├── handlers.md
│   ├── middleware.md
│   ├── error-handling.md
│   └── security.md
├── api/
│   ├── app.md
│   ├── context.md
│   ├── handler.md
│   ├── container.md
│   └── resource.md
├── examples/
│   ├── hello.md
│   ├── json-api.md
│   ├── di-services.md
│   └── ...
├── deployment/
│   ├── performance.md
│   ├── benchmarks.md
│   └── tuning.md
└── faq.md
```

### CLI Tool (`cmd/kruda/`)

```go
// cmd/kruda/main.go
func main() {
    rootCmd := &cobra.Command{}

    newCmd := &cobra.Command{
        Use: "new",
        Run: scaffoldProject,
    }

    generateCmd := &cobra.Command{
        Use: "generate",
        Run: generateCode,
    }

    rootCmd.AddCommand(newCmd, generateCmd)
    rootCmd.Execute()
}
```

- Uses Cobra for CLI
- Templates stored in `cmd/kruda/templates/`
- Code generation via text/template

### Examples Structure

```
examples/
├── hello/
│   └── main.go
├── json-api/
│   ├── main.go
│   └── Makefile
├── di-services/
│   ├── main.go
│   └── cmd_test.go
├── middleware/
│   └── main.go
└── ...
```

- Each example: runnable, documented, tested
- Makefile: build, run, test targets
- README per example

### GitHub Actions CI/CD

```yaml
# .github/workflows/test.yml
on: [push, pull_request]
jobs:
  test:
    strategy:
      matrix:
        go-version: [1.24, 1.25]
        os: [ubuntu-latest, windows-latest, macos-latest]
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ matrix.go-version }}
      - run: go test -v -race -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v3

# .github/workflows/bench.yml
# Run benchmarks, compare to baseline, report regressions
```

### AI-Friendly Files

- `llms.txt` — 2-3KB summary of project, APIs, file structure
- `.cursor/rules.txt` — Cursor-specific rules (style, patterns)
- `.claude/INSTRUCTIONS.md` — Claude Code context (minimal, reference to CLAUDE.md)
- `.github/copilot-instructions.md` — GitHub Copilot rules

## File Dependencies

```
docs/                  (VitePress site, markdown files)
cmd/kruda/             (CLI tool, depends on kruda.go)
.github/workflows/     (CI/CD, no code deps)
examples/*/main.go     (runnable examples, depend on kruda.go)
tests/                 (integration tests)
```

## Testing Strategy

- Unit: test each component in isolation
- Integration: test real scenarios (DI + resource + error handling)
- Benchmark: regression tests on performance
- Security: security audit checklist + tests
- Coverage: use `go test -cover`, track with Codecov
- OS-specific: skip tests based on GOOS
