# Routing

Kruda uses a radix tree router with O(1) child lookup for fast route matching.

## Route Registration

Register routes with HTTP method helpers:

```go
app := kruda.New()

app.Get("/users", listUsers)
app.Post("/users", createUser)
app.Put("/users/:id", updateUser)
app.Delete("/users/:id", deleteUser)
app.Patch("/users/:id", patchUser)
```

## Route Parameters

Use `:name` for named parameters and `*name` for wildcard (catch-all) parameters:

```go
// Named parameter — matches /users/123, /users/abc
app.Get("/users/:id", func(c *kruda.Ctx) error {
    id := c.Param("id") // "123"
    return c.JSON(200, map[string]string{"id": id})
})

// Wildcard parameter — matches /files/path/to/file.txt
app.Get("/files/*path", func(c *kruda.Ctx) error {
    path := c.Param("path") // "path/to/file.txt"
    return c.JSON(200, map[string]string{"path": path})
})
```

## Route Groups

Group routes under a common prefix with shared middleware:

```go
api := app.Group("/api/v1")

api.Get("/users", listUsers)
api.Post("/users", createUser)

// Nested groups
admin := api.Group("/admin")
admin.Get("/stats", getStats)
// Matches: GET /api/v1/admin/stats
```

## Group Middleware

Apply middleware to all routes in a group:

```go
api := app.Group("/api", authMiddleware)

api.Get("/profile", getProfile)   // requires auth
api.Get("/settings", getSettings) // requires auth
```

## Route Priority

Routes are matched in registration order with the following priority:

1. Static segments (`/users/list`)
2. Named parameters (`/users/:id`)
3. Wildcard parameters (`/users/*path`)

```go
app.Get("/users/list", listUsers)   // matched first for /users/list
app.Get("/users/:id", getUser)      // matched for /users/123
app.Get("/users/*path", catchAll)   // matched for /users/a/b/c
```

## Path Normalization

The router automatically normalizes request paths:

- Trailing slashes are handled consistently
- `.` and `..` segments are resolved
- Path traversal attempts (`/../`) are rejected with HTTP 400
