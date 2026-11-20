// Package ratelimit provides rate limiting middleware for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/ratelimit"
//
//	app := kruda.New()
//	app.Use(ratelimit.New(ratelimit.Config{
//	    Max:    100,
//	    Window: time.Minute,
//	}))
//
// Per-route limit:
//
//	app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))
//
// # What it does
//
// The middleware identifies clients by IP address (or a custom KeyFunc),
// counts requests in the chosen Window, and short-circuits with HTTP 429
// when the limit is exceeded. Standard headers are set on every response:
// X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, and
// Retry-After on 429 responses.
//
// Two algorithms are available:
//
//   - "token_bucket"   — default; smoother bursts, lower memory
//   - "sliding_window" — strict windowed count
//
// Clients are identified by c.IP(), so the app's WithTrustProxy and
// WithTrustedProxies settings apply. Setting TrustedProxies here replaces them
// for the limiter only. Either way, forwarded chains are read from right to
// left, and malformed or repeated headers fall back to the peer. CIDR is not
// supported; list each proxy IP individually.
//
// # Configuration
//
//   - Max / Window:     required limit and window
//   - Algorithm:        "token_bucket" (default) or "sliding_window"
//   - KeyFunc:          override default IP-based identifier
//   - TrustedProxies:   exact proxy IPs allowed to set X-Forwarded-For
//   - CleanupInterval:  expiry sweep interval (default 1 minute)
//   - Store:            backing store (default in-memory)
//   - Skip:             per-request bypass function
//
// # See also
//
//   - [ForRoute] — convenience wrapper for a single-route stricter limit
package ratelimit
