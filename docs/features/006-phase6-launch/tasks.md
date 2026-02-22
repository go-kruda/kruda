# Phase 6 — Launch: Tasks

> Component-level task breakdown for AI/developer to execute.
> Check off as completed. Each task = one focused unit of work.

## Component Status

| # | Component | File(s) | Status | Assignee |
|---|-----------|---------|--------|----------|
| 1 | GitHub Repo Setup | README, LICENSE, CONTRIBUTING | 🔴 Todo | - |
| 2 | Issue Templates | `.github/ISSUE_TEMPLATE/` | 🔴 Todo | - |
| 3 | Release Notes | `CHANGELOG.md` completion | 🔴 Todo | - |
| 4 | v1.0.0 Release Tag | git tag + artifacts | 🔴 Todo | - |
| 5 | Blog Post #1 | Introducing Kruda | 🔴 Todo | - |
| 6 | Blog Post #2 | Netpoll Performance | 🔴 Todo | - |
| 7 | Blog Post #3 | DI Containers | 🔴 Todo | - |
| 8 | Blog Post #4 | Typed Handlers | 🔴 Todo | - |
| 9 | Press Kit | `docs/press-kit/` | 🔴 Todo | - |
| 10 | Discord Community | Server setup + invite | 🔴 Todo | - |
| 11 | Telegram Community | Group setup + invite | 🔴 Todo | - |
| 12 | GitHub Discussions | Enable + pin announcement | 🔴 Todo | - |
| 13 | Twitter Account | Setup + profile | 🔴 Todo | - |
| 14 | Launch Posts | Reddit, HN, Twitter | 🔴 Todo | - |
| 15 | Social Promotion | Share blog posts + links | 🔴 Todo | - |
| 16 | Migration Guide: Gin | `docs/guide/from-gin.md` | 🔴 Todo | - |
| 17 | Migration Guide: Fiber | `docs/guide/from-fiber.md` | 🔴 Todo | - |
| 18 | Migration Guide: stdlib | `docs/guide/from-stdlib.md` | 🔴 Todo | - |
| 19 | Interactive Playground | `play.kruda.dev` | 🔴 Todo | - |

---

## Detailed Task Breakdown

### Task 1: GitHub Repository Setup
- [ ] Create repository: `https://github.com/go-kruda/kruda`
  - [ ] Initialize with README, LICENSE, .gitignore
  - [ ] Set default branch: main
  - [ ] Enable GitHub Discussions
  - [ ] Enable Issues + Pull Requests
- [ ] Create `LICENSE` file (MIT license, copyright 2026 Tiger)
- [ ] Create `README.md` (from Phase 5)
  - [ ] Include: project description, features, installation, quick start
  - [ ] Include: benchmarks vs other frameworks
  - [ ] Include: examples list with links
  - [ ] Include: documentation link
  - [ ] Include: contributing guidelines
- [ ] Create `CONTRIBUTING.md`
  - [ ] Setup: fork, clone, branch naming
  - [ ] Development: build, test, lint commands
  - [ ] Submission: PR process, code review expectations
  - [ ] DCO: Developer Certificate of Origin (optional)
- [ ] Create `CODE_OF_CONDUCT.md` (Contributor Covenant)
- [ ] Add `.github/FUNDING.yml` for GitHub Sponsors (optional)
- [ ] Add `.editorconfig` for consistent formatting
- [ ] Add `.gitignore` (Go standard)
- [ ] Verify: all tests pass, docs build, workflows enabled

### Task 2: Issue Templates
- [ ] Create `.github/ISSUE_TEMPLATE/BUG_REPORT.md`
  - [ ] Template: title, description, reproduction steps, expected vs actual
  - [ ] Labels auto-assignment: bug
- [ ] Create `.github/ISSUE_TEMPLATE/FEATURE_REQUEST.md`
  - [ ] Template: title, description, use case, proposed solution
  - [ ] Labels auto-assignment: enhancement
- [ ] Create `.github/ISSUE_TEMPLATE/QUESTION.md`
  - [ ] Template: title, question, context
  - [ ] Suggest: use GitHub Discussions instead

### Task 3: Release Notes
- [ ] Complete `CHANGELOG.md` (from Phase 5)
  - [ ] Consolidate all phase changes
  - [ ] Format: keep-a-changelog style
  - [ ] Highlight: major features, breaking changes
  - [ ] Include: contributor credits
- [ ] Create release summary document
  - [ ] 1-2 paragraph overview
  - [ ] Key stats: 6 phases, X files, Y LOC
  - [ ] Performance: benchmarks vs alternatives
  - [ ] Community: thank you to contributors

### Task 4: v1.0.0 Release Tag
- [ ] Create git tag: `git tag -a v1.0.0 -m "Release v1.0.0"`
- [ ] Push tag: `git push origin v1.0.0`
- [ ] Create GitHub Release
  - [ ] Tag: v1.0.0
  - [ ] Title: "Kruda Framework v1.0.0"
  - [ ] Description: release summary + feature highlights
  - [ ] Artifacts: attach binary downloads (optional)
- [ ] Verify: pkg.dev automatically publishes
- [ ] Document: add to CHANGELOG.md

### Task 5: Blog Post #1 - "Introducing Kruda"
- [ ] **File:** `blog/_posts/2026-02-introducing-kruda.md`
- [ ] **Sections:**
  - [ ] Problem: Gin/Fiber fast but not type-safe, Echo verbose, Fuego new/unproven
  - [ ] Solution: Kruda = type-safe + fast + simple
  - [ ] Core features: generics, typed handlers, DI, auto CRUD
  - [ ] Demo code: hello world → JSON API → typed handler
  - [ ] Benchmarks: vs Fiber, Gin, Echo (simple GET request)
  - [ ] Community: Discord/Telegram links
  - [ ] Call to action: try it, provide feedback
- [ ] **Publish to:** dev.to, Medium, personal blog
- [ ] **Target:** 1500-2000 words
- [ ] **SEO:** keywords: Go framework, type-safe, generics, REST API

### Task 6: Blog Post #2 - "Netpoll Performance"
- [ ] **File:** `blog/_posts/2026-02-netpoll-performance.md`
- [ ] **Sections:**
  - [ ] What is Netpoll: event multiplexing (epoll/kqueue)
  - [ ] Why it matters: massive throughput, low latency
  - [ ] Kruda's implementation: Transport interface, auto-selection
  - [ ] Benchmarks: net/http vs Netpoll (throughput, latency, memory)
  - [ ] Code example: auto-selection logic
  - [ ] Tuning: tips for maximum performance
  - [ ] Limitations: Windows fallback to net/http
- [ ] **Publish to:** dev.to, Medium (technical audience)
- [ ] **Target:** 2000+ words, include graphs
- [ ] **SEO:** keywords: Netpoll, Go performance, HTTP, benchmarks

### Task 7: Blog Post #3 - "Dependency Injection in Go"
- [ ] **File:** `blog/_posts/2026-03-di-containers.md`
- [ ] **Sections:**
  - [ ] Why DI matters: testability, flexibility, reusability
  - [ ] Patterns: singleton, transient, lazy, named
  - [ ] Kruda's Container: type-safe, reflection-based, zero external deps
  - [ ] Example: service → repository → database chain
  - [ ] Modules: encapsulate related services
  - [ ] Comparison: spring-like DI, Go idiomatic
  - [ ] Best practices: when to use, when not to
- [ ] **Publish to:** dev.to, Medium, hashnode
- [ ] **Target:** 1800-2200 words
- [ ] **SEO:** keywords: dependency injection, Go, containers, testability

### Task 8: Blog Post #4 - "Go Generics in Practice"
- [ ] **File:** `blog/_posts/2026-03-typed-handlers.md`
- [ ] **Sections:**
  - [ ] Go 1.18+ generics explained: syntax, constraints
  - [ ] Kruda's C[T] pattern: typed handler struct
  - [ ] Pre-compilation: parser built at registration, zero reflection at runtime
  - [ ] Validation: struct tags, pre-compiled validators
  - [ ] Type safety: compile-time checks, better IDE support
  - [ ] Performance: vs interface{} + reflection
  - [ ] Example: building a typed JSON API
- [ ] **Publish to:** dev.to, Medium, Go Blog (if accepted)
- [ ] **Target:** 1800-2000 words
- [ ] **SEO:** keywords: Go generics, type-safe, handlers, performance

### Task 9: Press Kit
- [ ] Create `docs/press-kit/` directory
- [ ] Create `docs/press-kit/PRESS_KIT.md`
  - [ ] Project description (2-3 paragraphs)
  - [ ] Founder: Tiger
  - [ ] License: MIT
  - [ ] Repository: https://github.com/go-kruda/kruda
  - [ ] Documentation: https://kruda.dev
  - [ ] Community: Discord, Telegram, GitHub Discussions
  - [ ] Key features: type-safe, fast, minimal boilerplate
  - [ ] Benchmarks: performance highlights
  - [ ] Vision: complete Go web framework ecosystem
- [ ] Add `docs/press-kit/one-pager.md` — single page project summary
- [ ] Add `docs/press-kit/MEDIA_KIT.md` (optional)
  - [ ] Logo files: SVG, PNG, colors
  - [ ] Tagline options
  - [ ] Sample quotes
  - [ ] Contact info

### Task 10: Discord Community Setup
- [ ] Create Discord server: "Kruda Framework"
- [ ] Channels:
  - [ ] `#announcements` — releases, blog posts
  - [ ] `#general` — discussions, off-topic
  - [ ] `#help` — questions, troubleshooting
  - [ ] `#showcase` — projects using Kruda
  - [ ] `#development` — core development, RFCs
  - [ ] `#random` — fun, memes
- [ ] Roles: Moderator, Contributor, Member
- [ ] Rules: posted in #rules
- [ ] Invite link: add to README, docs, first GitHub issue
- [ ] Set up welcome message with bot
- [ ] Create moderation guidelines

### Task 11: Telegram Community Setup
- [ ] Create Telegram group: "Kruda Framework"
- [ ] Description: quick Q&A, announcements
- [ ] Invite link: pinned in group
- [ ] Add to README and docs
- [ ] Rules: pinned message
- [ ] Bot: forward announcements from GitHub
- [ ] Target: 30+ members by end of week

### Task 12: GitHub Discussions Enable
- [ ] Enable Discussions in repo settings
- [ ] Create discussion categories:
  - [ ] `Announcements` — releases, news
  - [ ] `General Discussion` — topics
  - [ ] `Feature Requests` — ideas
  - [ ] `Q&A` — troubleshooting
  - [ ] `Show & Tell` — projects
- [ ] Pin announcement: "Welcome to Kruda"
- [ ] Link from README, docs

### Task 13: Twitter Account Setup
- [ ] Create @kruda_framework (or similar)
- [ ] Bio: "Type-safe, fast Go web framework"
- [ ] Links: GitHub, docs, discord
- [ ] Profile picture: Kruda logo
- [ ] First tweet: v1.0.0 announcement
- [ ] Engagement: retweet community projects

### Task 14: Launch Day Posts
- [ ] **Reddit:** r/golang post
  - [ ] Title: "Show HN: Kruda — Type-Safe Go Web Framework"
  - [ ] Content: project overview + benchmarks + feature highlights
  - [ ] Tone: honest, invite feedback
  - [ ] Engage: answer questions promptly
- [ ] **Hacker News:** HN submission
  - [ ] Title: "Kruda: Type-Safe Go Web Framework"
  - [ ] URL: GitHub repo
  - [ ] Text: brief intro, links to blog
  - [ ] Tag: Show HN
- [ ] **Twitter:** thread + individual posts
  - [ ] Post #1: "Introducing Kruda Framework v1.0.0 🎉"
  - [ ] Post #2: Type-safety + performance
  - [ ] Post #3: Built-in DI + auto CRUD
  - [ ] Post #4: Benchmarks
  - [ ] Post #5: Links to discord/docs
- [ ] **Go Forum:** golang-nuts post (if appropriate)
- [ ] **Dev.to:** share blog posts + engage comments

### Task 15: Social Promotion
- [ ] Share blog posts across platforms
  - [ ] Post #1: dev.to, Medium, Twitter, LinkedIn
  - [ ] Post #2: dev.to, Medium, Twitter, Reddit, HN
  - [ ] Post #3: dev.to, Medium, Twitter, LinkedIn
  - [ ] Post #4: dev.to, Medium, Twitter
- [ ] Engage with comments (Reddit, HN, Twitter)
- [ ] Respond to feedback and questions
- [ ] Share community projects using Kruda
- [ ] Thank early adopters + contributors
- [ ] Create weekly digest of discussions + issues

### Task 16: Migration Guide — Gin (NEW)
- [ ] Create `docs/guide/from-gin.md`
  - [ ] Side-by-side: Gin handler vs Kruda handler
  - [ ] Side-by-side: Gin typed handler vs Kruda C[T]
  - [ ] Context API mapping table: gin.Context → kruda.Ctx
  - [ ] Router pattern mapping: Gin → Kruda
  - [ ] Middleware migration examples
  - [ ] Common gotchas when migrating

### Task 17: Migration Guide — Fiber (NEW)
- [ ] Create `docs/guide/from-fiber.md`
  - [ ] Side-by-side: Fiber handler vs Kruda handler
  - [ ] Context safety: explain string copy behavior (Fiber pitfall)
  - [ ] Performance: fasthttp vs Netpoll comparison
  - [ ] Router pattern mapping: Fiber → Kruda
  - [ ] Common gotchas: context reuse, string safety

### Task 18: Migration Guide — stdlib (NEW)
- [ ] Create `docs/guide/from-stdlib.md`
  - [ ] Side-by-side: http.HandlerFunc vs kruda.HandlerFunc
  - [ ] Middleware pattern migration
  - [ ] Benefits: typed handlers, auto validation, DI
  - [ ] `kruda.FromHTTP()` adapter usage

### Task 19: Interactive Playground (NEW)
- [ ] Choose implementation: Go Playground API fork vs WASM vs custom backend
- [ ] Create playground UI: code editor, run button, output panel
- [ ] Pre-load 10+ example templates
- [ ] Share feature: generate URL with embedded code
- [ ] Deploy to `play.kruda.dev`
- [ ] Link from docs homepage and getting started guide

---

## Launch Timeline (Week 16-18)

### Week 16 — Preparation
- [ ] Monday-Tuesday: Complete Phase 5 work
- [ ] Wednesday: Finalize GitHub repo setup
- [ ] Thursday: Write all 4 blog posts
- [ ] Friday: Setup Discord, Telegram, Twitter

### Week 17 — Launch Week
- [ ] Monday: Create v1.0.0 tag + release
  - [ ] GitHub release + artifacts
  - [ ] CHANGELOG finalized
  - [ ] Announce on Discord/Telegram
- [ ] Tuesday-Wednesday: Blog post #1 published + promoted
  - [ ] dev.to, Medium, personal blog
  - [ ] Reddit post created
- [ ] Thursday: Blog posts #2-3 published
  - [ ] dev.to, Medium
  - [ ] HN submission
- [ ] Friday: Official launch
  - [ ] Blog post #4 published
  - [ ] Twitter thread
  - [ ] Community announcements

### Week 18 — Follow-up
- [ ] Monitor: feedback, issues, stars
- [ ] Engagement: respond to comments, help new users
- [ ] Blog promotion: share across platforms
- [ ] Gather: testimonials, success stories
- [ ] Analysis: track metrics (stars, downloads, traffic)

---

## Success Metrics

- **GitHub Stars:** 100+ by end of month
- **Community:** Discord 50+ members, Telegram 30+ members
- **Blog Traffic:** 1000+ impressions per post
- **Package Downloads:** 1000+/week on pkg.dev
- **Issues:** constructive, actionable feedback
- **Contributions:** first external PR within 2 weeks
- **Social:** 200+ followers on Twitter

---

## Post-Launch (Ongoing)

- Weekly community updates
- Monthly blog posts (tips, tutorials, case studies)
- Regular dependabot updates
- Community showcase: projects using Kruda
- Quarterly releases with new features
- Maintain: docs, examples, backward compatibility
