# WebSocket

WebSocket RFC 6455 implementation with text/binary frames, ping/pong, and close handshake.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/ws
```

## Usage

```go
import (
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/contrib/ws"
)

app := kruda.New()

ws.HandleFunc(app, "/ws", func(conn *ws.Conn) {
    msg, err := conn.ReadMessage()
    if err == nil {
        _ = conn.WriteMessage(ws.TextMessage, msg)
    }
}, ws.Config{})
```

`ws.HandleFunc` works with Wing and net/http. It applies the `kruda.Hijack` route preset required by Wing; fasthttp does not support WebSocket upgrades.

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| AllowedOrigins | []string | nil | Allowed origin values; nil allows the same origin only, `"*"` allows all |
| StrictOrigin | bool | false | Require Origin when AllowedOrigins is set |
| MaxMessageSize | int64 | 1048576 | Maximum message size |
| ReadBufferSize | int | 4096 | Read buffer size |
| WriteBufferSize | int | 4096 | Write buffer size |
| MessageTimeout | time.Duration | 30s | Fragmented message assembly timeout |
| MaxPingPerSecond | int | 10 | Ping frame rate limit |

## Origin checks behind a proxy

With no `AllowedOrigins`, the origin's scheme, host and port must match the
request. A TLS-terminating proxy can preserve the public `Host` and overwrite
`X-Forwarded-Proto` with `https`. Configure its socket IP with
`kruda.WithTrustedProxies(...)` and enable `kruda.WithTrustProxy(true)`.
This uses the core version that provides `Ctx.IsTrustedProxy`.

When the proxy rewrites the authority or the core does not provide that
method, set `AllowedOrigins` to the public origins, for example
`[]string{"https://app.example.com"}`.
