package main

import (
	"os"
	"os/signal"
	"syscall"

	kruda "github.com/go-kruda/kruda"
)

var j = []byte(`{"message":"Hello, World!"}`)
var t = []byte("Hello, World!")

func main() {
	app := kruda.New(kruda.FastHTTP())
	app.Get("/plaintext", func(c *kruda.Ctx) error { c.SetHeader("Content-Type", "text/plain"); return c.SendBytes(t) })
	app.Get("/json", func(c *kruda.Ctx) error { c.SetHeader("Content-Type", "application/json"); return c.SendBytes(j) })
	app.Compile()
	go app.Listen(":18080")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
