# Rate Limit

Token bucket and sliding window rate limiting with per-IP or custom key support.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/ratelimit
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/ratelimit"

// Global rate limit - 100 req/min per IP
app.Use(ratelimit.New(ratelimit.Config{
    Max: 100,
    Window: time.Minute,
}))

// Per-route limit
app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Max | int | 100 | Maximum requests |
| Window | time.Duration | time.Minute | Time window |
| KeyFunc | func(*kruda.Ctx) string | IP-based | Custom key function |
| Algorithm | string | "token_bucket" | Algorithm type |
| TrustedProxies | []string | nil | Trusted proxy IPs |

## Client IP and trusted proxies

The default key uses the socket peer, independently of `WithTrustProxy`. When
`TrustedProxies` contains the peer IP, the limiter checks `X-Forwarded-For`
from right to left and stops at the nearest untrusted IP. Invalid or repeated
forwarding headers fall back to the socket peer. `X-Real-IP` is used only when
`X-Forwarded-For` is absent. Only exact IPs are supported.

Custom transports must implement `PeerAddr() string` on their request to use
the default per-IP key. To honor forwarded headers they must additionally
implement `ForwardedHeaders() (string, int, string, int)`, returning the
first X-Forwarded-For value and field count, followed by the first X-Real-IP
value and field count. If the peer is unavailable or invalid, requests share
the `unknown` key; a custom `KeyFunc` can provide another policy.

The module remains compatible with core v1.7.2. Native net/http and fasthttp
requests expose the socket peer through their raw request on older cores.
Custom transports should implement `PeerAddr() string` and
`ForwardedHeaders() (string, int, string, int)`. On older Wing or custom
transports without a raw peer, direct requests without forwarding headers use
`RemoteAddr`; requests containing forwarding headers fail closed into the
`unknown` bucket. Use the updated core for trusted forwarding on Wing.
