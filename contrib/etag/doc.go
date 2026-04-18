// Package etag provides ETag and conditional GET/HEAD support for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/etag"
//
//	app := kruda.New()
//	app.Use(etag.New())
//
//	app.Get("/data", func(c *kruda.Ctx) error {
//	    body, _ := json.Marshal(fetchData())
//	    if etag.GenerateAndSetETag(c, body) {
//	        return nil // 304 Not Modified already sent
//	    }
//	    return c.SendBytes(body)
//	})
//
// # What it does
//
// The middleware runs only for GET and HEAD requests. It parses the
// If-None-Match request header and stashes it (plus the Weak setting) in
// the request context for handlers to consume. Handlers compute an ETag
// from the response body using one of the helpers:
//
//   - [GenerateETag]          — returns an ETag string (CRC32 + size)
//   - [SetETag]               — sets the header and short-circuits with 304 on match
//   - [GenerateAndSetETag]    — convenience: generate, set, and check
//
// When the client's If-None-Match matches, the helper writes a 304 response
// through Kruda's response path and returns true so the handler can return
// nil immediately.
//
// # Configuration
//
//   - Weak: emit weak ETags (W/"...") — default true; weak ETags are faster
//     and sufficient for most use cases
//   - Skip: per-request bypass function
//
// # See also
//
//   - RFC 7232 §2.3 (ETag) and §3.2 (If-None-Match)
package etag
