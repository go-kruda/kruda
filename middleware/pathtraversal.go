package middleware

import (
	"net/url"
	"path"
	"strings"

	"github.com/go-kruda/kruda"
)

// PathTraversal returns middleware that prevents path traversal attacks.
// It decodes percent-encoded sequences, then checks for ".." segments
// that would escape above the root directory. Such requests are rejected
// with a 400 Bad Request error.
//
// Usage:
//
//	app.Use(middleware.PathTraversal())
func PathTraversal() kruda.HandlerFunc {
	return func(c *kruda.Ctx) error {
		path := c.Path()
		// Quick check: if no dots or percent signs, no traversal possible
		if !containsDotOrPercent(path) {
			return c.Next()
		}
		if isPathTraversal(path) {
			return kruda.NewError(400, "bad request: path traversal detected")
		}
		return c.Next()
	}
}

// containsDotOrPercent is a fast byte scan for . or % characters.
func containsDotOrPercent(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '%' || s[i] == '\\' || s[i] == 0 {
			return true
		}
	}
	return false
}

// isPathTraversal decodes percent-encoding and checks if the path
// contains ".." segments that escape above the root directory.
func isPathTraversal(raw string) bool {
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return true
	}
	// Normalize backslashes to forward slashes (Windows-style traversal)
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	// Check for ".." in any segment before path.Clean normalizes it away
	for _, seg := range strings.Split(decoded, "/") {
		if seg == ".." {
			return true
		}
	}
	cleaned := path.Clean(decoded)
	return strings.Contains(cleaned, "..")
}
