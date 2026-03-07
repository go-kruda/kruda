# WebSocket

Real-time echo server using `contrib/ws` with message size limits and an HTML test page.

## Run

```bash
go run ./examples/websocket/
```

## Test

```bash
# Using websocat
websocat ws://localhost:3000/ws

# Or open http://localhost:3000 in a browser
```
