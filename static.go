package kruda

import (
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-kruda/kruda/transport"
)

// Static serves files from a directory.
//
//	app.Static("/assets", "./public")
//	app.Static("/", "./dist")  // SPA fallback
func (app *App) Static(prefix, root string, opts ...StaticOption) *App {
	cfg := staticConfig{root: root, index: "index.html"}
	for _, o := range opts {
		o(&cfg)
	}
	return app.staticHandler(prefix, os.DirFS(root), cfg)
}

// StaticFS serves files from an fs.FS (embed support).
//
//	//go:embed dist
//	var distFS embed.FS
//	app.StaticFS("/", distFS)
func (app *App) StaticFS(prefix string, fsys fs.FS, opts ...StaticOption) *App {
	cfg := staticConfig{index: "index.html"}
	for _, o := range opts {
		o(&cfg)
	}
	return app.staticHandler(prefix, fsys, cfg)
}

type staticConfig struct {
	root   string
	index  string
	browse bool
	maxAge int
	spa    bool // serve index.html for missing files
}

// StaticOption configures static file serving.
type StaticOption func(*staticConfig)

// WithIndex sets the index file name (default: "index.html").
func WithIndex(name string) StaticOption {
	return func(c *staticConfig) { c.index = name }
}

// WithBrowse enables directory listing.
func WithBrowse() StaticOption {
	return func(c *staticConfig) { c.browse = true }
}

// WithMaxAge sets Cache-Control max-age in seconds.
func WithMaxAge(seconds int) StaticOption {
	return func(c *staticConfig) { c.maxAge = seconds }
}

// WithSPA serves index.html for any path that doesn't match a file.
func WithSPA() StaticOption {
	return func(c *staticConfig) { c.spa = true }
}

func (app *App) staticHandler(prefix string, fsys fs.FS, cfg staticConfig) *App {
	prefix = strings.TrimRight(prefix, "/")
	handler := func(c *Ctx) error {
		path := c.Path()
		if prefix != "" {
			path = strings.TrimPrefix(path, prefix)
		}
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			path = cfg.index
		}

		// Security: prevent path traversal — reject before cleaning.
		if strings.Contains(path, "..") {
			return c.Status(403).Text("Forbidden")
		}
		path = filepath.ToSlash(filepath.Clean("/" + path))[1:]
		if path == "" {
			path = cfg.index
		}

		f, err := fsys.Open(path)
		if err != nil {
			if cfg.spa {
				// SPA fallback: serve index.html
				f, err = fsys.Open(cfg.index)
				if err != nil {
					return c.Status(404).Text("Not Found")
				}
			} else {
				return c.Status(404).Text("Not Found")
			}
		}
		defer func() { _ = f.Close() }()

		stat, err := f.Stat()
		if err != nil {
			return c.Status(500).Text("Internal Server Error")
		}
		if stat.IsDir() {
			_ = f.Close()
			f, err = fsys.Open(filepath.Join(path, cfg.index))
			if err != nil {
				if cfg.browse {
					return c.Status(200).Text("Directory listing not implemented")
				}
				return c.Status(404).Text("Not Found")
			}
			defer func() { _ = f.Close() }()
			stat, err = f.Stat()
			if err != nil {
				return c.Status(500).Text("Internal Server Error")
			}
		}

		// Content-Type from extension
		ext := filepath.Ext(stat.Name())
		ct := mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}

		if cfg.maxAge > 0 {
			c.SetHeader("Cache-Control", "public, max-age="+strconv.Itoa(cfg.maxAge))
		}

		// Try sendfile zero-copy path (Wing transport).
		if fs, ok := c.writer.(transport.FileSender); ok {
			if osFile, ok := f.(*os.File); ok {
				fs.SetSendFile(int32(osFile.Fd()), stat.Size())
				c.contentType = ct
				return c.send()
			}
		}

		// Fallback: read file and send.
		data := make([]byte, stat.Size())
		if _, err := io.ReadFull(f.(io.Reader), data); err != nil {
			return c.Status(500).Text("Internal Server Error")
		}
		c.contentType = ct
		return c.sendBytes(data)
	}

	app.Get(prefix+"/*filepath", handler)
	if prefix != "" {
		app.Get(prefix, handler)
	} else {
		app.Get("/", handler)
	}
	return app
}
