package kruda

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// --- ViewEngine via fs.FS ---

func TestViewEngineFS(t *testing.T) {
	vfs := fstest.MapFS{
		"hello.html": {Data: []byte("Hello {{.Name}}!")},
	}
	engine := NewViewEngineFS(vfs, "*.html")

	var buf bytes.Buffer
	err := engine.Render(&buf, "hello.html", struct{ Name string }{"World"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "Hello World!" {
		t.Errorf("render = %q", buf.String())
	}
}

func TestCtx_RenderFS(t *testing.T) {
	vfs := fstest.MapFS{
		"greet.html": {Data: []byte("Hi {{.Name}}")},
	}
	engine := NewViewEngineFS(vfs, "*.html")
	app := New(WithViews(engine))
	app.Get("/greet", func(c *Ctx) error {
		return c.Render("greet.html", struct{ Name string }{"User"})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/greet")
	if !strings.Contains(resp.BodyString(), "Hi User") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- ViewEngine via glob (NewViewEngine) ---

func TestNewViewEngine(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(dir + "/test.html")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("Hello {{.Name}}")
	f.Close()

	engine := NewViewEngine(dir + "/*.html")
	var buf bytes.Buffer
	err = engine.Render(&buf, "test.html", struct{ Name string }{"World"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "Hello World" {
		t.Errorf("render = %q", buf.String())
	}
}

// --- ViewEngineFS with multiple patterns ---

func TestNewViewEngineFS_MultiplePatterns(t *testing.T) {
	vfs := fstest.MapFS{
		"a.html": {Data: []byte("A {{.V}}")},
		"b.txt":  {Data: []byte("B {{.V}}")},
	}
	engine := NewViewEngineFS(vfs, "*.html", "*.txt")

	var buf bytes.Buffer
	err := engine.Render(&buf, "a.html", struct{ V string }{"1"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "A 1" {
		t.Errorf("render a = %q", buf.String())
	}

	buf.Reset()
	err = engine.Render(&buf, "b.txt", struct{ V string }{"2"})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "B 2" {
		t.Errorf("render b = %q", buf.String())
	}
}
