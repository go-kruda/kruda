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

// Stub handlers used only so the Example* functions compile.
func listUsersHandler(c *kruda.Ctx) error  { _ = c; return nil }
func createUserHandler(c *kruda.Ctx) error { _ = c; return nil }
