# HTTP/2 Support ใน Kruda

> **Status:** รองรับแล้วผ่าน net/http transport + TLS
>
> **Requirement:** R11 — HTTP/2 Documentation
>
> **Ref:** #[[file:.kiro/specs/phase3-performance/requirements.md]] — Requirement 11

## Overview

Kruda รองรับ HTTP/2 ผ่าน net/http transport เมื่อ configure TLS แล้ว HTTP/2 จะถูก negotiate อัตโนมัติผ่าน ALPN (Application-Layer Protocol Negotiation) — ไม่ต้อง configure อะไรเพิ่มเติม

Go stdlib `net/http` server รองรับ HTTP/2 โดย default เมื่อใช้ TLS ซึ่ง Kruda ใช้ประโยชน์จาก feature นี้โดยตรง

## วิธีเปิดใช้งาน HTTP/2

ใช้ `WithTLS()` option เพื่อ configure TLS certificate — HTTP/2 จะถูก enable อัตโนมัติ:

```go
package main

import "github.com/go-kruda/kruda"

func main() {
    app := kruda.New(
        kruda.WithTLS("cert.pem", "key.pem"),
    )

    app.Get("/", func(c *kruda.Ctx) error {
        return c.JSON(map[string]string{
            "protocol": "HTTP/2 auto-negotiated via ALPN",
        })
    })

    // HTTPS server พร้อม HTTP/2 support
    app.Listen(":443")
}
```

เพียงแค่นี้ — ไม่ต้อง import package เพิ่ม ไม่ต้อง set flag ไม่ต้อง configure HTTP/2 แยก

## การทำงานภายใน

1. `WithTLS(certFile, keyFile)` เก็บ path ของ certificate และ key ใน `Config`
2. `selectTransport()` ตรวจพบว่ามี TLS config → เลือก net/http transport อัตโนมัติ
3. `NetHTTPTransport.ListenAndServe()` เรียก `http.Server.ListenAndServeTLS()` แทน `ListenAndServe()`
4. Go stdlib จัดการ TLS handshake + ALPN negotiation → client ที่รองรับ HTTP/2 จะได้ HTTP/2 connection

```
Client (browser/curl)
    ↓ TLS handshake + ALPN "h2"
net/http server (Go stdlib)
    ↓ HTTP/2 multiplexed streams
Kruda App.ServeKruda()
    ↓ transport-agnostic — ทำงานเหมือน HTTP/1.1 ทุกประการ
Handler chain → Response
```

## Netpoll Transport กับ HTTP/2

**สำคัญ:** Netpoll transport ไม่รองรับ HTTP/2 เนื่องจาก Netpoll ทำงานที่ระดับ raw TCP connection และ parse HTTP/1.1 เอง — ไม่มี TLS layer และ HTTP/2 framing built-in

เมื่อ configure `WithTLS()` ร่วมกับ Netpoll transport, Kruda จะ:

1. Log warning: `"TLS configured with netpoll — falling back to nethttp for HTTP/2"`
2. **สลับไปใช้ net/http transport อัตโนมัติ** เพื่อให้ HTTP/2 ทำงานได้

```go
// กรณีนี้ Kruda จะ fallback ไป net/http อัตโนมัติ
app := kruda.New(
    kruda.WithTransportName("netpoll"),
    kruda.WithTLS("cert.pem", "key.pem"),
)
// ⚠️ Log: "TLS configured with netpoll — falling back to nethttp for HTTP/2"
// ✅ ใช้ net/http transport จริง — HTTP/2 ทำงานปกติ
```

ผู้ใช้ไม่ต้องจัดการ fallback เอง — framework ทำให้อัตโนมัติ

## Auto-selection Logic

`selectTransport()` มี priority ดังนี้เมื่อมี TLS:

| Config | Transport ที่ใช้จริง | เหตุผล |
|--------|---------------------|--------|
| `WithTLS()` เฉยๆ | net/http | TLS → ต้องใช้ net/http สำหรับ HTTP/2 |
| `WithTLS()` + `WithTransportName("netpoll")` | net/http | Netpoll ไม่ support TLS → fallback + warning |
| `WithTLS()` + `WithTransportName("nethttp")` | net/http | ตรงตาม config |
| ไม่มี TLS + auto | Netpoll (Linux/macOS) | ไม่ต้องการ HTTP/2 → ใช้ Netpoll ได้ |
| ไม่มี TLS + Windows | net/http | Netpoll ไม่ support Windows |

## คำแนะนำสำหรับ Request ขนาดใหญ่

สำหรับ request ที่มี body ขนาดใหญ่กว่า 1MB ควรพิจารณาใช้ net/http transport หรือ streaming แทน Netpoll transport เนื่องจาก Netpoll buffer request body ทั้งหมดใน memory ก่อน process (Decision D-006)

```go
// สำหรับ API ที่รับ file upload ขนาดใหญ่
app := kruda.New(
    kruda.WithTransportName("nethttp"), // ใช้ net/http สำหรับ large body handling
    kruda.WithBodyLimit(10 * 1024 * 1024), // 10MB limit
)
```

## HTTP/2 กับ HTTP/3

ถ้าต้องการทั้ง HTTP/2 และ HTTP/3 (QUIC) ใช้ `WithHTTP3()`:

```go
app := kruda.New(
    kruda.WithHTTP3("cert.pem", "key.pem"),
)
```

`WithHTTP3()` จะ:
- เปิด HTTP/2 (TCP) เป็น primary transport ผ่าน net/http + TLS
- เปิด HTTP/3 (UDP/QUIC) เป็น secondary transport ใน background
- Inject `Alt-Svc` header อัตโนมัติเพื่อ advertise HTTP/3 ให้ client

ดูรายละเอียดเพิ่มเติมที่ HTTP/3 documentation

## สรุป

| Feature | Status |
|---------|--------|
| HTTP/2 via net/http + TLS | ✅ รองรับ — auto-negotiated via ALPN |
| HTTP/2 via Netpoll | ❌ ไม่รองรับ — auto-fallback to net/http |
| TLS + Netpoll auto-fallback | ✅ อัตโนมัติ พร้อม warning log |
| HTTP/2 configuration | ไม่ต้อง configure เพิ่ม — แค่ `WithTLS()` |
| HTTP/2 + HTTP/3 dual-stack | ✅ ผ่าน `WithHTTP3()` |
