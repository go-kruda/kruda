package middleware

import (
	"strconv"
	"strings"

	"github.com/go-kruda/kruda"
)

// CORSConfig holds configuration for the CORS middleware.
type CORSConfig struct {
	// AllowOrigins is a list of origins that are allowed to make cross-origin requests.
	// Default: ["*"]
	AllowOrigins []string

	// AllowMethods is a list of HTTP methods allowed for cross-origin requests.
	// Default: ["GET","POST","PUT","DELETE","PATCH","HEAD","OPTIONS"]
	AllowMethods []string

	// AllowHeaders is a list of HTTP headers allowed in cross-origin requests.
	// Default: ["Origin","Content-Type","Accept","Authorization"]
	AllowHeaders []string

	// AllowCredentials indicates whether the response to the request can be
	// exposed when the credentials flag is true.
	// Default: false
	AllowCredentials bool

	// ExposeHeaders is a list of headers that browsers are allowed to access.
	// Default: []
	ExposeHeaders []string

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached.
	// Default: 86400
	MaxAge int
}

// CORS returns middleware that handles Cross-Origin Resource Sharing.
// It supports both preflight (OPTIONS) and non-preflight requests.
// Panics if AllowCredentials is true with AllowOrigins=["*"] per CORS spec.
// M7 fix: adds Vary: Origin header when origin is not wildcard.
// M8 fix: Expose-Headers only set on non-preflight responses.
func CORS(config ...CORSConfig) kruda.HandlerFunc {
	cfg := CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       86400,
	}
	if len(config) > 0 {
		c := config[0]
		if len(c.AllowOrigins) > 0 {
			cfg.AllowOrigins = c.AllowOrigins
		}
		if len(c.AllowMethods) > 0 {
			cfg.AllowMethods = c.AllowMethods
		}
		if len(c.AllowHeaders) > 0 {
			cfg.AllowHeaders = c.AllowHeaders
		}
		cfg.AllowCredentials = c.AllowCredentials
		cfg.ExposeHeaders = c.ExposeHeaders
		if c.MaxAge > 0 {
			cfg.MaxAge = c.MaxAge
		}
	}

	// Validate: AllowCredentials=true with wildcard origin is a CORS spec violation.
	if cfg.AllowCredentials {
		for _, o := range cfg.AllowOrigins {
			if o == "*" {
				panic("kruda: CORS AllowCredentials=true cannot be used with AllowOrigins=[\"*\"]")
			}
		}
	}

	// Pre-build header values once at init time.
	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	maxAge := strconv.Itoa(cfg.MaxAge)
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")

	// Build origin lookup set for O(1) checks.
	allowAll := len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*"
	originSet := make(map[string]bool, len(cfg.AllowOrigins))
	for _, o := range cfg.AllowOrigins {
		originSet[o] = true
	}

	return func(c *kruda.Ctx) error {
		origin := c.Header("Origin")

		// Determine allowed origin value.
		var allowOrigin string
		if allowAll {
			allowOrigin = "*"
		} else if originSet[origin] {
			allowOrigin = origin
		} else {
			// Origin not allowed — skip CORS headers, proceed normally.
			return c.Next()
		}

		// N3 fix: use AddHeader to append Vary: Origin without clobbering existing Vary values
		if !allowAll {
			c.AddHeader("Vary", "Origin")
		}

		// Preflight request: OPTIONS with Access-Control-Request-Method header.
		if c.Method() == "OPTIONS" && c.Header("Access-Control-Request-Method") != "" {
			c.SetHeader("Access-Control-Allow-Origin", allowOrigin)
			c.SetHeader("Access-Control-Allow-Methods", allowMethods)
			c.SetHeader("Access-Control-Allow-Headers", allowHeaders)
			c.SetHeader("Access-Control-Max-Age", maxAge)
			if cfg.AllowCredentials {
				c.SetHeader("Access-Control-Allow-Credentials", "true")
			}
			// M8 fix: do NOT set Expose-Headers on preflight (per CORS spec)
			return c.NoContent()
		}

		// Non-preflight: set Allow-Origin and proceed to next handler.
		c.SetHeader("Access-Control-Allow-Origin", allowOrigin)
		if cfg.AllowCredentials {
			c.SetHeader("Access-Control-Allow-Credentials", "true")
		}
		// M8: Expose-Headers only on actual (non-preflight) responses
		if exposeHeaders != "" {
			c.SetHeader("Access-Control-Expose-Headers", exposeHeaders)
		}

		return c.Next()
	}
}
