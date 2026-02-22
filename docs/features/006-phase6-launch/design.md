# Phase 6 — Launch: Design

> Detailed design to be written when Phase 5 is complete.
> See spec Sections 21-23 for full implementation details.

## Key Design Points (from spec)

### GitHub Repository Structure

```
kruda/
├── .github/
│   ├── ISSUE_TEMPLATE/
│   ├── workflows/
│   │   ├── test.yml
│   │   ├── bench.yml
│   │   └── deploy-docs.yml
│   └── FUNDING.yml              # GitHub Sponsors
├── .gitignore
├── LICENSE                       # MIT
├── README.md
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── CHANGELOG.md
├── go.mod, go.sum
├── *.go                          # core files
├── docs/                         # VitePress site
├── examples/                     # 10+ examples
├── cmd/kruda/                   # CLI tool
├── bench/                       # benchmarks
└── internal/                    # internal packages
```

### Release Process

```bash
# On main branch
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions:
# 1. Create release notes from CHANGELOG
# 2. Build binaries (Windows, Linux, macOS)
# 3. Upload to releases
# 4. Publish to pkg.dev (automatic for Go modules)
```

### Blog Post Platform Strategy

- **dev.to** — quick reach, Go community, SEO
- **Medium** — polished articles, backlinks
- **Personal blog** — control, branding
- **Go Blog** (if accepted) — official recognition
- **hashnode** — technical audience

### Community Channel Roles

- **Discord:** technical support, code reviews, announcements
- **Telegram:** quick Q&A, informal discussions
- **GitHub Discussions:** long-form topics, decisions
- **GitHub Issues:** bugs, feature requests

### Launch Timeline (Week 16-18)

```
Week 16:
  - Setup: GitHub repo, Discord, Telegram
  - Finalize: docs, examples, blog posts

Week 17 (Launch Week):
  - Monday: v1.0.0 release (GitHub + pkg.dev)
  - Wednesday: Blog post #1 + Reddit/HN
  - Thursday: Blog posts #2-3 published
  - Friday: Community announcement + Twitter thread

Week 18:
  - Monitor: feedback, issues, community activity
  - Blog post #4: published
  - Promote: share blog posts in communities
  - Gather: testimonials for future use
```

## Marketing Strategy

### Messaging
- **Core value:** Type-safe, fast, minimal boilerplate
- **Target audience:** Go developers frustrated with Gin/Fiber limitations
- **Differentiation:** Generics + DI + auto CRUD = complete platform

### Key Talking Points
1. Go generics for type-safe handlers (compile-time safety)
2. Netpoll performance (200K+ req/sec)
3. Built-in DI container (no third-party)
4. Auto CRUD from service interface (write less code)
5. Zero external dependencies on core (security + simplicity)

### Success Metrics
- GitHub stars: 100+ by end of month
- Community members: Discord 50+, Telegram 30+
- Blog traffic: 1000+ impressions per post
- Package downloads: 1000+ per week

## File Structure (Post-Launch)

```
kruda/
├── blog/                        # Blog posts (new)
│   ├── _posts/
│   │   ├── 2026-02-introducing-kruda.md
│   │   ├── 2026-02-netpoll-performance.md
│   │   ├── 2026-03-di-containers.md
│   │   └── 2026-03-typed-handlers.md
│   └── README.md
├── docs/press-kit/              # Press kit (new)
│   ├── PRESS_KIT.md
│   ├── logo.svg
│   ├── screenshot.png
│   └── one-pager.md
├── .github/
│   ├── ISSUE_TEMPLATE/
│   │   ├── BUG_REPORT.md
│   │   └── FEATURE_REQUEST.md
│   └── FUNDING.yml
└── ... (rest from Phase 5)
```

## Launch Checklist

```
Finalizations:
☐ All Phase 1-5 work complete
☐ Tests passing: go test -v -race ./...
☐ Linter passing: golangci-lint run
☐ Coverage ≥90%
☐ Benchmarks stable

GitHub Setup:
☐ Repository created and public
☐ README complete + benchmarks visible
☐ LICENSE file (MIT)
☐ CONTRIBUTING.md clear
☐ CODE_OF_CONDUCT.md included
☐ Workflows enabled + passing
☐ GitHub Discussions enabled
☐ FUNDING.yml configured

Release:
☐ v1.0.0 tag created
☐ Release notes published
☐ Binaries uploaded
☐ CHANGELOG.md complete
☐ pkg.dev page live

Community:
☐ Discord server created + invite link
☐ Telegram group created + invite link
☐ GitHub Discussions pinned announcement
☐ Twitter account ready

Content:
☐ Blog post #1 complete + published
☐ Blog post #2 complete + published
☐ Blog post #3 complete + published
☐ Blog post #4 complete + scheduled
☐ Press kit ready

Launch Day:
☐ GitHub release published
☐ Blog post #1 shared
☐ Reddit post created
☐ HN submission created
☐ Twitter thread posted
☐ Community posts made
☐ Monitor feedback + issues
```
