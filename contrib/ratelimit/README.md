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
| KeyGenerator | func | IP-based | Custom key function |
| Algorithm | string | "token_bucket" | Algorithm type |
| TrustedProxies | []string | nil | Trusted proxy IPs |