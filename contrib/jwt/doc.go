// Package jwt provides JWT authentication middleware for Kruda.
//
// It supports HS256, HS384, HS512, and RS256 algorithms using only the Go
// standard library — no third-party crypto.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/jwt"
//
//	app := kruda.New()
//	api := app.Group("/api").Guard(jwt.New(jwt.Config{
//	    Secret: []byte(os.Getenv("JWT_SECRET")),
//	}))
//
// # What it does
//
// The middleware extracts a token from the configured Lookup source
// (header, query, or cookie), verifies its signature against Secret /
// PublicKey, validates the standard time claims (exp, nbf), and stores the
// resolved [Claims] on the request context. Failures short-circuit with a
// 401 response. The Skip function lets callers bypass auth on public
// routes (e.g. health checks) with zero allocations on the skip path.
//
// Token signing ([Sign]) and refresh ([Refresh]) helpers are also exported
// for handlers that issue tokens themselves.
//
// # Configuration
//
//   - Secret:      HMAC signing key (HS256/HS384/HS512)
//   - PublicKey:   RSA public key for RS256 verification
//   - PrivateKey:  RSA private key for RS256 signing
//   - Algorithm:   HS256 (default), HS384, HS512, or RS256
//   - Lookup:      "source:name" — e.g. "header:Authorization" (default),
//                  "query:token", "cookie:jwt"
//   - GracePeriod: window after exp during which a token is still
//                  refreshable but not authoritative
//   - Skip:        per-request bypass function
//
// # See also
//
//   - RFC 7519 (JSON Web Token)
//   - RFC 6750 (Bearer Token Usage)
package jwt
