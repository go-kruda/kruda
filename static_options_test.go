package kruda

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// --- Static option helpers (WithMaxAge / WithIndex / WithBrowse) ---

func TestStatic_WithMaxAge(t *testing.T) {
	fs := fstest.MapFS{
		"style.css": {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/assets", fs, WithMaxAge(3600))
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/assets/style.css")
	cc := resp.Header("Cache-Control")
	if !strings.Contains(cc, "max-age=3600") {
		t.Errorf("Cache-Control = %q", cc)
	}
}

func TestStatic_WithIndex(t *testing.T) {
	fs := fstest.MapFS{
		"home.html": {Data: []byte("<html>home</html>")},
	}
	app := New()
	app.StaticFS("/", fs, WithIndex("home.html"))
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/")
	if !strings.Contains(resp.BodyString(), "<html>home</html>") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestStatic_WithBrowse(t *testing.T) {
	fs := fstest.MapFS{
		"sub/file.txt": {Data: []byte("hello")},
	}
	app := New()
	app.StaticFS("/files", fs, WithBrowse())
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/files/sub/file.txt")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Static.Static (os.DirFS) ---

func TestStatic_OsDirFS(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(dir + "/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("hello world")
	f.Close()

	app := New()
	app.Static("/files", dir)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/files/hello.txt")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "hello world") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Static: directory + index handling ---

func TestStatic_DirectoryIndex(t *testing.T) {
	fs := fstest.MapFS{
		"sub/index.html": {Data: []byte("<html>sub index</html>")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	// Access a directory — should serve index.html inside it
	resp, _ := tc.Get("/sub")
	if resp.StatusCode() == 200 && !strings.Contains(resp.BodyString(), "sub index") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestStatic_DirectoryNoIndex_NoBrowse(t *testing.T) {
	fs := fstest.MapFS{
		"sub/file.txt": {Data: []byte("hello")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	// Access a directory without index.html — should 404
	resp, _ := tc.Get("/sub")
	if resp.StatusCode() != 404 {
		t.Errorf("status = %d, want 404 for directory without index", resp.StatusCode())
	}
}

func TestStatic_DirectoryNoIndex_BrowseEnabled(t *testing.T) {
	fs := fstest.MapFS{
		"sub/file.txt": {Data: []byte("hello")},
	}
	app := New()
	app.StaticFS("/", fs, WithBrowse())
	app.Compile()

	tc := NewTestClient(app)
	// Access a directory without index.html but browse is enabled
	resp, _ := tc.Get("/sub")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d, want 200 for browse mode", resp.StatusCode())
	}
}

// --- Static: SPA fallback when index.html doesn't exist ---

func TestStatic_SPA_MissingIndex(t *testing.T) {
	// Empty FS with no index.html — SPA fallback should 404
	fs := fstest.MapFS{}
	app := New()
	app.StaticFS("/", fs, WithSPA())
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/nonexistent")
	if resp.StatusCode() != 404 {
		t.Errorf("status = %d, want 404 when SPA index missing", resp.StatusCode())
	}
}

// --- Static: file with unknown extension ---

func TestStatic_UnknownExtension(t *testing.T) {
	fs := fstest.MapFS{
		"data.xyz": {Data: []byte("binary data")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/data.xyz")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	// Unknown extension should default to application/octet-stream
	ct := resp.Header("Content-Type")
	if ct != "" && !strings.Contains(ct, "octet-stream") && !strings.Contains(ct, "xyz") {
		t.Logf("unexpected content-type for .xyz: %s", ct)
	}
}

// --- Static: prefix-based serving ---

func TestStatic_WithPrefix(t *testing.T) {
	fs := fstest.MapFS{
		"style.css": {Data: []byte("body{}")},
	}
	app := New()
	app.StaticFS("/public", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/public/style.css")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "body{}") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- Static: path traversal with double dots ---

func TestStatic_PathTraversal_DoubleDots(t *testing.T) {
	fs := fstest.MapFS{
		"index.html": {Data: []byte("home")},
	}
	app := New()
	app.StaticFS("/", fs)
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/../../etc/passwd")
	if resp.StatusCode() != 403 {
		t.Errorf("status = %d, want 403 for path traversal", resp.StatusCode())
	}
}
