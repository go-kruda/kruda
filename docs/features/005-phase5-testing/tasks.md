# Phase 5 — Production Ready: Tasks

> Component-level task breakdown for AI/developer to execute.
> Check off as completed. Each task = one focused unit of work.

## Component Status

| # | Component | File(s) | Status | Assignee |
|---|-----------|---------|--------|----------|
| 1 | Security Audit | `docs/SECURITY.md` | 🔴 Todo | - |
| 2 | Unit Test Coverage | `*_test.go` (all) | 🔴 Todo | - |
| 3 | Integration Tests | `tests/integration_test.go` | 🔴 Todo | - |
| 4 | Documentation Site | `docs/`, VitePress | 🔴 Todo | - |
| 5 | Getting Started Docs | `docs/guide/getting-started.md` | 🔴 Todo | - |
| 6 | API Reference Docs | `docs/api/` | 🔴 Todo | - |
| 7 | Example Apps | `examples/*/main.go` (10+) | 🔴 Todo | - |
| 8 | CLI Tool | `cmd/kruda/main.go` | 🔴 Todo | - |
| 9 | GitHub Actions CI | `.github/workflows/test.yml` | 🔴 Todo | - |
| 10 | Benchmark CI | `.github/workflows/bench.yml` | 🔴 Todo | - |
| 11 | AI-Friendly Files | `llms.txt`, `.cursor/`, `.claude/` | 🔴 Todo | - |
| 12 | README.md | `README.md` | 🔴 Todo | - |
| 13 | CHANGELOG.md | `CHANGELOG.md` | 🔴 Todo | - |
| 14 | Dev Mode Error Page | `devmode.go` | 🔴 Todo | - |
| 15 | Hot Reload Dev Server | `cmd/kruda/ dev` command | 🔴 Todo | - |

---

## Detailed Task Breakdown

### Task 1: Security Audit (`docs/SECURITY.md`)
- [ ] Document security model
- [ ] List all security features (secure defaults, headers, etc.)
- [ ] Path traversal: validate file paths, document file serving best practices
- [ ] Header injection: validate header names/values, no CRLF injection
- [ ] DoS protection: body limit, timeout enforcement, document config
- [ ] CORS: document origin validation, preflight handling
- [ ] CSRF: document middleware availability
- [ ] XSS: document response encoding, safe HTML output
- [ ] SQL injection: remind users to use parameterized queries (driver responsibility)
- [ ] Authentication: document security best practices with JWT contrib
- [ ] Dependency security: monitor and update regularly
- [ ] Security reporting: document responsible disclosure process
- [ ] Include vulnerability response timeline

### Task 2: Unit Test Coverage (all *_test.go files)
- [ ] Review all existing tests from Phase 1-4
- [ ] Identify coverage gaps (use `go test -cover`)
- [ ] Add tests for error paths, edge cases, panics
- [ ] **Core coverage requirements:**
  - [ ] `router_test.go` — 100% routes, conflicts, edge cases
  - [ ] `context_test.go` — pooling, thread-safety, header operations
  - [ ] `handler_test.go` — typed handlers, generics
  - [ ] `bind_test.go` — all binding types, validation
  - [ ] `container_test.go` — DI, resolution, circular deps
  - [ ] `resource_test.go` — auto CRUD, all methods
  - [ ] `health_test.go` — health checks, timeouts
  - [ ] `middleware/*_test.go` — all built-in middleware
  - [ ] `transport/*_test.go` — both transports, graceful shutdown
- [ ] Test concurrency: race detector (`go test -race`)
- [ ] Generate coverage report: `go test -coverprofile=coverage.out ./...`
- [ ] Target: ≥90% coverage, ideally 100% on core paths

### Task 3: Integration Tests (`tests/integration_test.go`)
- [ ] Create test scenarios combining multiple components
- [ ] Test: DI + resource + error handling + middleware
- [ ] Test: routing + handlers + validation + response
- [ ] Test: lifecycle hooks + graceful shutdown
- [ ] Test: both transports (net/http + Netpoll) with same app
- [ ] Test: stress scenarios (many concurrent requests)
- [ ] Test: large payloads, slow clients, timeouts
- [ ] Document test setup and expectations

### Task 4: Documentation Site Setup (VitePress)
- [ ] Create `docs/` directory structure
- [ ] Create `.vitepress/config.ts` — VitePress configuration
  - [ ] Site title: "Kruda Framework"
  - [ ] Nav: Guide, API, Examples, Deployment, FAQ
  - [ ] GitHub link, search
- [ ] Create `docs/index.md` — homepage
  - [ ] Hero section with tagline
  - [ ] Feature highlights
  - [ ] Quick start code
  - [ ] Benchmark results
  - [ ] Links to guide/API
- [ ] Set up GitHub Pages publishing in CI
- [ ] Test locally: `npm run docs:dev`

### Task 5: Getting Started Docs
- [ ] `docs/guide/getting-started.md`
  - [ ] Installation: `go get github.com/go-kruda/kruda`
  - [ ] Hello world example
  - [ ] Run and test
  - [ ] Next steps: routing, handlers, middleware
- [ ] `docs/guide/installation.md`
  - [ ] Go version requirement (1.24+)
  - [ ] Install command
  - [ ] Verify installation
- [ ] `docs/guide/quick-start.md`
  - [ ] 5-minute walkthrough
  - [ ] Create new project
  - [ ] Scaffold with CLI tool
- [ ] `docs/guide/routing.md`
  - [ ] Static routes
  - [ ] Parameterized routes
  - [ ] Wildcard routes
  - [ ] Regex constraints
  - [ ] Groups and prefixes
- [ ] `docs/guide/handlers.md`
  - [ ] Basic handlers
  - [ ] Typed handlers (generics)
  - [ ] Request/response binding
  - [ ] Error handling
- [ ] `docs/guide/middleware.md`
  - [ ] What is middleware?
  - [ ] Built-in middleware list
  - [ ] Custom middleware
  - [ ] Middleware chaining
- [ ] `docs/guide/error-handling.md`
  - [ ] KrudaError type
  - [ ] Status codes
  - [ ] Error handlers
  - [ ] Custom errors
- [ ] `docs/guide/security.md` (reference to SECURITY.md)

### Task 6: API Reference Docs
- [ ] `docs/api/app.md` — App struct, methods
- [ ] `docs/api/context.md` — Ctx struct, request/response API
- [ ] `docs/api/handler.md` — Handler types, C[T], typed functions
- [ ] `docs/api/container.md` — DI Container API
- [ ] `docs/api/resource.md` — Auto CRUD resource
- [ ] `docs/api/middleware.md` — Middleware types, built-in middleware
- [ ] Auto-generate from code comments where possible

### Task 7: Example Apps (10+ examples)
Create in `examples/`:
- [ ] `hello/main.go` — basic GET endpoint
- [ ] `json-api/main.go` — POST/PUT/DELETE with JSON
- [ ] `typed-handlers/main.go` — C[T], generic handlers
- [ ] `di-services/main.go` — DI container, service injection
- [ ] `auto-crud/main.go` — Resource with auto CRUD
- [ ] `middleware/main.go` — custom middleware, logging
- [ ] `error-handling/main.go` — error responses, custom errors
- [ ] `testing/main.go` + `testing_test.go` — test client usage
- [ ] `groups/main.go` — route groups, prefixes, guards
- [ ] `health-check/main.go` — HealthChecker interface, /health endpoint
- [ ] `database/main.go` — database integration example
- [ ] `auth/main.go` — Authentication pattern using `contrib/jwt`

Each example:
- [ ] Runnable: `go run examples/xxx/main.go`
- [ ] Documented: comments explaining key concepts
- [ ] Include Makefile with build, run, test targets
- [ ] Include README with description and usage

### Task 8: CLI Tool (`cmd/kruda/`)
- [ ] Create `cmd/kruda/main.go` with Cobra
- [ ] Implement `kruda new <name>` — scaffold project
  - [ ] Create directory structure
  - [ ] Generate `go.mod`, `main.go`, `Makefile`
  - [ ] Create basic hello world app
- [ ] Implement `kruda new <name> --template=api` — API template
  - [ ] Include DI, services, example CRUD
- [ ] Implement `kruda generate handler --path=/users` — code generation
  - [ ] Create handler file with template
- [ ] Implement `kruda validate` — validate kruda.yaml
- [ ] Create `cmd/kruda/templates/` with project templates
- [ ] Document CLI in README

### Task 9: GitHub Actions Test CI (`.github/workflows/test.yml`)
- [ ] Create workflow: trigger on push + PR
- [ ] Matrix: Go 1.24, 1.25+ × Windows, Linux, macOS
- [ ] Steps:
  - [ ] Checkout code
  - [ ] Setup Go
  - [ ] Run `go test -v -race -coverprofile=coverage.out ./...`
  - [ ] Upload coverage to Codecov
  - [ ] Run `go vet ./...`
  - [ ] Run linter (golangci-lint)
- [ ] Report results in PR

### Task 10: GitHub Actions Benchmark CI (`.github/workflows/bench.yml`)
- [ ] Create workflow: trigger on push to main, schedule daily
- [ ] Run: `go test -bench=. -benchmem ./bench/`
- [ ] Store baseline results
- [ ] Compare to previous run, detect regressions >10%
- [ ] Comment on PR with results
- [ ] Update README with latest benchmarks

### Task 11: AI-Friendly Files
- [ ] Create `llms.txt` — 2-3KB project summary
  - [ ] What is Kruda
  - [ ] Core features
  - [ ] File structure
  - [ ] Key APIs
- [ ] Create `.cursor/rules.txt` — Cursor IDE rules
  - [ ] Code style
  - [ ] Go idioms
  - [ ] Testing patterns
- [ ] Create `.claude/INSTRUCTIONS.md` — Claude Code context
  - [ ] Reference to CLAUDE.md
  - [ ] Quick tips for AI code generation
- [ ] Create `.github/copilot-instructions.md` — GitHub Copilot rules

### Task 12: README.md
- [ ] Create `README.md` at repo root
  - [ ] Project description
  - [ ] Features list
  - [ ] Installation
  - [ ] Quick start code
  - [ ] Benchmarks vs Fiber/Gin/Echo/Hertz
  - [ ] Examples list
  - [ ] Documentation link
  - [ ] Contributing guidelines
  - [ ] License (MIT)
  - [ ] Author credit

### Task 13: CHANGELOG.md
- [ ] Create `CHANGELOG.md` documenting all releases
  - [ ] Phase 1 changes
  - [ ] Phase 2 changes
  - [ ] Phase 3 changes
  - [ ] Phase 4 changes
  - [ ] Phase 5 changes
  - [ ] Format: keep-a-changelog style

### Task 14: Dev Mode Error Page (NEW)
- [ ] Create `devmode.go`
  - [ ] `DevErrorHandler` — renders rich HTML error page
  - [ ] Source code extraction: read ±10 lines around error location via `runtime.Caller`
  - [ ] Syntax highlighting: embedded CSS for Go code
  - [ ] Collapsible sections: Stack Trace, Request Info, Routes, Config
  - [ ] "Copy error" button (clipboard JS)
  - [ ] Helpful suggestions engine (e.g. missing route, unregistered service)
- [ ] Gate behind `Config.DevMode` — never render in production
- [ ] Auto-detect: `KRUDA_ENV=development` environment variable
- [ ] HTML template: embedded via `embed.FS` (Go 1.16+)
- [ ] Test: error page renders correctly with source context
- [ ] Test: production mode returns standard JSON error (no HTML)
- [ ] Test: performance — zero overhead on success path

### Task 15: Hot Reload Dev Server (NEW)
- [ ] Implement `kruda dev` CLI command
  - [ ] Watch `.go` files recursively (stdlib os.Stat polling, 100ms interval)
  - [ ] Debounce: 100ms after last change
  - [ ] Build: `go build -o .kruda-tmp ./`
  - [ ] Kill old process, start new binary
  - [ ] Reuse same port
  - [ ] Color output: green=success, red=build errors
  - [ ] Show build errors inline
- [ ] Optional proxy mode: keep port alive during rebuild (deferred)
- [ ] `kruda dev --port 3000` flag
- [ ] Test: file change triggers rebuild
- [ ] Test: build error displays correctly
- [ ] Integration with `devmode.go` error page

---

## Execution Order (Recommended)

1. Set up GitHub Actions CI/CD (get feedback loop first)
2. Generate `bench/baseline.txt` (prerequisite for benchmark regression)
3. Security audit + write `docs/SECURITY.md`
4. Review coverage, add missing tests (`*_test.go`)
5. Write integration tests
6. Implement CLI Tier 1: `kruda new` + `kruda dev`
7. Create 10+ example apps
8. Set up VitePress site + publish
9. Write getting started guides + API reference docs
10. Create AI-friendly files + README.md + CHANGELOG.md
11. Implement CLI Tier 2: `kruda generate` + `kruda validate` (if time allows)
12. Run full test suite, verify 100% on core + ≥90% overall
13. Deploy docs site
14. Tag v1.0.0-rc1

---

## Success Criteria

- All tests pass on Go 1.24+ with -race flag
- Coverage ≥90% (core ≥100%)
- Documentation complete and searchable
- 10+ examples runnable
- CLI tool working: `kruda new myapp`
- CI/CD passing: test + lint + bench
- Security audit complete, no critical issues
- Documentation site live at https://kruda.dev
