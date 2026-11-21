package etag

import (
	"fmt"
	"hash/crc32"
	"net/http"
	"strings"

	"github.com/go-kruda/kruda"
)

// Config holds ETag middleware configuration.
type Config struct {
	// Weak generates weak ETags (W/"...") when true, strong ETags otherwise.
	// New() without a Config generates weak ETags.
	Weak bool

	// Skip allows conditional bypassing of ETag generation.
	// Return true to skip ETag processing for this request.
	Skip func(*kruda.Ctx) bool
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
// Send the same bytes the ETag was generated from. Marshalling again to
// respond, with c.JSON(data), encodes the value a second time for no benefit and
// lets the ETag describe bytes other than the ones sent.
//
// Example:
//
//	app.Use(etag.New())
//	app.Get("/data", func(c *kruda.Ctx) error {
//	    data := fetchData()
//	    body, err := json.Marshal(data)
//	    if err != nil {
//	        return err
//	    }
//	    if etag.GenerateAndSetETag(c, body) {
//	        return nil // 304 sent
//	    }
//	    return c.SendBytesWithType("application/json; charset=utf-8", body)
//	})
func New(config ...Config) kruda.HandlerFunc {
	cfg := Config{Weak: true}
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *kruda.Ctx) error {
		method := c.Method()
		conditional := method == http.MethodGet || method == http.MethodHead
		if conditional && cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		c.Set("etag:weak", cfg.Weak)
		if conditional {
			c.Set("etag:if-none-match", c.Header("If-None-Match"))
		}

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

	if c.Method() != http.MethodGet && c.Method() != http.MethodHead {
		return false
	}

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
	ifNoneMatch = trimSpaces(ifNoneMatch)
	return ifNoneMatch == "*" || containsETag(ifNoneMatch, etag)
}

// containsETag compares a list of entity tags using weak comparison.
func containsETag(list, etag string) bool {
	for list = trimSpaces(list); list != ""; list = trimSpaces(list) {
		if list[0] == ',' {
			list = list[1:]
			continue
		}
		tag := strings.TrimPrefix(list, "W/")
		if len(tag) == 0 || tag[0] != '"' {
			return false
		}
		end := strings.IndexByte(tag[1:], '"')
		if end == -1 {
			return false
		}
		candidate, rest := tag[:end+2], trimSpaces(tag[end+2:])
		if rest != "" && rest[0] != ',' {
			return false
		}
		if weakETagsMatch(candidate, etag) {
			return true
		}
		list = rest
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
