# WebSocket

WebSocket RFC 6455 implementation with text/binary frames, ping/pong, and close handshake.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/ws
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/ws"

upgrader := ws.New(ws.Config{})

app.Get("/ws", func(c *kruda.Ctx) error {
    return upgrader.Upgrade(c, func(conn *ws.Conn) {
        msg, err := conn.ReadMessage()
        if err == nil {
            _ = conn.WriteMessage(ws.TextMessage, msg)
        }
    })
})
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| AllowedOrigins | []string | nil | Allowed origin values; nil allows all origins |
| StrictOrigin | bool | false | Require Origin when AllowedOrigins is set |
| MaxMessageSize | int64 | 1048576 | Maximum message size |
| ReadBufferSize | int | 4096 | Read buffer size |
| WriteBufferSize | int | 4096 | Write buffer size |
| MessageTimeout | time.Duration | 30s | Fragmented message assembly timeout |
| MaxPingPerSecond | int | 10 | Ping frame rate limit |
