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
// Wing is built into core since v1.2.0. New code should import
// github.com/go-kruda/kruda directly and use the kruda.Wing option/helpers.
// The legacy github.com/go-kruda/kruda/transport/wing import path remains a
// compatibility shim for older v1.x consumers.
//
// # Wing dispatch tuning (advanced)
//
// Wing's per-route dispatch mode controls how each handler is scheduled.
// The defaults work for typical workloads; tune only when profiling says so.
//
//   - [Bolt] (default) — handler runs inline in the event loop. Zero overhead;
//     best for plaintext, JSON, and health checks where latency dominates and
//     there is no I/O wait.
//   - [Arrow] — handler dispatched to a bounded goroutine pool (~1µs overhead).
//     Use for short I/O like a DB query or Redis lookup.
//   - [Spear] — a goroutine takes over the connection with blocking syscalls,
//     letting the Go runtime spawn extra OS threads for heavy I/O. Use for
//     DB-heavy endpoints or template rendering.
//
// Apply per route with the matching helper:
//
//	app.Get("/ping",       handler, kruda.WingPlaintext()) // Bolt
//	app.Get("/api/json",   handler, kruda.WingJSON())      // Bolt
//	app.Get("/users/:id",  handler, kruda.WingQuery())     // Spear
//	app.Get("/render/:id", handler, kruda.WingRender())    // Spear
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
