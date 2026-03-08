package kruda

import (
	"bytes"
	"io"
	"testing"
)

type mockViewEngine struct {
	rendered string
	data     any
	err      error
}

func (m *mockViewEngine) Render(w io.Writer, name string, data any) error {
	if m.err != nil {
		return m.err
	}
	m.rendered = name
	m.data = data
	w.Write([]byte("<h1>Hello</h1>"))
	return nil
}

func TestCtx_Render(t *testing.T) {
	engine := &mockViewEngine{}
	app := New(WithViews(engine))
	w := newMockResponse()
	c := newCtx(app)
	c.reset(w, &mockRequest{method: "GET", path: "/"})

	err := c.Render("index", Map{"title": "Home"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.rendered != "index" {
		t.Errorf("rendered = %q, want %q", engine.rendered, "index")
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if !bytes.Contains(w.body, []byte("<h1>Hello</h1>")) {
		t.Errorf("body = %q, want to contain <h1>Hello</h1>", w.body)
	}
}

func TestCtx_Render_WithStatus(t *testing.T) {
	engine := &mockViewEngine{}
	app := New(WithViews(engine))
	w := newMockResponse()
	c := newCtx(app)
	c.reset(w, &mockRequest{method: "GET", path: "/"})

	err := c.Render("error", nil, 404)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.statusCode != 404 {
		t.Errorf("status = %d, want 404", w.statusCode)
	}
}

func TestCtx_Render_NoEngine(t *testing.T) {
	app := New() // no WithViews
	w := newMockResponse()
	c := newCtx(app)
	c.reset(w, &mockRequest{method: "GET", path: "/"})

	err := c.Render("index", nil)
	if err == nil {
		t.Fatal("expected error when no view engine configured")
	}
}

func TestCtx_Render_EngineError(t *testing.T) {
	engine := &mockViewEngine{err: io.ErrUnexpectedEOF}
	app := New(WithViews(engine))
	w := newMockResponse()
	c := newCtx(app)
	c.reset(w, &mockRequest{method: "GET", path: "/"})

	err := c.Render("broken", nil)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("error = %v, want io.ErrUnexpectedEOF", err)
	}
}
