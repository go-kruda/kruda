# DI Container

Kruda includes a built-in dependency injection container. Services are registered on a `*Container` and resolved in handlers via `*Ctx`.

## Setting Up

```go
c := kruda.NewContainer()
c.Give(&UserService{db: connectDB()})
c.GiveLazy(func() (*DBPool, error) { return connectDB() })
c.GiveNamed("write", &DB{DSN: "primary"})

app := kruda.New(kruda.WithContainer(c))
```

## Resolving in Handlers

```go
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.MustResolve[*UserService](c)
    return c.JSON(svc.ListAll())
})
```

## Registration Methods

All registration is done on `*Container`:

| Method | Behavior |
|--------|----------|
| `Give(instance)` | Singleton — pre-built instance |
| `GiveLazy(factory)` | Lazy singleton — factory runs once on first use |
| `GiveTransient(factory)` | New instance per resolution |
| `GiveNamed(name, instance)` | Named singleton |
| `GiveAs(instance, ifacePtr)` | Register under interface type |

## Resolution Functions

In handlers, use package-level generic functions with `*Ctx`:

| Function | Behavior |
|----------|----------|
| `Resolve[T](c)` | Returns `(T, error)` |
| `MustResolve[T](c)` | Returns `T`, panics on error |
| `ResolveNamed[T](c, name)` | Named resolution with error |
| `MustResolveNamed[T](c, name)` | Named resolution, panics on error |

## Modules

Group related services into modules:

```go
type UserModule struct{}

func (m *UserModule) Install(c *kruda.Container) error {
    c.Give(&UserRepository{})
    c.Give(&UserService{})
    return nil
}

app.Module(&UserModule{})
```

The `Module` interface:

```go
type Module interface {
    Install(c *Container) error
}
```

## Dependency Chains

```go
c := kruda.NewContainer()
c.Give(&DBPool{dsn: os.Getenv("DATABASE_URL")})
c.Give(&UserRepository{})
c.Give(&UserService{})

app := kruda.New(kruda.WithContainer(c))
```

## Example

```go
package main

import "github.com/go-kruda/kruda"

type GreetService struct {
    prefix string
}

func (s *GreetService) Greet(name string) string {
    return s.prefix + " " + name
}

func main() {
    c := kruda.NewContainer()
    c.Give(&GreetService{prefix: "Hello"})

    app := kruda.New(kruda.WithContainer(c))

    app.Get("/greet/:name", func(c *kruda.Ctx) error {
        svc := kruda.MustResolve[*GreetService](c)
        msg := svc.Greet(c.Param("name"))
        return c.JSON(map[string]string{"message": msg})
    })

    app.Listen(":3000")
}
```

See [Container API](/api/container) for the full reference.
