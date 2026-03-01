# DI Container

Kruda includes a built-in dependency injection container. Services are registered on a `*Container` and resolved in handlers via `*Ctx`.

## Creating a Container

```go
c := kruda.NewContainer()
app := kruda.New(kruda.WithContainer(c))
```

Or let modules create it automatically:

```go
app := kruda.New()
app.Module(&MyModule{}) // creates container if needed
```

## Registering Services

### Give — Singleton

```go
func (c *Container) Give(instance any) error
```

Registers a pre-built singleton instance:

```go
c := kruda.NewContainer()
c.Give(&UserService{db: connectDB()})
```

### GiveLazy — Lazy Singleton

```go
func (c *Container) GiveLazy(factory any) error
```

Registers a factory that runs once on first resolution:

```go
c.GiveLazy(func() (*DBPool, error) {
    return sql.Open("postgres", os.Getenv("DATABASE_URL"))
})
```

### GiveTransient — New Instance Per Resolution

```go
func (c *Container) GiveTransient(factory any) error
```

Registers a factory that creates a new instance every time:

```go
c.GiveTransient(func() (*RequestLogger, error) {
    return &RequestLogger{}, nil
})
```

### GiveNamed — Named Instance

```go
func (c *Container) GiveNamed(name string, instance any) error
```

Registers a named instance:

```go
c.GiveNamed("primary", &DB{DSN: "primary-dsn"})
c.GiveNamed("replica", &DB{DSN: "replica-dsn"})
```

### GiveAs — Interface Registration

```go
func (c *Container) GiveAs(instance any, ifacePtr any) error
```

Registers an instance under an interface type:

```go
c.GiveAs(&PostgresRepo{}, (*UserRepository)(nil))
```

## Resolving in Handlers

Use package-level generic functions with `*Ctx`:

### Resolve

```go
func Resolve[T any](c *Ctx) (T, error)
```

```go
app.Get("/users", func(c *kruda.Ctx) error {
    svc, err := kruda.Resolve[*UserService](c)
    if err != nil {
        return err
    }
    return c.JSON(svc.ListAll())
})
```

### MustResolve

```go
func MustResolve[T any](c *Ctx) T
```

Like `Resolve` but panics on error (caught by Recovery middleware):

```go
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.MustResolve[*UserService](c)
    return c.JSON(svc.ListAll())
})
```

### ResolveNamed / MustResolveNamed

```go
func ResolveNamed[T any](c *Ctx, name string) (T, error)
func MustResolveNamed[T any](c *Ctx, name string) T
```

```go
primary := kruda.MustResolveNamed[*DB](c, "primary")
replica := kruda.MustResolveNamed[*DB](c, "replica")
```

## Low-Level Container Resolution

For use outside of handlers (e.g., in tests or setup):

```go
func Use[T any](c *Container) (T, error)
func UseNamed[T any](c *Container, name string) (T, error)
```

## Modules

Group related service registrations into modules:

```go
type Module interface {
    Install(c *Container) error
}
```

```go
type UserModule struct{}

func (m *UserModule) Install(c *kruda.Container) error {
    c.Give(&UserRepository{})
    c.Give(&UserService{})
    return nil
}

app.Module(&UserModule{})
```

## Lifecycle

Services implementing `Initializer` or `Shutdowner` are managed automatically:

```go
type Initializer interface {
    Init(ctx context.Context) error
}

type Shutdowner interface {
    Shutdown(ctx context.Context) error
}
```

```go
func (db *DBPool) Init(ctx context.Context) error {
    return db.Ping(ctx)
}

func (db *DBPool) Shutdown(ctx context.Context) error {
    return db.Close()
}
```

## InjectMiddleware

```go
func (c *Container) InjectMiddleware() HandlerFunc
```

Returns middleware that makes the container available to handlers for `Resolve`/`MustResolve`:

```go
app.Use(container.InjectMiddleware())
```

This is set up automatically when using `WithContainer(c)`.

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
