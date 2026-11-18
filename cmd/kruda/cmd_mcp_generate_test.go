package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPAddHandlerKeepsAnEditedFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := toolAddHandler(map[string]any{"path": "/users"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("handlers", "users.go")
	if err := os.WriteFile(path, []byte("// my edits\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := toolAddHandler(map[string]any{"path": "/users"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second call err = %v, want already exists", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "// my edits\n" {
		t.Fatalf("edited file was replaced: %q", b)
	}
	if _, err := toolAddHandler(map[string]any{"path": "/users", "force": true}); err != nil {
		t.Fatalf("force: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) == "// my edits\n" {
		t.Fatal("force did not overwrite")
	}
}

func TestMCPAddResourceKeepsAnEditedFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := toolAddResource(map[string]any{"name": "Product"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("services", "product_service.go")
	if err := os.WriteFile(path, []byte("// my edits\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := toolAddResource(map[string]any{"name": "Product"}); err == nil {
		t.Fatal("second call replaced the file")
	}
	if b, _ := os.ReadFile(path); string(b) != "// my edits\n" {
		t.Fatalf("edited file was replaced: %q", b)
	}
}

func TestGeneratorRejectsNamesOutsideItsDirectory(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	for _, name := range []string{"../../evil", "a/b", "my-thing", "9lives", ""} {
		if _, err := toolAddResource(map[string]any{"name": name}); err == nil {
			t.Errorf("name %q was accepted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "evil_service.go")); err == nil {
		t.Fatal("a file was written outside services/")
	}
	for _, p := range []string{"/users/..", "/user-profiles", "/v1/9lives"} {
		if _, err := toolAddHandler(map[string]any{"path": p}); err == nil {
			t.Errorf("path %q was accepted", p)
		}
	}
}
