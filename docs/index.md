---
layout: home

hero:
  name: Kruda
  text: Fast by default, type-safe by design
  tagline: High-performance Go web framework with typed handlers, auto CRUD, and custom async I/O transport
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/go-kruda/kruda
  image:
    src: /kruda-hero.jpg
    alt: Kruda Garuda mascot carrying Go gopher

features:
  - icon: 🔒
    title: Typed Handlers C[T]
    details: Compiler-checked handler types. Body, params, and query are bound and validated before execution.
  - icon: ⚡
    title: Auto CRUD
    details: Implement ResourceService[T] and get 5 REST endpoints. One line of code.
  - icon: 🚀
    title: Wing Transport
    details: Custom epoll+eventfd on Linux and kqueue on macOS, backed by versioned reproducible benchmarks.
  - icon: 📦
    title: Built-in DI
    details: Optional dependency injection with Go generics. No codegen, no reflection.
  - icon: 🤖
    title: AI-Friendly
    details: Built-in MCP documentation and runnable examples for coding assistants.
  - icon: 🛡
    title: Minimal Dependencies
    details: Sonic JSON (opt-out via build tag). 11 contrib modules. Pluggable transport.
---
