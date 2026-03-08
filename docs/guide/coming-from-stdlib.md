# Coming from stdlib (net/http)

If you've been writing Go web services with `net/http` and `http.ServeMux`, Kruda will feel familiar — it builds on the same patterns but removes the boilerplate.

## Quick Comparison

| Concept | stdlib | Kruda |
|---------|--------|-------|
| Create server | `http.NewServeMux()` | `kruda.New()` |
| Route | `mux.HandleFunc("GET /path", h)` | `app.Get("/path", h)` |
| Handler sig | `func(w http.ResponseWriter, r *http.Request)` | `func(c *kruda.Ctx) error` |
| JSON response | `json.NewEncoder(w).Encode(obj)` | `c.JSON(obj)` |
| Status code | `w.WriteHeader(404)` | `c.Status(404).JSON(obj)` |
| Path param | `r.PathValue("id")` | `c.Param("id")` |
| Query param | `r.URL.Query().Get("q")` | `c.Query("q")` |
| Body parse | `json.NewDecoder(r.Body).Decode(&v)` | `c.Bind(&v)` or `C[T]` |
| Middleware | manual wrapping | `app.Use(mw)` |
| Route group | manual prefix | `app.Group("/api")` |
| Start | `http.ListenAndServe(":3000", mux)` | `app.Listen(":3000")` |

## Hello World

**stdlib:**
```go
package main

import (
	"encoding/json"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "hello",
		})
	})
	http.ListenAndServe(":3000", mux)
}
```

**Kruda:**
```go
package main

import "github.com/go-kruda/kruda"

func main() {
	app := kruda.New()
	app.Get("/", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"message": "hello"})
	})
	app.Listen(":3000")
}
```

No manual `Content-Type` header. No `json.NewEncoder`. Handlers return `error` instead of silently failing.

## Key Differences

### 1. Handler Signature

stdlib handlers take two args and return nothing — errors are easy to forget:

```go
// stdlib — must remember to return after writing error
func handler(w http.ResponseWriter, r *http.Request) {
	user, err := findUser(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return // forget this and you write twice
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
```

Kruda handlers return `error` — the compiler helps you:

```go
// Kruda — error return enforces handling
func handler(c *kruda.Ctx) error {
	user, err := findUser(c.Param("id"))
	if err != nil {
		return err // centralized error handler formats the response
	}
	return c.JSON(user)
}
```

### 2. JSON Body Parsing

**stdlib:**
```go
type CreateUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
	var input CreateUser
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	// manual validation...
	if input.Name == "" {
		http.Error(w, "name required", 422)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(createUser(input))
})
```

**Kruda:**
```go
type CreateUser struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

app.Post("/users", func(c *kruda.Ctx) error {
	var input CreateUser
	if err := c.Bind(&input); err != nil {
		return err // auto 422 with structured validation errors
	}
	return c.Status(201).JSON(createUser(input))
})
// Or use typed handlers for compile-time safety:
// kruda.Post[CreateUser, User](app, "/users", handler)
```

No manual `json.NewDecoder`. No manual validation. Validation errors return structured 422 automatically.

### 3. Middleware

stdlib requires manual function wrapping:

```go
// stdlib — nested wrapping, hard to read
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

http.ListenAndServe(":3000", logging(cors(mux)))
```

Kruda has a built-in middleware chain:

```go
import "github.com/go-kruda/kruda/middleware"

app.Use(middleware.Logger())
app.Use(middleware.CORS())
app.Use(middleware.Recovery())
app.Use(middleware.RequestID())
```

### 4. Route Groups

stdlib has no built-in grouping — you manage prefixes manually:

```go
// stdlib — manual prefix management
mux.HandleFunc("GET /api/v1/users", listUsers)
mux.HandleFunc("POST /api/v1/users", createUser)
mux.HandleFunc("GET /api/v1/users/{id}", getUser)
// auth middleware? wrap each handler individually or the whole mux
```

Kruda supports groups with scoped middleware:

```go
app.Group("/api/v1").
	Guard(authMiddleware).
	Get("/users", listUsers).
	Post("/users", createUser).
	Get("/users/:id", getUser).
	Done()
```

### 5. Pluggable Transport

stdlib is locked to `net/http`. Kruda auto-selects the fastest transport for your platform:

```go
// Wing (default on Linux) — epoll+eventfd, 846K req/s
app := kruda.New()

// net/http — same engine as stdlib, with Kruda's ergonomics
app := kruda.New(kruda.NetHTTP())

// fasthttp — broad compatibility
app := kruda.New(kruda.FastHTTP())
```

You can use `kruda.NetHTTP()` to keep the exact same transport as stdlib while gaining Kruda's API.

## What You Gain

- **846K req/s** — Wing transport (epoll+eventfd) by default on Linux
- **Error-returning handlers** — no more silent failures or forgotten `return`
- **Type-safe handlers** — `C[T]` with auto-bind, auto-validate, auto-OpenAPI
- **Built-in middleware** — Logger, CORS, Recovery, RequestID, RateLimit
- **Route groups** — with scoped middleware via `Guard()`
- **Auto CRUD** — `kruda.Resource[Product, string](app, "/products", service)` generates full REST
- **Built-in DI** — optional dependency injection without codegen
- **Same transport option** — `kruda.NetHTTP()` uses `net/http` under the hood if you prefer
