package kruda

import (
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"sync"
)

// ViewEngine renders named templates with data.
type ViewEngine interface {
	Render(w io.Writer, name string, data any) error
}

// GoViewEngine wraps html/template as a ViewEngine.
type GoViewEngine struct {
	tmpl *template.Template
}

// NewViewEngine creates a ViewEngine from glob patterns.
//
//	engine := kruda.NewViewEngine("views/*.html")
func NewViewEngine(patterns ...string) *GoViewEngine {
	t := template.New("")
	for _, p := range patterns {
		template.Must(t.ParseGlob(p))
	}
	return &GoViewEngine{tmpl: t}
}

// NewViewEngineFS creates a ViewEngine from an embedded filesystem.
//
//	//go:embed views
//	var viewsFS embed.FS
//	engine := kruda.NewViewEngineFS(viewsFS, "views/*.html")
func NewViewEngineFS(fsys fs.FS, patterns ...string) *GoViewEngine {
	t := template.New("")
	for _, p := range patterns {
		template.Must(t.ParseFS(fsys, p))
	}
	return &GoViewEngine{tmpl: t}
}

func (e *GoViewEngine) Render(w io.Writer, name string, data any) error {
	return e.tmpl.ExecuteTemplate(w, name, data)
}

var viewBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// WithViews sets the template engine for c.Render().
func WithViews(engine ViewEngine) Option {
	return func(app *App) { app.config.Views = engine }
}
