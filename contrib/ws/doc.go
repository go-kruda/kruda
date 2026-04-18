// Package ws provides WebSocket support for Kruda.
//
// It implements RFC 6455 with text and binary frames, fragmented messages,
// ping/pong, the close handshake, and origin validation.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/ws"
//
//	app := kruda.New()
//	upgrader := ws.New()
//	app.Get("/echo", func(c *kruda.Ctx) error {
//	    return upgrader.Upgrade(c, func(conn *ws.Conn) {
//	        defer conn.Close()
//	        for {
//	            mt, msg, err := conn.ReadMessage()
//	            if err != nil { return }
//	            _ = conn.WriteMessage(mt, msg)
//	        }
//	    })
//	})
//
// # Transport compatibility
//
//   - net/http: supported (via http.Hijacker)
//   - fasthttp: supported (via RequestCtx.Hijack)
//   - Wing:     not supported in v1 — Wing manages the fd directly via
//               epoll/kqueue and does not expose a hijack API. Routes that
//               need WebSocket should run under net/http or fasthttp.
//
// # What it does
//
// [Upgrader.Upgrade] performs the RFC 6455 handshake, hands the resulting
// [Conn] synchronously to the user callback, and enforces the configured
// per-message and per-frame size limits, read/write deadlines, and ping
// rate limit. MessageTimeout caps the time allowed to assemble a
// fragmented message (mitigating slowloris-style attacks);
// MaxPingPerSecond closes flooding clients with ClosePolicyViolation.
//
// # Configuration
//
//   - AllowedOrigins / StrictOrigin: origin validation
//   - MaxMessageSize:    bytes; enforced at frame and message level
//   - ReadTimeout / WriteTimeout: per-message deadlines
//   - ReadBufferSize / WriteBufferSize: I/O buffer sizes (default 4096)
//   - MessageTimeout:    deadline for assembling a fragmented message
//   - MaxPingPerSecond:  per-connection ping rate cap
//
// # See also
//
//   - RFC 6455 (The WebSocket Protocol)
package ws
