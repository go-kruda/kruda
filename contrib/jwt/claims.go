// Package jwt provides JWT authentication middleware for Kruda.
// It supports HS256, HS384, HS512, and RS256 algorithms using only Go stdlib.
package jwt

// Claims represents the payload of a JWT token.
// It includes registered claims (RFC 7519) and an Extra map for custom fields.
type Claims struct {
	Subject   string         `json:"sub,omitempty"`
	Issuer    string         `json:"iss,omitempty"`
	Audience  string         `json:"aud,omitempty"`
	ExpiresAt int64          `json:"exp,omitempty"`
	IssuedAt  int64          `json:"iat,omitempty"`
	NotBefore int64          `json:"nbf,omitempty"`
	ID        string         `json:"jti,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}
