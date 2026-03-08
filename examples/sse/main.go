package main

import (
	"fmt"
	"time"

	"github.com/go-kruda/kruda"
)

func main() {
	app := kruda.New(kruda.NetHTTP())

	// SSE endpoint — streams time every second
	app.Get("/events", func(c *kruda.Ctx) error {
		return c.SSE(func(s *kruda.SSEStream) error {
			for i := 1; i <= 10; i++ {
				if err := s.Event("time", kruda.Map{
					"tick": i,
					"now":  time.Now().Format(time.RFC3339),
				}); err != nil {
					return err // client disconnected
				}
				time.Sleep(time.Second)
			}
			return s.Event("done", kruda.Map{"message": "stream complete"})
		})
	})

	// Simple HTML page to test SSE
	app.Get("/", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "text/html")
		return c.HTML(`<!DOCTYPE html>
<html><body>
<h2>SSE Demo</h2>
<pre id="log"></pre>
<script>
const es = new EventSource("/events");
const log = document.getElementById("log");
es.addEventListener("time", e => { log.textContent += e.data + "\n"; });
es.addEventListener("done", e => { log.textContent += "DONE: " + e.data + "\n"; es.close(); });
es.onerror = () => { log.textContent += "Connection closed\n"; es.close(); };
</script>
</body></html>`)
	})

	fmt.Println("SSE example: http://localhost:3000")
	app.Listen(":3000")
}
