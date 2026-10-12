::: code-group

```go [Typed handler]
package main

import "github.com/go-kruda/kruda"

type CreateUser struct {
	Name  string `json:"name"  validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
}

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	app := kruda.New()

	kruda.Post[CreateUser, User](app, "/users",
		func(c *kruda.C[CreateUser]) (*User, error) {
			u := c.In
			return &User{ID: "1", Name: u.Name, Email: u.Email}, nil
		})

	app.Listen(":3000")
}
```

```go [Auto CRUD]
package main

import (
	"context"

	"github.com/go-kruda/kruda"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Users implements kruda.ResourceService[User, string].
type Users struct{ db map[string]User }

func (s *Users) List(ctx context.Context, page, limit int) ([]User, int, error) {
	out := make([]User, 0, len(s.db))
	for _, u := range s.db {
		out = append(out, u)
	}
	return out, len(out), nil
}

func (s *Users) Get(ctx context.Context, id string) (User, error) {
	return s.db[id], nil
}

func (s *Users) Create(ctx context.Context, u User) (User, error) {
	s.db[u.ID] = u
	return u, nil
}

func (s *Users) Update(ctx context.Context, id string, u User) (User, error) {
	s.db[id] = u
	return u, nil
}

func (s *Users) Delete(ctx context.Context, id string) error {
	delete(s.db, id)
	return nil
}

func main() {
	app := kruda.New()

	// GET /users, POST /users, GET/PUT/DELETE /users/:id
	kruda.Resource[User, string](app, "/users", &Users{db: map[string]User{}})

	app.Listen(":3000")
}
```

```go [Presets]
package main

import "github.com/go-kruda/kruda"

func main() {
	app := kruda.New()

	app.Get("/", func(c *kruda.Ctx) error {
		return c.Text("Hello, Kruda!")
	}, kruda.Plaintext)

	app.Get("/users/:id", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"id": c.Param("id")})
	}, kruda.JSON)

	app.Listen(":3000")
}
```

:::
