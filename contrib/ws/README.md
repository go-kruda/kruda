# WebSocket

WebSocket RFC 6455 implementation with text/binary frames, ping/pong, and close handshake.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/ws
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/ws"

app.Get("/ws", ws.New(ws.Config{}))

// In handler
conn, err := ws.Upgrade(c)
if err != nil {
    return err
}
defer conn.Close()

// Read/write messages
msg, err := conn.ReadMessage()
err = conn.WriteMessage(ws.TextMessage, []byte("hello"))
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| CheckOrigin | func | nil | Origin validation function |
| Subprotocols | []string | nil | Supported subprotocols |
| ReadBufferSize | int | 4096 | Read buffer size |
| WriteBufferSize | int | 4096 | Write buffer size |