// Package compress provides gzip and deflate response compression for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/compress"
//
//	app := kruda.New()
//	app.Use(compress.New())
//
//	app.Get("/api/data", func(c *kruda.Ctx) error {
//	    return compress.CompressText(c, "large response data...")
//	})
//
// # What it does
//
// The middleware parses the request's Accept-Encoding header and stores the
// negotiated encoding (gzip or deflate) in the request context. Handlers
// then call [Compress], [CompressText], or [CompressHTML] to actually
// compress and send the body. Pooled gzip/flate writers (keyed by
// compression level) keep allocations low on the hot path.
//
// Responses are skipped when:
//
//   - The client did not send Accept-Encoding
//   - A Content-Encoding response header is already set
//   - The body is smaller than MinSize
//   - The Content-Type matches an entry in ExcludedTypes
//
// # Configuration
//
//   - MinSize: minimum body size before compression kicks in (default 1024)
//   - Level: gzip/flate compression level (default -1 = stdlib default)
//   - ExcludedTypes: Content-Type prefixes that skip compression
//     (default "image/", "video/", "audio/")
//
// # See also
//
//   - net/http "Accept-Encoding" semantics
package compress
