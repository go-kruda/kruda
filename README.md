# Kruda (ครุฑ)

A high-performance Go web framework combining speed with type-safety through Go generics.

## Installation

```bash
go get github.com/go-kruda/kruda
```

## Quick Start

```go
package main

import (
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/middleware"
)

func main() {
    app := kruda.New()

    app.Use(middleware.Recovery())
    app.Use(middleware.Logger())

    app.Get("/ping", func(c *kruda.Ctx) error {
        return c.JSON(kruda.Map{"pong": true})
    })

    app.Listen(":3000")
}
```

## Features

- **Radix tree router** — O(1) static route lookup with parameterized, wildcard, regex, and optional patterns
- **Middleware chain** — Pre-built at registration time for zero-allocation request handling
- **Route groups** — Nested groups with scoped middleware and prefix routing
- **Lifecycle hooks** — OnRequest, BeforeHandle, AfterHandle, OnResponse, OnError, OnShutdown
- **Typed handlers** — Generic `C[T]` context with auto-parse via Go generics
- **Built-in middleware** — Logger, Recovery, CORS, RequestID, Timeout
- **Graceful shutdown** — Signal handling with configurable drain timeout
- **Zero external dependencies** — Built entirely on Go standard library

## License

[MIT](LICENSE)
