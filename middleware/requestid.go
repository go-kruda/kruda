package middleware

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/go-kruda/kruda"
)

// RequestIDConfig holds configuration for the RequestID middleware.
type RequestIDConfig struct {
	// Header is the HTTP header name used for the request ID.
	// Default: "X-Request-ID"
	Header string

	// Generator is a function that returns a new unique ID.
	// Default: UUID v4 via crypto/rand
	Generator func() string
}

// maxRequestIDLen is the maximum allowed length for an incoming request ID header.
// M9: prevents abuse via excessively long or malformed request IDs.
const maxRequestIDLen = 256

// RequestID returns middleware that ensures every request has a unique ID.
// If the incoming request already has an X-Request-ID header, it uses that value
// after validation (M9: length check, printable ASCII only).
// Otherwise, it generates a UUID v4 using crypto/rand.
// The request ID is stored in the context via c.Set("request_id", id) and set
// as a response header.
func RequestID(config ...RequestIDConfig) kruda.HandlerFunc {
	cfg := RequestIDConfig{
		Header:    "X-Request-ID",
		Generator: generateUUID,
	}
	if len(config) > 0 {
		if config[0].Header != "" {
			cfg.Header = config[0].Header
		}
		if config[0].Generator != nil {
			cfg.Generator = config[0].Generator
		}
	}

	return func(c *kruda.Ctx) error {
		id := c.Header(cfg.Header)
		if id == "" || !isValidRequestID(id) {
			id = cfg.Generator()
		}
		c.Set("request_id", id)
		c.SetHeader(cfg.Header, id)
		return c.Next()
	}
}

// isValidRequestID checks that a request ID is safe to use:
// - not too long
// - contains only printable ASCII (no control chars, no newlines)
func isValidRequestID(id string) bool {
	if len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

// generateUUID generates a UUID v4 string using crypto/rand.
func generateUUID() string {
	var uuid [16]byte
	_, _ = io.ReadFull(rand.Reader, uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
