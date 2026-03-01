---
layout: home

hero:
  name: Kruda
  text: Type-safe Go Web Framework
  tagline: Auto-everything — typed handlers, validation, OpenAPI, CRUD, DI. Write less, ship faster.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/go-kruda/kruda

features:
  - icon: 🔒
    title: Type-Safe Handlers
    details: Generic typed handlers C[T] with compile-time checks. Body, params, and query parsed into a single struct — zero reflection at runtime.
  - icon: ⚡
    title: Auto-Everything
    details: Auto-validation, auto-error-mapping, auto-OpenAPI 3.1, auto-CRUD. 60-70% less boilerplate than Gin or Fiber.
  - icon: 🚀
    title: Blazing Fast
    details: Pluggable transport with fasthttp and net/http. Zero-alloc hot paths, pooled contexts, radix tree router.
  - icon: 📦
    title: Built-in DI
    details: Optional dependency injection with Give/Use pattern. No codegen, no reflection. Modules, lifecycle management, and auto-wiring.
---

## Quick Start

```go
package main

import "github.com/go-kruda/kruda"

func main() {
    app := kruda.New()

    app.Get("/", func(c *kruda.Ctx) error {
        return c.Status(200).JSON(map[string]string{"message": "Hello, Kruda!"})
    })

    app.Listen(":3000")
}
```

```bash
go get github.com/go-kruda/kruda
go run main.go
```
