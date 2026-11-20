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
| TrustedProxies | []string | nil | Trusted proxy IPs for the limiter only; nil follows the app |

## Client IP and trusted proxies

The default key uses `c.IP()`, so it follows the app's `WithTrustProxy` and
`WithTrustedProxies`. Behind a trusted proxy each client keeps its own bucket.
Setting `TrustedProxies` here replaces the app's list for the limiter. When the
peer is trusted, `X-Forwarded-For` is read from right to left and stops at the
nearest untrusted IP. Invalid or repeated
forwarding headers fall back to the socket peer. `X-Real-IP` is used only when
`X-Forwarded-For` is absent. Only exact IPs are supported.

The default key is the client IP. IPv6 clients are grouped by their /64
network, written like `2001:db8:1:2::/64`, because one subscriber usually
holds a whole /64 and could otherwise use a fresh address for every request.
IPv4 clients keep one key per address.

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
