# SSE

Server-Sent Events streaming with `c.SSE()`. Sends 10 time ticks then closes the stream.

## Run

```bash
go run ./examples/sse/
```

## Test

```bash
# CLI stream
curl -N http://localhost:3000/events

# Or open http://localhost:3000 in a browser
```
