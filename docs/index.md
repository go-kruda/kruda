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
    details: Body, params, and query parsed into one struct. Validated at compile time. No manual binding.
  - icon: ⚡
    title: Auto CRUD
    details: Implement ResourceService[T] and get 5 REST endpoints. One line of code.
  - icon: 🚀
    title: Wing Transport
    details: Custom epoll+eventfd on Linux, kqueue on macOS. 846K req/s plaintext — beats Fiber by 26%.
  - icon: 📦
    title: Built-in DI
    details: Optional dependency injection with Go generics. No codegen, no reflection.
  - icon: 🤖
    title: AI-Friendly
    details: Built-in MCP server for AI coding assistants. Typed API means AI generates correct code on first try.
  - icon: 🛡
    title: Minimal Dependencies
    details: Sonic JSON (opt-out via build tag). 10 contrib modules. Pluggable transport.
---
