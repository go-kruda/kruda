package jwt

import (
	"crypto/rsa"
	"time"

	"github.com/go-kruda/kruda"
)

// Config holds the configuration for the JWT middleware.
type Config struct {
	// Secret is the HMAC signing key (used for HS256/HS384/HS512).
	Secret []byte

	// PublicKey is the RSA public key for RS256 verification.
	PublicKey *rsa.PublicKey

	// PrivateKey is the RSA private key for RS256 signing.
	PrivateKey *rsa.PrivateKey

	// Algorithm specifies the signing algorithm: HS256 (default), HS384, HS512, RS256.
	Algorithm string

	// Lookup specifies where to extract the token from.
	// Format: "source:name" — e.g. "header:Authorization" (default), "query:token", "cookie:jwt".
	Lookup string

	// Skip is an optional function that returns true for requests that should
	// bypass JWT authentication (e.g. health checks, public routes).
	// R9.14: zero allocations on skip path.
	Skip func(*kruda.Ctx) bool

	// GracePeriod allows expired tokens to be refreshed within this duration
	// after expiration.
	GracePeriod time.Duration
}

// defaults returns a Config with default values applied.
func (cfg Config) defaults() Config {
	if cfg.Algorithm == "" {
		cfg.Algorithm = "HS256"
	}
	if cfg.Lookup == "" {
		cfg.Lookup = "header:Authorization"
	}
	return cfg
}
