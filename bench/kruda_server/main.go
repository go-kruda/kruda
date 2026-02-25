package main

import (
	"os"

	kruda "github.com/go-kruda/kruda"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	var opts []kruda.Option
	if transport := os.Getenv("TRANSPORT"); transport != "" {
		opts = append(opts, kruda.WithTransportName(transport))
	}

	app := kruda.New(opts...)
	
	app.Get("/", func(c *kruda.Ctx) error {
		return c.Text("Hello, World!")
	})
	
	app.Get("/users/:id", func(c *kruda.Ctx) error {
		return c.Text(c.Param("id"))
	})
	
	app.Post("/json", func(c *kruda.Ctx) error {
		var body map[string]interface{}
		if err := c.Bind(&body); err != nil {
			return err
		}
		return c.JSON(body)
	})

	app.Listen(":" + port)
}