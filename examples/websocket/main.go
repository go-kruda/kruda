// Example: WebSocket — Real-time echo server using contrib/ws
//
// Demonstrates WebSocket support with Kruda:
//   - Upgrading HTTP connections to WebSocket
//   - Reading and writing text/binary messages
//   - Origin validation
//   - Max message size enforcement
//
// Run: go run ./examples/websocket/
// Test with websocat: websocat ws://localhost:3000/ws
// Or use browser console:
//
//	ws = new WebSocket("ws://localhost:3000/ws")
//	ws.onmessage = e => console.log(e.data)
//	ws.send("hello")
package main

import (
	"fmt"
	"log"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/contrib/ws"
	"github.com/go-kruda/kruda/middleware"
)

func main() {
	app := kruda.New()

	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	// WebSocket upgrader with config
	upgrader := ws.New(ws.Config{
		MaxMessageSize: 64 * 1024, // 64KB max message
	})

	// Echo WebSocket endpoint
	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *ws.Conn) {
			defer conn.Close(ws.CloseNormalClosure, "bye")

			for {
				msgType, data, err := conn.ReadMessage()
				if err != nil {
					return // client disconnected
				}

				// Echo the message back
				if err := conn.WriteMessage(msgType, data); err != nil {
					return
				}
			}
		})
	})

	app.Get("/", func(c *kruda.Ctx) error {
		return c.HTML(`<!DOCTYPE html>
<html><head><title>Kruda WebSocket</title></head>
<body>
<h1>WebSocket Echo</h1>
<input id="msg" placeholder="Type a message..." />
<button onclick="send()">Send</button>
<pre id="log"></pre>
<script>
const ws = new WebSocket("ws://" + location.host + "/ws");
const log = document.getElementById("log");
ws.onmessage = e => log.textContent += "← " + e.data + "\n";
ws.onopen = () => log.textContent += "connected\n";
ws.onclose = () => log.textContent += "disconnected\n";
function send() {
  const msg = document.getElementById("msg").value;
  ws.send(msg);
  log.textContent += "→ " + msg + "\n";
  document.getElementById("msg").value = "";
}
</script>
</body></html>`)
	})

	fmt.Println("WebSocket example listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
