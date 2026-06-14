package kruda_test

import (
	"github.com/go-kruda/kruda"
)

// ExampleNew shows the smallest possible Kruda app.
func ExampleNew() {
	app := kruda.New()
	app.Get("/ping", func(c *kruda.Ctx) error {
		return c.Text("pong")
	})
	_ = app
	// Output:
}

// ExamplePost shows a typed handler that binds and validates the request
// body before the handler is called.
func ExamplePost() {
	type CreateUser struct {
		Name string `json:"name" validate:"required"`
	}
	type User struct {
		ID, Name string
	}

	app := kruda.New()
	kruda.Post[CreateUser, User](app, "/users",
		func(c *kruda.C[CreateUser]) (*User, error) {
			return &User{ID: "1", Name: c.In.Name}, nil
		})
	_ = app
	// Output:
}

// ExampleApp_Use shows global middleware registration.
func ExampleApp_Use() {
	app := kruda.New()
	app.Use(func(c *kruda.Ctx) error {
		// e.g. attach a request ID, then continue
		return c.Next()
	})
	app.Get("/", func(c *kruda.Ctx) error { return c.Text("ok") })
	_ = app
	// Output:
}

// ExampleApp_Group shows a route group with a shared prefix.
func ExampleApp_Group() {
	app := kruda.New()
	api := app.Group("/api/v1")
	api.Get("/users", listUsersHandler)
	api.Post("/users", createUserHandler)
	_ = api
	// Output:
}

// ExamplePreset shows the per-route Preset options. A Preset is itself a
// RouteOption, so it is passed straight to route registration. Semantic
// presets (Plaintext, JSON, DB, Render) name the workload; structural presets
// (Bolt, Arrow, Spear) name the dispatch and can be tuned with With.
func ExamplePreset() {
	app := kruda.New()
	h := func(c *kruda.Ctx) error { return c.Text("ok") }

	// Semantic presets — pick by what the route does:
	app.Get("/health", h, kruda.Plaintext) // static text / health check
	app.Get("/user", h, kruda.JSON)        // JSON, no I/O
	app.Get("/db", h, kruda.DB)            // short DB / Redis lookup
	app.Get("/page", h, kruda.Render)      // DB + HTML/template render

	// Structural presets name the dispatch; tune with With:
	app.Get("/heavy", h, kruda.Spear)
	app.Get("/tuned", h, kruda.Bolt.With(kruda.Dispatch(kruda.Pool)))

	// StaticText/StaticJSON opt into Wing's handler bypass for hot paths:
	app.Get("/version", h, kruda.StaticJSON(200, `{"version":"1.3.0"}`))

	_ = app
	// Output:
}

// ExampleWithProblemJSON shows opt-in RFC 9457 problem+json error responses.
func ExampleWithProblemJSON() {
	app := kruda.New(kruda.WithProblemJSON())
	app.Get("/users/:id", func(c *kruda.Ctx) error {
		return kruda.NotFound("user not found").
			WithType("https://errors.example.com/not-found").
			With("userId", c.Param("id"))
	})
	_ = app
	// Output:
}

// Stub handlers used only so the Example* functions compile.
func listUsersHandler(c *kruda.Ctx) error  { _ = c; return nil }
func createUserHandler(c *kruda.Ctx) error { _ = c; return nil }
