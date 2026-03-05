package main

import (
	"fmt"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/contrib/session"
)

func main() {
	// Session and SSE require net/http transport (Wing doesn't support
	// Set-Cookie headers or HTTP flushing). See docs/guide/transport.md.
	app := kruda.New(kruda.NetHTTP())

	// Session middleware with in-memory store
	app.Use(session.New())

	app.Post("/login", func(c *kruda.Ctx) error {
		sess := session.GetSession(c)
		sess.Set("user", "Tiger")
		sess.Set("role", "admin")
		return c.JSON(kruda.Map{"message": "logged in", "session_id": sess.ID()})
	})

	app.Get("/profile", func(c *kruda.Ctx) error {
		sess := session.GetSession(c)
		user := sess.GetString("user")
		if user == "" {
			return c.Status(401).JSON(kruda.Map{"error": "not logged in"})
		}
		return c.JSON(kruda.Map{
			"user": user,
			"role": sess.GetString("role"),
			"new":  sess.IsNew(),
		})
	})

	app.Post("/logout", func(c *kruda.Ctx) error {
		sess := session.GetSession(c)
		sess.Destroy()
		return c.JSON(kruda.Map{"message": "logged out"})
	})

	fmt.Println("Session example: http://localhost:3000")
	fmt.Println("  POST /login   → create session")
	fmt.Println("  GET  /profile → read session")
	fmt.Println("  POST /logout  → destroy session")
	app.Listen(":3000")
}
