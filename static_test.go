package kruda

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestStatic_ServesFile(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("<html>home</html>")},
		"style.css":  {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "<html>home</html>") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestStatic_Index(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("<html>root</html>")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.BodyString(), "<html>root</html>") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestStatic_NotFound(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("home")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/nope.txt")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode())
	}
}

func TestStatic_PathTraversal(t *testing.T) {
	fs := fstest.MapFS{
		"secret.txt": {Data: []byte("secret")},
	}
	app := New()
	app.StaticFS("/public", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/public/../secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() == 200 && strings.Contains(resp.BodyString(), "secret") {
		t.Error("path traversal should be blocked")
	}
}

func TestStatic_SPA(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("<html>spa</html>")},
		"style.css":  {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/", fs, WithSPA())
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/style.css")
	if !strings.Contains(resp.BodyString(), "body{}") {
		t.Errorf("existing file body = %q", resp.BodyString())
	}

	resp, _ = tc.Get("/about")
	if !strings.Contains(resp.BodyString(), "<html>spa</html>") {
		t.Errorf("SPA fallback body = %q", resp.BodyString())
	}
}

func TestStatic_ContentType(t *testing.T) {
	fs := fstest.MapFS{
		"app.js":    {Data: []byte("var x=1")},
		"style.css": {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/style.css")
	if !strings.Contains(resp.Header("Content-Type"), "css") {
		t.Errorf("CSS Content-Type = %q", resp.Header("Content-Type"))
	}

	resp, _ = tc.Get("/app.js")
	if !strings.Contains(resp.Header("Content-Type"), "javascript") {
		t.Errorf("JS Content-Type = %q", resp.Header("Content-Type"))
	}
}
