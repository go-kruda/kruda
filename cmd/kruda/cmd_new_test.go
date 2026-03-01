package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMinimalTemplate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "myapp")

	data := templateData{
		ProjectName: "myapp",
		ModuleName:  "myapp",
	}

	if err := scaffoldFromFS(templateFS, "templates/minimal", target, data); err != nil {
		t.Fatalf("scaffoldFromFS minimal: %v", err)
	}

	// Verify expected files exist.
	for _, name := range []string{"main.go", "go.mod", ".gitignore"} {
		path := filepath.Join(target, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", name)
		}
	}
}

func TestNewAPITemplate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "myapi")

	data := templateData{
		ProjectName: "myapi",
		ModuleName:  "myapi",
	}

	if err := scaffoldFromFS(templateFS, "templates/api", target, data); err != nil {
		t.Fatalf("scaffoldFromFS api: %v", err)
	}

	// Verify core files.
	for _, name := range []string{"main.go", "go.mod", ".gitignore"} {
		path := filepath.Join(target, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", name)
		}
	}

	// Verify API-specific directories.
	for _, dirName := range []string{"handlers", "models", "routes"} {
		path := filepath.Join(target, dirName)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dirName)
		} else if err == nil && !info.IsDir() {
			t.Errorf("expected %s to be a directory", dirName)
		}
	}
}

func TestNewFullstackTemplate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "myfull")

	data := templateData{
		ProjectName: "myfull",
		ModuleName:  "myfull",
	}

	if err := scaffoldFromFS(templateFS, "templates/fullstack", target, data); err != nil {
		t.Fatalf("scaffoldFromFS fullstack: %v", err)
	}

	// Verify static directory exists (fullstack-specific).
	staticDir := filepath.Join(target, "static")
	info, err := os.Stat(staticDir)
	if os.IsNotExist(err) {
		t.Error("expected static/ directory to exist")
	} else if err == nil && !info.IsDir() {
		t.Error("expected static/ to be a directory")
	}
}

func TestNewExistingDir(t *testing.T) {
	dir := t.TempDir()

	// Create a file inside to make it non-empty.
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !isNonEmptyDir(dir) {
		t.Error("expected isNonEmptyDir to return true for non-empty directory")
	}
}

func TestIsNonEmptyDirEmpty(t *testing.T) {
	dir := t.TempDir()

	if isNonEmptyDir(dir) {
		t.Error("expected isNonEmptyDir to return false for empty directory")
	}
}

func TestIsNonEmptyDirNonExistent(t *testing.T) {
	if isNonEmptyDir("/nonexistent/path/that/does/not/exist") {
		t.Error("expected isNonEmptyDir to return false for non-existent path")
	}
}
