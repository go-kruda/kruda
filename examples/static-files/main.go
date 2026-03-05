package main

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/go-kruda/kruda"
)

//go:embed public/*
var publicFS embed.FS

func main() {
	app := kruda.New()

	// Serve embedded files at /static/*
	sub, _ := fs.Sub(publicFS, "public")
	app.StaticFS("/static", sub)

	app.Get("/", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "text/html")
		return c.HTML(`<!DOCTYPE html>
<html><body>
<h2>Static Files Demo</h2>
<p>Try: <a href="/static/hello.txt">/static/hello.txt</a></p>
<p>Try: <a href="/static/style.css">/static/style.css</a></p>
</body></html>`)
	})

	fmt.Println("Static files example: http://localhost:3000")
	app.Listen(":3000")
}
