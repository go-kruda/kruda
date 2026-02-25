# DI Container

Kruda includes a built-in dependency injection container using the Give/Use pattern. No codegen, no reflection at runtime.

## Registering Services

Use `kruda.Give` to register a service:

```go
app := kruda.New()

// Register a service
kruda.Give(app, func() *UserService {
    return &UserService{
        db: connectDB(),
    }
})
```

## Resolving Services

Use `kruda.Use` to resolve a service in handlers:

```go
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.Use[*UserService](c)
    users, err := svc.ListAll()
    if err != nil {
        return err
    }
    return c.JSON(200, users)
})
```

## Service Lifecycle

Services are created lazily on first resolution and cached for the app lifetime (singleton by default):

```go
// This factory runs once, on first Use[*DBPool] call
kruda.Give(app, func() *DBPool {
    pool, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    return &DBPool{pool}
})
```

## Modules

Group related services into modules for better organization:

```go
type UserModule struct{}

func (m *UserModule) Register(app *kruda.App) {
    kruda.Give(app, NewUserRepository)
    kruda.Give(app, NewUserService)
}

// Register the module
app.Module(&UserModule{})
```

## Dependency Chains

Services can depend on other services:

```go
kruda.Give(app, func() *DBPool {
    return NewDBPool()
})

kruda.Give(app, func() *UserRepository {
    // Resolved from container automatically
    return &UserRepository{}
})

kruda.Give(app, func() *UserService {
    return &UserService{}
})
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
    app := kruda.New()

    kruda.Give(app, func() *GreetService {
        return &GreetService{prefix: "Hello"}
    })

    app.Get("/greet/:name", func(c *kruda.Ctx) error {
        svc := kruda.Use[*GreetService](c)
        msg := svc.Greet(c.Param("name"))
        return c.JSON(200, map[string]string{"message": msg})
    })

    app.Listen(":3000")
}
```

See [Container API](/api/container) for the full reference.
