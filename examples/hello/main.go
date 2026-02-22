package main

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

func main() {
	app := kruda.New()

	// Global middleware
	app.Use(middleware.Recovery())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())
	app.Use(middleware.CORS())

	// Root routes
	app.Get("/ping", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"pong": true})
	})

	app.Get("/", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{
			"name":    "Kruda",
			"version": "0.1.0",
		})
	})

	// API group with scoped middleware
	api := app.Group("/api")

	api.Get("/hello", func(c *kruda.Ctx) error {
		name := c.Query("name", "World")
		return c.JSON(kruda.Map{"message": "Hello, " + name + "!"})
	})

	api.Post("/echo", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"body": c.BodyString()})
	})

	// Nested group
	v1 := api.Group("/v1")
	v1.Get("/status", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"status": "ok"})
	})

	app.Listen(":3000")
}
