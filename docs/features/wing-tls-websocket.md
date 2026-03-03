# Wing TLS + WebSocket Design

## Summary of Decisions

### 1. TLS — Separate Method

```go
func (t *Transport) ListenAndServe(addr string, handler Handler) error
func (t *Transport) ListenAndServeTLS(addr, certFile, keyFile string, handler Handler) error
```

- Separate code path — TLS uses goroutine-per-conn, not epoll
- Zero impact on plaintext performance (100% separate path)
- Consistent with net/http and Fiber API
- No `if config.TLS` check on hot path

### 2. WebSocket Handler — Simple Func

```go
type WSHandler func(c *WSConn)
```

- Go idiomatic for callbacks
- No struct required to implement interface
- Consistent with Fiber: `func(*websocket.Conn)`

### 3. WSConn API

```go
// Message type constants
const (
    TextMessage   = 1
    BinaryMessage = 2
    CloseMessage  = 8
    PingMessage   = 9
    PongMessage   = 10
)

type WSConn struct { /* internal */ }

// Core
func (c *WSConn) ReadMessage() (messageType int, data []byte, err error)
func (c *WSConn) WriteMessage(messageType int, data []byte) error
func (c *WSConn) Close() error
func (c *WSConn) RemoteAddr() string

// Shorthands
func (c *WSConn) WriteText(text string) error
func (c *WSConn) WriteBinary(data []byte) error

// Deadlines
func (c *WSConn) SetReadDeadline(t time.Time) error
func (c *WSConn) SetWriteDeadline(t time.Time) error
```

### 4. Benchmark Guard — in bench_test.go

```go
// transport/wing/bench_test.go
func TestPlaintextPerformanceGuard(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping performance guard in short mode")
    }
    result := testing.Benchmark(BenchmarkPlaintextBaseline)
    nsPerOp := result.NsPerOp()
    const maxNsPerOp = 2000 // ~500K req/s minimum
    if nsPerOp > maxNsPerOp {
        t.Errorf("Performance regression: %d ns/op, want < %d", nsPerOp, maxNsPerOp)
    }
}
```

---

## Performance Guarantee

| Feature | Impact on plaintext | Protection mechanism |
|---------|--------------------|--------------------|
| TLS | 0% | Separate ListenAndServeTLS() code path |
| WebSocket | <1ns | Route-level `isWS` bool, checked only after route match |

WebSocket check is a single bool compare after radix tree match — not on every request.

---

## Implementation Files

```
transport/wing/
├── tls.go          # ListenAndServeTLS — goroutine-per-conn mode
├── websocket.go    # WSConn, WSHandler, WebSocket() middleware
├── ws_frame.go     # RFC 6455 frame parser/builder
├── ws_upgrade.go   # HTTP → WebSocket upgrade handshake
└── ws_test.go      # Tests
```

---

## TLS Implementation Sketch

```go
// tls.go
func (t *Transport) ListenAndServeTLS(addr, certFile, keyFile string, handler Handler) error {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return err
    }
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        NextProtos:   []string{"http/1.1"}, // no h2 in Wing
    }
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    tlsLn := tls.NewListener(ln, tlsConfig)
    // goroutine-per-conn — reuses HTTP parser + response builder
    for {
        conn, err := tlsLn.Accept()
        if err != nil {
            return err
        }
        go t.handleTLSConn(conn, handler)
    }
}
```

---

## WebSocket Upgrade Sketch

```go
// ws_upgrade.go
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func computeAcceptKey(key string) string {
    h := sha1.New()
    h.Write([]byte(key + wsGUID))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ws_frame.go — RFC 6455 frame
type wsFrame struct {
    fin     bool
    opcode  byte
    masked  bool
    payload []byte
}

func parseFrame(buf []byte) (frame wsFrame, consumed int, ok bool) {
    if len(buf) < 2 {
        return frame, 0, false
    }
    frame.fin    = buf[0]&0x80 != 0
    frame.opcode = buf[0] & 0x0F
    frame.masked = buf[1]&0x80 != 0
    payloadLen   := int(buf[1] & 0x7F)
    headerLen    := 2

    switch payloadLen {
    case 126:
        if len(buf) < 4 { return frame, 0, false }
        payloadLen = int(binary.BigEndian.Uint16(buf[2:4]))
        headerLen  = 4
    case 127:
        if len(buf) < 10 { return frame, 0, false }
        payloadLen = int(binary.BigEndian.Uint64(buf[2:10]))
        headerLen  = 10
    }

    maskLen := 0
    if frame.masked { maskLen = 4 }
    totalLen := headerLen + maskLen + payloadLen
    if len(buf) < totalLen { return frame, 0, false }

    frame.payload = buf[headerLen+maskLen : totalLen]
    if frame.masked {
        mask := buf[headerLen : headerLen+4]
        for i := range frame.payload {
            frame.payload[i] ^= mask[i%4]
        }
    }
    return frame, totalLen, true
}
```

---

## Timeline

| Phase | Task | Effort |
|-------|------|--------|
| 3.1 | TLS (goroutine-per-conn) | 2-3 days |
| 3.2 | WebSocket upgrade + frame parser | 3-4 days |
| 3.3 | WSConn API + middleware | 2 days |
| 3.4 | Tests + benchmark guard | 1-2 days |

---

## Non-Goals

- HTTP/2 in Wing — use Soar preset (net/http transport) or reverse proxy
- Kernel TLS (ktls) — future optimization
- WebSocket compression (permessage-deflate) — Phase 6 contrib
