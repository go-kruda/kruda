package etag

import (
	"fmt"
	"hash/crc32"
	"net/http"

	"github.com/go-kruda/kruda"
)

// Config holds ETag middleware configuration.
type Config struct {
	// Weak generates weak ETags (W/"...") when true, strong ETags otherwise.
	// Default: true (weak ETags are faster and sufficient for most use cases).
	Weak bool

	// Skip allows conditional bypassing of ETag generation.
	// Return true to skip ETag processing for this request.
	Skip func(*kruda.Ctx) bool
}

// defaults applies default configuration values.
func (c *Config) defaults() {
	c.Weak = true // Default to weak ETags
}

// New creates an ETag middleware with the given configuration.
//
// This middleware handles conditional GET/HEAD requests by:
//  1. Parsing the If-None-Match header from the client
//  2. Storing the ETag configuration in context for handler use
//  3. After the handler runs, checking if an ETag was set via SetETag/GenerateAndSetETag
//
// Handlers are responsible for generating ETags using the helper functions:
//   - SetETag(c, etag) — sets ETag and checks for 304
//   - GenerateAndSetETag(c, body) — generates ETag from body, sets it, and checks for 304
//   - GenerateETag(body, weak) — generates an ETag string without setting it
//
// Example:
//
//	app.Use(etag.New())
//	app.Get("/data", func(c *kruda.Ctx) error {
//	    data := fetchData()
//	    body, _ := json.Marshal(data)
//	    if etag.GenerateAndSetETag(c, body) {
//	        return nil // 304 sent
//	    }
//	    return c.JSON(data)
//	})
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.defaults()

	return func(c *kruda.Ctx) error {
		// Skip non-GET/HEAD requests
		method := c.Method()
		if method != http.MethodGet && method != http.MethodHead {
			return c.Next()
		}

		// Skip if configured to skip
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		// Parse If-None-Match header and store for helper functions
		ifNoneMatch := c.Header("If-None-Match")

		// Store ETag config in context for handlers to use
		c.Set("etag:weak", cfg.Weak)
		c.Set("etag:if-none-match", ifNoneMatch)

		// Execute handler chain
		return c.Next()
	}
}

// GenerateETag creates an ETag from response body using CRC32 + size.
// Returns empty string for empty body.
func GenerateETag(body []byte, weak bool) string {
	if len(body) == 0 {
		return ""
	}
	hash := crc32.ChecksumIEEE(body)
	etag := fmt.Sprintf(`"%x-%d"`, hash, len(body))
	if weak {
		etag = "W/" + etag
	}
	return etag
}

// SetETag sets an ETag header and checks for conditional requests.
// Returns true if a 304 Not Modified response was sent (handler should return nil).
// Returns false if the handler should continue sending the full response.
//
// Uses Kruda's c.Status() to properly track the 304 response through
// the middleware chain, avoiding the response tracking bypass.
func SetETag(c *kruda.Ctx, etag string) bool {
	if etag == "" {
		return false
	}

	// Set ETag header via transport writer directly.
	// c.SetHeader() may not be flushed by all response methods (e.g. c.Text()
	// has a fast path that bypasses writeHeaders), so we write directly.
	c.ResponseWriter().Header().Set("ETag", etag)

	// Check If-None-Match from middleware or directly from request
	ifNoneMatch := ""
	if v, ok := c.Get("etag:if-none-match").(string); ok && v != "" {
		ifNoneMatch = v
	} else {
		ifNoneMatch = c.Header("If-None-Match")
	}

	if ifNoneMatch != "" && etagMatches(ifNoneMatch, etag) {
		// Send 304 Not Modified through Kruda's response path
		// This properly sets c.responded = true via SendBytes
		c.Status(http.StatusNotModified)
		_ = c.SendBytes(nil)
		return true
	}

	return false
}

// GenerateAndSetETag generates an ETag from body and sets it.
// Returns true if a 304 Not Modified response was sent (handler should return nil).
// This is a convenience function combining GenerateETag and SetETag.
//
// Example:
//
//	body := []byte("response data")
//	if etag.GenerateAndSetETag(c, body) {
//	    return nil // 304 already sent
//	}
//	return c.SendBytes(body)
func GenerateAndSetETag(c *kruda.Ctx, body []byte) bool {
	weak := true // Default to weak ETags
	if w, ok := c.Get("etag:weak").(bool); ok {
		weak = w
	}

	etag := GenerateETag(body, weak)
	return SetETag(c, etag)
}

// etagMatches checks if the If-None-Match header matches the generated ETag.
func etagMatches(ifNoneMatch, etag string) bool {
	// Handle "*" wildcard
	if ifNoneMatch == "*" {
		return true
	}

	// Simple comparison for most common case
	if ifNoneMatch == etag {
		return true
	}

	// Handle weak ETag comparison - weak ETags can match strong ETags with same value
	if isWeakETag(ifNoneMatch) || isWeakETag(etag) {
		return weakETagsMatch(ifNoneMatch, etag)
	}

	// Handle comma-separated ETags (simplified)
	if len(ifNoneMatch) > len(etag) {
		// Check if etag is contained in the list
		return containsETag(ifNoneMatch, etag)
	}

	return false
}

// containsETag checks if an ETag is present in a comma-separated list.
func containsETag(list, etag string) bool {
	start := 0
	for i := 0; i <= len(list); i++ {
		if i == len(list) || list[i] == ',' {
			candidate := trimSpaces(list[start:i])
			if candidate == etag ||
				(isWeakETag(candidate) || isWeakETag(etag)) && weakETagsMatch(candidate, etag) {
				return true
			}
			start = i + 1
		}
	}
	return false
}

// trimSpaces removes leading and trailing spaces.
func trimSpaces(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}

// isWeakETag checks if an ETag is weak (starts with W/).
func isWeakETag(etag string) bool {
	return len(etag) > 2 && etag[0] == 'W' && etag[1] == '/'
}

// weakETagsMatch compares weak ETags by removing W/ prefix.
func weakETagsMatch(etag1, etag2 string) bool {
	if isWeakETag(etag1) {
		etag1 = etag1[2:]
	}
	if isWeakETag(etag2) {
		etag2 = etag2[2:]
	}
	return etag1 == etag2
}
