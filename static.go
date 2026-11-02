package kruda

import (
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kruda/kruda/transport"
)

// Static serves files from a directory. It panics if root cannot be opened.
//
//	app.Static("/assets", "./public")
//	app.Static("/", "./dist")  // SPA fallback
func (app *App) Static(prefix, root string, opts ...StaticOption) *App {
	cfg := staticConfig{root: root, index: "index.html", maxOpenFiles: 1024}
	for _, o := range opts {
		o(&cfg)
	}

	rootDir, err := os.OpenRoot(root)
	if err != nil {
		panic(fmt.Errorf("kruda: open static root %q: %w", root, err))
	}
	app.OnShutdown(func() { _ = rootDir.Close() })
	return app.staticHandler(prefix, rootDir.FS(), cfg)
}

// StaticFS serves files from an fs.FS (embed support).
//
//	//go:embed dist
//	var distFS embed.FS
//	app.StaticFS("/", distFS)
func (app *App) StaticFS(prefix string, fsys fs.FS, opts ...StaticOption) *App {
	cfg := staticConfig{index: "index.html", maxOpenFiles: 1024}
	for _, o := range opts {
		o(&cfg)
	}
	return app.staticHandler(prefix, fsys, cfg)
}

type staticConfig struct {
	maxOpenFiles int
	root         string
	index        string
	browse       bool
	maxAge       int
	spa          bool // serve index.html for missing files
}

// StaticOption configures static file serving.
type StaticOption func(*staticConfig)

// WithIndex sets the index file name (default: "index.html").
func WithIndex(name string) StaticOption {
	return func(c *staticConfig) { c.index = name }
}

// WithMaxOpenFiles limits open files per static route, including deferred reads
// and closes. The default is 1024. Excess requests receive HTTP 503.
func WithMaxOpenFiles(n int) StaticOption {
	if n <= 0 {
		panic("kruda: static max open files must be positive")
	}
	return func(c *staticConfig) { c.maxOpenFiles = n }
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
	openFiles := make(chan struct{}, cfg.maxOpenFiles)
	handler := func(c *Ctx) error {
		if c.responded {
			return ErrAlreadyResponded
		}
		path := c.Path()
		if prefix != "" {
			path = strings.TrimPrefix(path, prefix)
		}
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			path = cfg.index
		}

		// Security: prevent path traversal — reject before cleaning.
		for _, segment := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
			if segment == ".." {
				return c.Status(403).Text("Forbidden")
			}
		}
		path = filepath.ToSlash(filepath.Clean("/" + path))[1:]
		if path == "" {
			path = cfg.index
		}

		select {
		case openFiles <- struct{}{}:
		default:
			return c.Status(503).Text("Service Unavailable")
		}
		transferred := false
		defer func() {
			if !transferred {
				<-openFiles
			}
		}()
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
		defer func() {
			if f != nil {
				_ = f.Close()
			}
		}()

		stat, err := f.Stat()
		if err != nil {
			return c.Status(500).Text("Internal Server Error")
		}
		if stat.IsDir() {
			_ = f.Close()
			f, err = fsys.Open(pathpkg.Join(path, cfg.index))
			if err != nil {
				if cfg.browse {
					return c.Status(200).Text("Directory listing not implemented")
				}
				return c.Status(404).Text("Not Found")
			}
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
				ticket, canSendFile := c.writer.(interface{ setStaticSendfileRelease(func()) })
				if capability, ok := c.writer.(interface{ supportsStaticSendfile() bool }); ok {
					canSendFile = canSendFile && capability.supportsStaticSendfile()
				}
				if canSendFile {
					if fd, ok := duplicateStaticFile(osFile); ok {
						fs.SetSendFile(fd, stat.Size())
						ticket.setStaticSendfileRelease(func() { <-openFiles })
						transferred = true
						c.contentType = ct
						return c.send()
					}
				}
			}
		}

		c.contentType = ct
		c.writer = c.ResponseWriter()
		c.SetHeader("Content-Length", strconv.FormatInt(stat.Size(), 10))
		c.writeHeaders()
		c.writer.WriteHeader(c.status)
		c.responded = true
		if streamer, ok := c.writer.(transport.FileStreamer); ok {
			streamer.SetFileStream(&staticStreamFile{File: f, release: func() { <-openFiles }}, stat.Size())
			transferred = true
			f = nil // the transport closes it after delivery
			return nil
		}
		_, err = io.CopyN(writerAdapter{c.writer}, f, stat.Size())
		return err
	}

	app.Get(prefix+"/*filepath", handler)
	if prefix != "" {
		app.Get(prefix, handler)
	} else {
		app.Get("/", handler)
	}
	return app
}

type staticStreamFile struct {
	fs.File
	release func()
	once    sync.Once
	err     error
}

func (f *staticStreamFile) Close() error {
	f.once.Do(func() { f.err = f.File.Close(); f.release() })
	return f.err
}
