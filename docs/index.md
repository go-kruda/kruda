---
layout: home

hero:
  name: Kruda<span class="kruda-feather" aria-hidden="true"></span>
  text: Fast by default, <span class="nowrap">type-safe</span> by design
  tagline: High-performance Go web framework with typed handlers, auto CRUD, and custom async I/O transport<span class="kruda-origin">Named after <span lang="th">ครุฑ</span>, the Garuda of Thai mythology.</span>
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/go-kruda/kruda
  image:
    src: /hero/mascot.webp
    alt: Kruda Garuda mascot carrying Go gopher

features:
  - title: Typed Handlers C[T]
    details: Compiler-checked handler types. Body, params, and query are bound and validated before execution.
    link: /guide/handlers
  - title: Auto CRUD
    details: Implement ResourceService[T] and get 5 REST endpoints. One line of code.
    link: /guide/auto-crud
  - title: Wing Transport
    details: Custom epoll+eventfd on Linux and kqueue on macOS, backed by versioned reproducible benchmarks.
    link: /guide/transport
  - title: Built-in DI
    details: Optional dependency injection with Go generics. No codegen, no reflection.
    link: /guide/di-container
  - title: AI-Friendly
    details: Built-in MCP documentation and runnable examples for coding assistants.
    link: /guide/ai-integration
  - title: Minimal Dependencies
    details: Sonic JSON (opt-out via build tag). 11 contrib modules. Pluggable transport.
    link: /guide/contrib
---
