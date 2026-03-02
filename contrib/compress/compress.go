// Package compress provides gzip and deflate compression middleware for Kruda.
//
// Usage:
//
//	app := kruda.New()
//	app.Use(compress.New())
//	
//	app.Get("/api/data", func(c *kruda.Ctx) error {
//		return compress.CompressText(c, "large response data...")
//	})
//
// The middleware parses Accept-Encoding headers and stores compression preferences.
// Use the Compress* helper functions to compress responses that meet the criteria.
package compress

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kruda/kruda"
)

// Config holds compression middleware configuration.
type Config struct {
	MinSize       int      // Minimum response size to compress (default: 1024)
	Level         int      // Compression level (default: -1 for default)
	ExcludedTypes []string // Content types to skip compression (default: image/, video/, audio/)
}

// New creates a compression middleware with optional config.
// The middleware parses Accept-Encoding headers and stores compression preferences
// in the request context for use by Compress* functions.
func New(config ...Config) kruda.HandlerFunc {
	cfg := defaultConfig()
	if len(config) > 0 {
		cfg = config[0]
		cfg.setDefaults()
	}

	return func(c *kruda.Ctx) error {
		// Skip if no Accept-Encoding header
		acceptEncoding := c.Header("Accept-Encoding")
		if acceptEncoding == "" {
			return c.Next()
		}

		// Skip if response already has Content-Encoding
		if c.ResponseWriter().Header().Get("Content-Encoding") != "" {
			return c.Next()
		}

		// Parse Accept-Encoding and select best supported encoding
		encoding := selectEncoding(acceptEncoding)
		if encoding == "" {
			return c.Next()
		}

		// Store compression settings for Compress* functions
		c.Set("_compress_encoding", encoding)
		c.Set("_compress_config", cfg)

		return c.Next()
	}
}

// defaultConfig returns the default configuration.
func defaultConfig() Config {
	return Config{
		MinSize:       1024,
		Level:         -1, // Use default compression level
		ExcludedTypes: []string{"image/", "video/", "audio/"},
	}
}

// setDefaults fills in zero values with defaults.
func (c *Config) setDefaults() {
	if c.MinSize == 0 {
		c.MinSize = 1024
	}
	if c.Level == 0 {
		c.Level = -1
	}
	if c.ExcludedTypes == nil {
		c.ExcludedTypes = []string{"image/", "video/", "audio/"}
	}
}

// Compress compresses a response if compression is requested and criteria are met.
// This is the main function - use CompressText/CompressHTML for convenience.
func Compress(c *kruda.Ctx, data []byte, contentType string) error {
	encoding, ok := c.Get("_compress_encoding").(string)
	if !ok || encoding == "" {
		// No compression requested, send as-is
		c.SetHeader("Content-Type", contentType)
		return c.SendBytes(data)
	}

	cfg, ok := c.Get("_compress_config").(Config)
	if !ok {
		cfg = defaultConfig()
	}

	// Check if response should be compressed
	if !shouldCompress(data, contentType, cfg) {
		c.SetHeader("Content-Type", contentType)
		return c.SendBytes(data)
	}

	// Compress the data
	compressed, err := compressData(data, encoding, cfg.Level)
	if err != nil {
		return err
	}

	// Set compression headers and send response
	c.SetHeader("Content-Type", contentType)
	c.SetHeader("Content-Encoding", encoding)
	c.SetHeader("Vary", "Accept-Encoding")
	return c.SendBytes(compressed)
}

// CompressText compresses a text response.
func CompressText(c *kruda.Ctx, text string) error {
	return Compress(c, []byte(text), "text/plain; charset=utf-8")
}

// CompressHTML compresses an HTML response.
func CompressHTML(c *kruda.Ctx, html string) error {
	return Compress(c, []byte(html), "text/html; charset=utf-8")
}

// shouldCompress checks if a response should be compressed based on size and content type.
func shouldCompress(data []byte, contentType string, cfg Config) bool {
	// Check minimum size threshold
	if len(data) < cfg.MinSize {
		return false
	}

	// Check excluded content types
	for _, excluded := range cfg.ExcludedTypes {
		if strings.HasPrefix(contentType, excluded) {
			return false
		}
	}

	return true
}

// poolManager provides level-keyed sync.Pools for gzip and deflate writers.
// Each compression level gets its own pool to avoid re-creating writers
// when custom levels are used.
var (
	gzipPools  sync.Map // map[int]*sync.Pool
	flatePools sync.Map // map[int]*sync.Pool
)

// getGzipPool returns a sync.Pool for the given compression level.
func getGzipPool(level int) *sync.Pool {
	if v, ok := gzipPools.Load(level); ok {
		return v.(*sync.Pool)
	}
	pool := &sync.Pool{
		New: func() interface{} {
			w, _ := gzip.NewWriterLevel(nil, level)
			return w
		},
	}
	actual, _ := gzipPools.LoadOrStore(level, pool)
	return actual.(*sync.Pool)
}

// getFlatePool returns a sync.Pool for the given compression level.
func getFlatePool(level int) *sync.Pool {
	if v, ok := flatePools.Load(level); ok {
		return v.(*sync.Pool)
	}
	pool := &sync.Pool{
		New: func() interface{} {
			w, _ := flate.NewWriter(nil, level)
			return w
		},
	}
	actual, _ := flatePools.LoadOrStore(level, pool)
	return actual.(*sync.Pool)
}

// compressData compresses data using the specified encoding and level.
func compressData(data []byte, encoding string, level int) ([]byte, error) {
	var buf bytes.Buffer

	// Normalize: -1 means default compression
	if level == -1 {
		level = flate.DefaultCompression
	}

	switch encoding {
	case "gzip":
		return compressGzip(&buf, data, level)
	case "deflate":
		return compressDeflate(&buf, data, level)
	default:
		return data, nil
	}
}

// compressGzip compresses data using gzip with level-keyed pooling.
func compressGzip(buf *bytes.Buffer, data []byte, level int) ([]byte, error) {
	pool := getGzipPool(level)

	v := pool.Get()
	w, ok := v.(*gzip.Writer)
	if !ok {
		// Defensive: should never happen since Pool.New guarantees the type,
		// but guard against corrupted pool entries.
		var err error
		w, err = gzip.NewWriterLevel(buf, level)
		if err != nil {
			return nil, err
		}
	} else {
		w.Reset(buf)
	}
	defer pool.Put(w)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// compressDeflate compresses data using deflate with level-keyed pooling.
func compressDeflate(buf *bytes.Buffer, data []byte, level int) ([]byte, error) {
	pool := getFlatePool(level)

	v := pool.Get()
	w, ok := v.(*flate.Writer)
	if !ok {
		var err error
		w, err = flate.NewWriter(buf, level)
		if err != nil {
			return nil, err
		}
	} else {
		w.Reset(buf)
	}
	defer pool.Put(w)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// selectEncoding parses Accept-Encoding header and returns the best supported encoding.
// Prefers gzip over deflate when quality values are equal.
func selectEncoding(acceptEncoding string) string {
	encodings := parseAcceptEncoding(acceptEncoding)

	bestGzip := 0.0
	bestDeflate := 0.0

	for _, enc := range encodings {
		switch enc.name {
		case "gzip":
			if enc.quality > bestGzip {
				bestGzip = enc.quality
			}
		case "deflate":
			if enc.quality > bestDeflate {
				bestDeflate = enc.quality
			}
		}
	}

	// Prefer gzip when quality values are equal
	if bestGzip > bestDeflate && bestGzip > 0 {
		return "gzip"
	}
	if bestDeflate > 0 {
		return "deflate"
	}

	return ""
}

// encoding represents a parsed Accept-Encoding entry.
type encoding struct {
	name    string
	quality float64
}

// parseAcceptEncoding parses Accept-Encoding header with quality values.
// Example: "gzip, deflate;q=0.8, br;q=0.9" -> [{gzip 1.0}, {deflate 0.8}, {br 0.9}]
func parseAcceptEncoding(header string) []encoding {
	var encodings []encoding
	parts := strings.Split(header, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		enc := encoding{quality: 1.0} // Default quality

		// Check for quality value (q=0.8)
		if idx := strings.Index(part, ";"); idx >= 0 {
			enc.name = strings.TrimSpace(part[:idx])
			qpart := strings.TrimSpace(part[idx+1:])
			if strings.HasPrefix(qpart, "q=") {
				if q, err := strconv.ParseFloat(qpart[2:], 64); err == nil {
					enc.quality = q
				}
			}
		} else {
			enc.name = part
		}

		encodings = append(encodings, enc)
	}

	return encodings
}