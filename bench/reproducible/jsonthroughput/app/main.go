// Subject for "does the JSON engine change req/sec, and where".
//
// Three routes chosen to match the payload shapes the engine benchmarks measure,
// so a microbenchmark gain can be traced to a throughput gain or shown not to
// reach one:
//
//	/small   encode ~30 B, the TFB /json shape — where the syscall floor dominates
//	/large   encode a 100-item array (~9 KB), the shape json/engine_bench_test.go uses
//	/decode  POST: decode a 100-item body, reply small — the 6× decode path
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-kruda/kruda"
)

type message struct {
	Message string `json:"message"`
}

type item struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Active bool   `json:"active"`
	Score  int    `json:"score"`
}

func payload(n int) []item {
	out := make([]item, n)
	for i := range out {
		out[i] = item{
			ID:     i,
			Name:   "user-" + strconv.Itoa(i),
			Email:  "user" + strconv.Itoa(i) + "@example.com",
			Active: i%2 == 0,
			Score:  i * 7,
		}
	}
	return out
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3700"
	}
	large := payload(100)

	app := kruda.New()

	app.Get("/small", func(c *kruda.Ctx) error {
		return c.JSON(message{Message: "Hello, World!"})
	}, kruda.JSON)

	app.Get("/large", func(c *kruda.Ctx) error {
		return c.JSON(large)
	}, kruda.JSON)

	app.Post("/decode", func(c *kruda.Ctx) error {
		var in []item
		if err := c.Bind(&in); err != nil {
			return c.Status(400).JSON(message{Message: "bad"})
		}
		return c.JSON(message{Message: strconv.Itoa(len(in))})
	}, kruda.JSON)

	app.Get("/ready", func(c *kruda.Ctx) error { return c.Text("ok") })

	if err := app.Listen(":" + port); err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
}
