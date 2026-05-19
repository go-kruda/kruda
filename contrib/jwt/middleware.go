package jwt

import (
	"errors"
	"strings"

	"github.com/go-kruda/kruda"
)

// New creates a JWT authentication middleware with the given config.
// It extracts the token from the configured location, verifies it,
// and stores the claims in the Kruda context as "jwt_claims".
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg = cfg.defaults()

	// Pre-parse lookup config once at registration time.
	source, name := parseLookup(cfg.Lookup)

	// Determine the verification key once.
	var verifyKey any
	var configErr error
	switch {
	case cfg.PublicKey != nil:
		verifyKey = cfg.PublicKey
		if cfg.Algorithm != "RS256" {
			configErr = errors.New("jwt: public key requires RS256 algorithm")
		}
	default:
		verifyKey = cfg.Secret
		if isHMACAlgorithm(cfg.Algorithm) && len(cfg.Secret) == 0 {
			configErr = ErrInvalidKey
		}
	}

	return func(c *kruda.Ctx) error {
		// R9.14: zero alloc on skip path
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}
		if configErr != nil {
			return c.Status(500).JSON(map[string]string{"error": "invalid_jwt_config"})
		}

		token := extractToken(c, source, name)
		if token == "" {
			return c.Status(401).JSON(map[string]string{"error": "missing_token"})
		}

		claims, err := VerifyWithAlgorithm(token, verifyKey, cfg.Algorithm)
		if err != nil {
			errMsg := "invalid_token"
			if err == ErrTokenExpired {
				errMsg = "token_expired"
			}
			return c.Status(401).JSON(map[string]string{"error": errMsg})
		}

		c.Set("jwt_claims", claims)
		return c.Next()
	}
}

func isHMACAlgorithm(alg string) bool {
	return alg == "HS256" || alg == "HS384" || alg == "HS512"
}

// parseLookup splits "source:name" into its parts.
func parseLookup(lookup string) (source, name string) {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "header", "Authorization"
}

// extractToken gets the token string from the configured location.
func extractToken(c *kruda.Ctx, source, name string) string {
	switch source {
	case "header":
		val := c.Header(name)
		if name == "Authorization" {
			// Strip "Bearer " prefix
			const prefix = "Bearer "
			if len(val) > len(prefix) && strings.EqualFold(val[:len(prefix)], prefix) {
				return val[len(prefix):]
			}
			return ""
		}
		return val
	case "query":
		return c.Query(name)
	case "cookie":
		return c.Cookie(name)
	default:
		return ""
	}
}
