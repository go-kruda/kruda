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
// (via [WithTLS]) auto-falls back to net/http on every platform.
//
// Wing is built into core since v1.2.0; import github.com/go-kruda/kruda
// directly and use the kruda.Wing option. The legacy
// github.com/go-kruda/kruda/transport/wing shim was removed in v1.3.0 —
// consumers pinned to v1.2.x tags are unaffected until they upgrade.
//
// # Wing route presets (advanced)
//
// Wing schedules each route's handler according to a per-route [Preset].
// The defaults work for typical workloads; tune only when profiling says so.
//
//   - [Bolt] (default) — handler runs inline in the event loop. Zero overhead;
//     best for plaintext, JSON, and health checks where latency dominates and
//     there is no I/O wait. Semantic aliases: [Plaintext], [JSON].
//   - [Arrow] — handler dispatched to a bounded goroutine pool (~1µs overhead).
//     Use for short I/O like a DB query or Redis lookup.
//   - [Spear] — a goroutine takes over the connection with blocking syscalls,
//     letting the Go runtime spawn extra OS threads for heavy I/O. Use for
//     DB-heavy endpoints or template rendering. Semantic aliases: [DB], [Render].
//
// Presets are route options — pass them directly at registration:
//
//	app.Get("/ping",       handler, kruda.Plaintext) // inline
//	app.Get("/api/json",   handler, kruda.JSON)      // inline
//	app.Get("/users/:id",  handler, kruda.DB)        // takeover
//	app.Get("/render/:id", handler, kruda.Render)    // takeover
//
// If a route left on inline dispatch repeatedly blocks the event loop,
// Kruda logs a one-time warning per route suggesting kruda.DB or
// kruda.Spear. No dispatch mode is ever switched automatically.
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
