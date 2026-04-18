// Package kruda is a high-performance Go web framework that combines raw
// speed with type-safety through Go generics.
//
// # Quick start
//
//	app := kruda.New()
//	app.Get("/", func(c *kruda.Ctx) error { return c.Text("hello") })
//	app.Listen(":3000")
//
// # Typed handlers
//
// Handlers can declare their input as a struct; kruda binds + validates
// automatically and only calls the handler with a clean, typed input:
//
//	type CreateUser struct {
//	    Name  string `json:"name"  validate:"required,min=2"`
//	    Email string `json:"email" validate:"required,email"`
//	}
//	type User struct{ ID, Name, Email string }
//
//	kruda.Post[CreateUser, User](app, "/users",
//	    func(c *kruda.C[CreateUser]) (*User, error) {
//	        return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
//	    })
//
// # Auto CRUD
//
// Implementing ResourceService[T] gets 5 REST endpoints automatically:
//
//	kruda.Resource[User, string](app, "/users", &UserCRUD{db: db})
//	// → GET /users, GET /users/:id, POST /users, PUT /users/:id, DELETE /users/:id
//
// # Transports
//
// kruda picks a transport automatically based on platform:
//
//   - Linux:   Wing (epoll + eventfd, fastest)
//   - macOS:   fasthttp
//   - Windows: net/http
//
// Override with the [WithTransport], [NetHTTP], or [FastHTTP] options. TLS
// (via [WithTLS] / [WithHTTP3]) auto-falls back to net/http on every
// platform.
//
// Wing is built into core since v1.2.0; the legacy
// import "github.com/go-kruda/kruda/transport/wing" still works as a
// deprecated re-export and will be removed in v2.0.0.
//
// # Dependency injection
//
// An optional, zero-overhead container resolves services by type:
//
//	c := kruda.NewContainer()
//	c.Give(&UserService{})
//
//	app := kruda.New(kruda.WithContainer(c))
//	svc := kruda.MustResolve[*UserService](c)
//
// # Build tags
//
//   - kruda_stdjson: use stdlib encoding/json instead of sonic (slower
//     but no asm; useful for portability and CI matrix builds).
//
// # See also
//
//   - https://pkg.go.dev/github.com/go-kruda/kruda — full API reference
//   - examples/ — 22 runnable examples covering every feature
package kruda
