// Flat pattern — everything in one file.
// Best for: prototypes, small APIs, scripts, hackathons.
package main

import (
	"fmt"
	"sync"

	"github.com/go-kruda/kruda"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var (
	mu    sync.RWMutex
	users = map[string]*User{}
	seq   int
)

func main() {
	app := kruda.New()

	app.Get("/users", func(c *kruda.Ctx) error {
		mu.RLock()
		list := make([]*User, 0, len(users))
		for _, u := range users {
			list = append(list, u)
		}
		mu.RUnlock()
		return c.JSON(list)
	})

	app.Get("/users/:id", func(c *kruda.Ctx) error {
		mu.RLock()
		u, ok := users[c.Param("id")]
		mu.RUnlock()
		if !ok {
			return c.Status(404).JSON(kruda.Map{"error": "not found"})
		}
		return c.JSON(u)
	})

	app.Post("/users", func(c *kruda.Ctx) error {
		var u User
		if err := c.Bind(&u); err != nil {
			return c.Status(400).JSON(kruda.Map{"error": err.Error()})
		}
		mu.Lock()
		seq++
		u.ID = fmt.Sprintf("%d", seq)
		users[u.ID] = &u
		mu.Unlock()
		return c.Status(201).JSON(u)
	})

	app.Put("/users/:id", func(c *kruda.Ctx) error {
		mu.Lock()
		defer mu.Unlock()
		u, ok := users[c.Param("id")]
		if !ok {
			return c.Status(404).JSON(kruda.Map{"error": "not found"})
		}
		var body User
		if err := c.Bind(&body); err != nil {
			return c.Status(400).JSON(kruda.Map{"error": err.Error()})
		}
		u.Name = body.Name
		u.Email = body.Email
		return c.JSON(u)
	})

	app.Delete("/users/:id", func(c *kruda.Ctx) error {
		mu.Lock()
		defer mu.Unlock()
		if _, ok := users[c.Param("id")]; !ok {
			return c.Status(404).JSON(kruda.Map{"error": "not found"})
		}
		delete(users, c.Param("id"))
		return c.NoContent()
	})

	app.Listen(":3000")
}
