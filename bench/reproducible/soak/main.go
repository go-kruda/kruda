// Soak server: exercises the Wing hot path plus SSE stream churn so
// soak.sh can assert goroutine/RSS stability over long runs.
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	kruda "github.com/go-kruda/kruda"
)

func main() {
	port := os.Getenv("SOAK_PORT")
	if port == "" {
		port = "3499"
	}
	app := kruda.New(kruda.Wing())
	app.Get("/plaintext", func(c *kruda.Ctx) error { return c.Text("hello") })
	app.Get("/json", func(c *kruda.Ctx) error {
		return c.JSON(map[string]string{"message": "hello"})
	})
	app.Get("/events", func(c *kruda.Ctx) error {
		return c.SSE(func(s *kruda.SSEStream) error {
			tick := time.NewTicker(50 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-s.Done():
					return nil
				case now := <-tick.C:
					if err := s.Data(now.UnixNano()); err != nil {
						return err
					}
				}
			}
		})
	}, kruda.Stream)
	app.Get("/soakstats", func(c *kruda.Ctx) error {
		return c.JSON(map[string]any{"goroutines": runtime.NumGoroutine()})
	})
	fmt.Println("soak server listening on :" + port)
	if err := app.Listen(":" + port); err != nil {
		panic(err)
	}
}
