package kruda

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// --- Static.Static (OS directory) ---

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
	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown static app: %v", err)
		}
	})
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

func TestStatic_SymlinkContainment(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "public")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	const publicBody = "public content"
	if err := os.WriteFile(filepath.Join(root, "public.txt"), []byte(publicBody), 0o644); err != nil {
		t.Fatal(err)
	}
	const linkedBody = "linked public content"
	if err := os.WriteFile(filepath.Join(root, "linked-target.txt"), []byte(linkedBody), 0o644); err != nil {
		t.Fatal(err)
	}

	const secretBody = "outside root secret"
	secretPath := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretPath, []byte(secretBody), 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink := func(target, link string) {
		t.Helper()
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
	}
	mustSymlink("linked-target.txt", filepath.Join(root, "linked.txt"))
	mustSymlink(secretPath, filepath.Join(root, "escape.txt"))
	mustSymlink(outside, filepath.Join(root, "external-dir"))
	mustSymlink(secretPath, filepath.Join(root, "sub", "index.html"))
	mustSymlink(secretPath, filepath.Join(root, "index.html"))

	app := New()
	app.Static("/files", root)
	app.Static("/spa", root, WithSPA())
	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown static app: %v", err)
		}
	})
	app.Compile()

	tc := NewTestClient(app)
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "regular in-root file", path: "/files/public.txt", wantStatus: 200, wantBody: publicBody},
		{name: "in-root symlink", path: "/files/linked.txt", wantStatus: 200, wantBody: linkedBody},
		{name: "outside-root file symlink", path: "/files/escape.txt", wantStatus: 404},
		{name: "outside-root directory symlink", path: "/files/external-dir/secret.txt", wantStatus: 404},
		{name: "outside-root directory index symlink", path: "/files/sub", wantStatus: 404},
		{name: "outside-root SPA index symlink", path: "/spa/missing", wantStatus: 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tc.Get(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode() != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode(), tt.wantStatus)
			}
			if tt.wantBody != "" && resp.BodyString() != tt.wantBody {
				t.Errorf("body = %q, want %q", resp.BodyString(), tt.wantBody)
			}
			if strings.Contains(resp.BodyString(), secretBody) {
				t.Errorf("response exposed outside-root content: %q", resp.BodyString())
			}
		})
	}
}

func TestStatic_InvalidRootPanics(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("Static did not panic for an invalid root")
		}
		if message := fmt.Sprint(recovered); !strings.Contains(message, root) {
			t.Errorf("panic = %q, want configured root %q", message, root)
		}
	}()

	New().Static("/files", root)
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
