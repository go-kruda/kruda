# Kruda DI vs The World

Same app, three frameworks. See the difference.

**Scenario:** E-commerce API with multiple services, two databases (read/write),
caching layer, and email notifications.

## Kruda — Built-in, Generics, Zero Codegen

```go
package main

import (
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/middleware"
)

func main() {
    c := kruda.NewContainer()

    // Named instances — two databases, one container
    c.GiveNamed("write", &DB{DSN: "postgres://primary:5432"})
    c.GiveNamed("read", &DB{DSN: "postgres://replica:5432"})

    // Interface binding — swap implementations freely
    c.GiveAs(&RedisCache{Addr: "localhost:6379"}, (*Cache)(nil))

    // Lazy init — only connects when first used
    c.GiveLazy(func() (*Mailer, error) {
        return NewMailer("smtp://mail:587")
    })

    // Transient — new instance per resolve (per request)
    c.GiveTransient(func() (*RequestLogger, error) {
        return &RequestLogger{ID: generateID()}, nil
    })

    // Plain struct
    c.Give(&OrderService{})
    c.Give(&UserService{})

    app := kruda.New(kruda.WithContainer(c))
    app.Use(middleware.Logger())

    // Resolve in handlers — type-safe, one line
    app.Post("/orders", func(c *kruda.Ctx) error {
        orders := kruda.MustResolve[*OrderService](c)
        writeDB := kruda.MustResolveNamed[*DB](c, "write")
        cache := kruda.MustResolve[Cache](c)  // resolves interface
        logger := kruda.MustResolve[*RequestLogger](c) // new per request

        order, err := orders.Create(writeDB, cache, logger)
        if err != nil {
            return err
        }
        return c.Status(201).JSON(order)
    })

    app.Get("/orders", func(c *kruda.Ctx) error {
        orders := kruda.MustResolve[*OrderService](c)
        readDB := kruda.MustResolveNamed[*DB](c, "read")
        return c.JSON(orders.List(readDB))
    })

    app.Listen(":3000")
}
```

**Lines of DI code: ~15**
No codegen. No reflect tags. No `fx.In`/`fx.Out` structs. Just Go.

---

## Gin + Wire (Google) — Codegen Required

```go
// wire.go — you write this
//go:build wireinject

package main

import "github.com/google/wire"

func InitializeApp() (*App, error) {
    wire.Build(
        NewWriteDB,
        NewReadDB,
        NewRedisCache,
        wire.Bind(new(Cache), new(*RedisCache)),
        NewMailer,
        NewOrderService,
        NewUserService,
        NewApp,
    )
    return nil, nil
}

// Then run: wire ./...
// Generates wire_gen.go (100+ lines of boilerplate)
// Every time you add a service, re-run wire.
// Named instances? Need wrapper types:

type WriteDB struct{ *DB }
type ReadDB struct{ *DB }

func NewWriteDB() WriteDB { return WriteDB{&DB{DSN: "primary"}} }
func NewReadDB() ReadDB   { return ReadDB{&DB{DSN: "replica"}} }

// No per-request injection. No lazy init.
// No resolving in handlers — everything wired at startup.
```

**Lines of DI code: ~40+** (plus generated code)
Named instances need wrapper types. No transient scope. No handler-level resolve.

---

## Echo + fx (Uber) — Reflect-based, Verbose

```go
package main

import (
    "go.uber.org/fx"
    "go.uber.org/fx/fxevent"
)

func main() {
    fx.New(
        fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),

        // Named instances — need fx.Out struct
        fx.Provide(fx.Annotate(
            func() *DB { return &DB{DSN: "primary"} },
            fx.ResultTags(`name:"write"`),
        )),
        fx.Provide(fx.Annotate(
            func() *DB { return &DB{DSN: "replica"} },
            fx.ResultTags(`name:"read"`),
        )),

        // Interface binding
        fx.Provide(fx.Annotate(
            NewRedisCache,
            fx.As(new(Cache)),
        )),

        fx.Provide(NewMailer),
        fx.Provide(NewOrderService),
        fx.Provide(NewUserService),

        // To use named deps in a service:
        fx.Provide(fx.Annotate(
            NewOrderService,
            fx.ParamTags(`name:"write"`, `name:"read"`),
        )),

        fx.Invoke(func(app *echo.Echo, orders *OrderService) {
            app.POST("/orders", func(c echo.Context) error {
                // Can't resolve per-request. Everything injected at startup.
                // No transient scope without manual factory pattern.
                return c.JSON(201, orders.Create())
            })
        }),
    ).Run()
}
```

**Lines of DI code: ~35**
String-based tags (`name:"write"`). Runtime errors only. No per-request resolve.
`fx.Annotate` + `fx.ParamTags` + `fx.ResultTags` = tag soup.

---

## Comparison

| Feature | Kruda | Wire (Google) | fx (Uber) |
|---------|:-----:|:------------:|:---------:|
| Built into framework | ✅ | ❌ | ❌ |
| Type-safe (compile time) | ✅ generics | ✅ codegen | ❌ reflect |
| No codegen step | ✅ | ❌ `wire ./...` | ✅ |
| Named instances | ✅ `GiveNamed` | ❌ wrapper types | ⚠️ string tags |
| Interface binding | ✅ `GiveAs` | ✅ `wire.Bind` | ✅ `fx.As` |
| Lazy init | ✅ `GiveLazy` | ❌ | ✅ |
| Per-request (transient) | ✅ `GiveTransient` | ❌ | ❌ |
| Resolve in handler | ✅ `MustResolve[T](c)` | ❌ startup only | ❌ startup only |
| Lifecycle hooks | ✅ OnInit/OnShutdown | ❌ | ✅ OnStart/OnStop |
| Error detection | compile + runtime | compile | runtime only |
| Lines for same app | ~15 | ~40+ | ~35 |

### The Killer Feature

```go
// Only Kruda can do this — resolve different things per request:
app.Get("/orders", func(c *kruda.Ctx) error {
    // Each request gets its own logger with unique ID
    logger := kruda.MustResolve[*RequestLogger](c)  // transient

    // Read from replica
    db := kruda.MustResolveNamed[*DB](c, "read")

    // Interface — swap Redis → Memcached without touching handlers
    cache := kruda.MustResolve[Cache](c)

    return c.JSON(listOrders(db, cache, logger))
})
```

Wire and fx can't do per-request injection without manual factory patterns.
Kruda does it in one line.
