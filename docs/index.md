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

features:
  - icon: "\U0001F512"
    title: Typed Handlers C[T]
    details: Body, params, and query parsed into one struct. Validated at compile time. No manual binding.
  - icon: "\U000026A1"
    title: Auto CRUD
    details: Implement ResourceService[T] and get 5 REST endpoints. One line of code.
  - icon: "\U0001F680"
    title: Wing Transport
    details: Custom epoll+eventfd on Linux, kqueue on macOS. 846K req/s plaintext — beats Fiber by 26%.
  - icon: "\U0001F4E6"
    title: Built-in DI
    details: Optional dependency injection with Go generics. No codegen, no reflection.
  - icon: "\U0001F916"
    title: AI-Friendly
    details: Built-in MCP server for AI coding assistants. Typed API means AI generates correct code on first try.
  - icon: "\U0001F6E1"
    title: Minimal Dependencies
    details: Sonic JSON (opt-out via build tag). 10 contrib modules. Pluggable transport.
---
