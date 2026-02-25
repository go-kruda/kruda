# Container

The DI container provides dependency injection using Go generics. No codegen, no runtime reflection.

## Give

```go
func Give[T any](app *App, factory func() T)
```

Registers a service factory in the container. The factory is called lazily on first resolution.

```go
kruda.Give(app, func() *UserService {
    return &UserService{db: connectDB()}
})
```

## Use

```go
func Use[T any](c *Ctx) T
```

Resolves a service from the container. Services are singleton by default — the factory runs once and the result is cached.

```go
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.Use[*UserService](c)
    users, _ := svc.ListAll()
    return c.JSON(200, users)
})
```

## Container Lifecycle

Services follow a singleton lifecycle:

1. `Give[T]` registers a factory function
2. First `Use[T]` call invokes the factory and caches the result
3. Subsequent `Use[T]` calls return the cached instance

```go
kruda.Give(app, func() *DBPool {
    fmt.Println("creating pool") // printed once
    return NewDBPool()
})

// Both handlers share the same *DBPool instance
app.Get("/a", func(c *kruda.Ctx) error {
    pool := kruda.Use[*DBPool](c)
    return c.String(200, "ok")
})
app.Get("/b", func(c *kruda.Ctx) error {
    pool := kruda.Use[*DBPool](c)
    return c.String(200, "ok")
})
```

## Module Interface

```go
type Module interface {
    Register(app *App)
}
```

Group related service registrations into a module:

```go
type UserModule struct{}

func (m *UserModule) Register(app *kruda.App) {
    kruda.Give(app, NewUserRepository)
    kruda.Give(app, NewUserService)
}

app.Module(&UserModule{})
```

## App.Module

```go
func (a *App) Module(m Module) *App
```

Registers a module, calling its `Register` method.

```go
app.Module(&UserModule{})
app.Module(&OrderModule{})
```

## Example

```go
package main

import "github.com/go-kruda/kruda"

type Config struct {
    DBUrl string
}

type DB struct {
    url string
}

func main() {
    app := kruda.New()

    kruda.Give(app, func() *Config {
        return &Config{DBUrl: "postgres://localhost/mydb"}
    })

    kruda.Give(app, func() *DB {
        return &DB{url: "postgres://localhost/mydb"}
    })

    app.Get("/health", func(c *kruda.Ctx) error {
        db := kruda.Use[*DB](c)
        return c.JSON(200, map[string]string{"db": db.url})
    })

    app.Listen(":3000")
}
```
