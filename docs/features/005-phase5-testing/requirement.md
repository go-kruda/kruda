# Phase 5 — Production Ready: Requirements

> **Goal:** Production quality, security, docs, AI-friendly DX
> **Timeline:** Week 13-16
> **Spec Reference:** Sections 17, 18, 19, 20
> **Depends on:** Phase 1-4 complete

## Milestone

```go
// Security audit passed
// 100% test coverage
// Documentation site live
// 10+ examples
// CI/CD passing
// AI-friendly instructions

kruda version 1.0.0-rc1
```

## Components

| # | Component | File(s) | Est. Days | Priority | Status |
|---|-----------|---------|-----------|----------|--------|
| 1 | Security Audit | `docs/SECURITY.md` | 3 | 🔴 | 🔴 |
| 2 | Unit Test Coverage | `*_test.go` | 5 | 🔴 | 🔴 |
| 3 | Integration Tests | `tests/integration_test.go` | 3 | 🔴 | 🔴 |
| 4 | Documentation Site | `docs/`, VitePress | 4 | 🔴 | 🔴 |
| 5 | CLI Tool | `cmd/kruda/` | 2 | 🟡 | 🔴 |
| 6 | Examples (10+) | `examples/` | 3 | 🔴 | 🔴 |
| 7 | GitHub Actions CI/CD | `.github/workflows/` | 2 | 🔴 | 🔴 |
| 8 | AI-Friendly DX | `llms.txt`, `.cursor/rules`, `.claude/INSTRUCTIONS.md` | 1 | 🟡 | 🔴 |
| 9 | README.md | `README.md` | 1 | 🔴 | 🔴 |

## Key Requirements

### Security Audit
- Path traversal prevention (file serving)
- Header injection prevention
- DoS protection (request size limits, timeout handling)
- SQL injection guidance (user responsibility, ORM/driver)
- CSRF protection (optional middleware)
- XSS prevention (response encoding)
- CORS bypass prevention
- Secure defaults (all security headers on by default)
- Document in `docs/SECURITY.md`

### Test Coverage
- **Target:** 100% line coverage on core components
- Unit tests for every function
- Integration tests for real-world scenarios
- Benchmark regression tests
- OS-specific tests (Windows, Linux, macOS)
- Both transports tested: net/http + Netpoll

### Documentation
- **Format:** VitePress/Starlight (static site)
- **Content:**
  - Getting started guide
  - API reference (auto-generated from code)
  - Architecture guide
  - Configuration options
  - Middleware guide
  - DI container guide
  - Error handling
  - Security best practices
  - Performance tuning
  - FAQ
  - Troubleshooting
- **Live at:** https://kruda.dev

### CLI Tool (`cmd/kruda/`)
```bash
kruda new myapp                    # scaffold project
kruda new myapp --template=api     # predefined templates
kruda generate handler --path=/users   # code generation
kruda validate                     # validate kruda.yaml
kruda dev                          # hot reload dev server (NEW)
kruda dev --port 3000              # with custom port
```

### Hot Reload Dev Server (NEW — DX feature)

`kruda dev` watches `.go` files, auto-rebuilds and restarts on change:

```bash
$ kruda dev
Watching ./...
Building... done (0.8s)
Listening on :3000
[change detected] main.go
Rebuilding... done (0.3s)
Restarting...
Listening on :3000
```

**Requirements:**
- Watch `.go` files recursively via stdlib `os.Stat` polling (500ms interval)
- Debounce: wait 100ms after last change before rebuilding
- Build: `go build -o .kruda-tmp ./`
- Restart: kill old process, start new binary
- Preserve port: reuse same port after restart
- Color output: green for success, red for build errors
- Show build errors inline — don't require switching to terminal
- Optional: proxy mode (keep port alive during rebuild, queue requests)
- Integrated with dev mode error page (Phase 5 `devmode.go`)

### Examples (10+)
1. Hello World
2. JSON API
3. Typed Handlers
4. DI & Services
5. Auto CRUD
6. Middleware
7. Error Handling
8. Testing
9. Health Checks
10. Database Integration
11. Authentication pattern using `contrib/jwt`

### CI/CD
- **On PR:** `go test ./...`, `go vet ./...`, bench regression
- **On Push to main:** publish docs, release artifacts
- **Coverage:** report to Codecov
- **Lint:** golangci-lint
- **Test matrix:** Go 1.24, 1.25+ on Windows, Linux, macOS

### Dev Mode Error Page (NEW — killer DX feature)

Rich error display in development mode, inspired by Next.js and Phoenix (Elixir):

```go
app := kruda.New(kruda.WithDevMode(true))
// When error occurs in dev mode:
// - Rich HTML error page with source code context
// - Stack trace with clickable file links
// - Request details (headers, params, body)
// - Middleware pipeline state
// - Available routes table
// - Environment variables (filtered)
```

**Requirements:**
- `devmode.go` — Dev mode error page renderer
- Show source code ±10 lines around error location
- Syntax-highlighted Go code in the error page
- Collapsible sections: Stack Trace, Request Info, Routes, Config
- Never show in production (gated by `Config.DevMode`)
- Auto-detect dev mode: `KRUDA_ENV=development` or `WithDevMode(true)`
- Performance: only render when error actually happens, zero overhead on success path
- Include "copy error" button for easy reporting
- Show helpful suggestions (e.g., "Did you forget to register this route?")

### AI-Friendly DX
- `llms.txt` — concise project overview for LLMs
- `.cursor/rules.txt` — Cursor IDE rules
- `.claude/INSTRUCTIONS.md` — Claude Code context
- `.github/copilot-instructions.md` — GitHub Copilot context

## Non-Functional Requirements

- **NFR-1:** No breaking API changes from Phase 1-4
- **NFR-2:** 100% of exported functions documented
- **NFR-3:** All tests pass on Go 1.24+
- **NFR-4:** Documentation searchable and indexed
- **NFR-5:** Examples runnable: `go run examples/*/main.go`
- **NFR-6:** Security audit passed by external reviewer
