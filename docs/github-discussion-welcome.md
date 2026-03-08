# 👋 Welcome to Kruda Discussions!

สวัสดีครับ/ค่ะ! Welcome to the Kruda community.

This is the place to ask questions, share ideas, and show off what you've built with Kruda.

## Getting Started

- 📖 [README](https://github.com/go-kruda/kruda#readme) — Quick Start, features, benchmarks
- 📂 [Examples](https://github.com/go-kruda/kruda/tree/main/examples) — 21 copy-paste-able examples (hello, typed-handlers, auto-crud, middleware, DI, and more)
- 🤝 [Contributing Guide](https://github.com/go-kruda/kruda/blob/main/CONTRIBUTING.md) — Dev setup, code style, PR process
- 📦 [pkg.go.dev](https://pkg.go.dev/github.com/go-kruda/kruda) — API reference
- 📋 [CHANGELOG](https://github.com/go-kruda/kruda/blob/main/CHANGELOG.md) — Release history

## Discussion Categories

| Category | Use for |
|----------|---------|
| 💬 General | Anything Kruda-related |
| ❓ Help | Questions, troubleshooting, "how do I..." |
| 💡 Ideas | Feature requests, API design suggestions |
| 🙌 Show & Tell | Share your projects, benchmarks, blog posts |

## Quick Install

```bash
go get github.com/go-kruda/kruda
```

```go
package main

import "github.com/go-kruda/kruda"

func main() {
    app := kruda.New()
    app.Get("/", func(c *kruda.Ctx) error {
        return c.Text("Hello, Kruda!")
    })
    app.Listen(":3000")
}
```

## Tips

- AI tools (Claude, Cursor, etc.) can convert your existing Gin/Fiber/Echo code to Kruda — just paste and ask "Convert this to Kruda"
- Check the [concept mapping table](https://github.com/go-kruda/kruda#coming-from-another-framework) in the README if you're coming from another framework
- All examples work with `go run .` — no extra setup needed

Happy coding! 🚀
