package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// templateNames are the templates `kruda new -t` accepts.
var templateNames = []string{"minimal", "api", "fullstack"}

// TestTemplatesDoNotPinCoreVersion guards the scaffolder's central promise: the
// project it writes has to build.
//
// The templates used to carry `require github.com/go-kruda/kruda v0.0.0`, a
// placeholder that no proxy can resolve, so `go mod tidy` — step two of the
// instructions the CLI itself prints — failed for every template, and `go get
// ...@latest` failed too because Go resolves the bad requirement first. It
// shipped that way for five months because the tests below only checked that
// files existed.
//
// Leaving the requirement out entirely is what keeps this fixed: `go mod tidy`
// resolves the current release on its own, so there is no pinned version here
// to go stale at the next tag.
func TestTemplatesDoNotPinCoreVersion(t *testing.T) {
	for _, name := range templateNames {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			data := templateData{ProjectName: "probe", ModuleName: "probe"}
			if err := scaffoldFromFS(templateFS, "templates/"+name, dir, data); err != nil {
				t.Fatalf("scaffoldFromFS: %v", err)
			}
			b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err != nil {
				t.Fatalf("read go.mod: %v", err)
			}
			if strings.Contains(string(b), "github.com/go-kruda/kruda") {
				t.Errorf("template pins the core module; let `go mod tidy` resolve it instead:\n%s", b)
			}
		})
	}
}

// TestScaffoldedProjectBuilds runs what a new user runs. It needs the module
// proxy, so it is skipped under -short.
func TestScaffoldedProjectBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("needs the module proxy")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	for _, name := range templateNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			data := templateData{ProjectName: "probe", ModuleName: "probe"}
			if err := scaffoldFromFS(templateFS, "templates/"+name, dir, data); err != nil {
				t.Fatalf("scaffoldFromFS: %v", err)
			}
			for _, args := range [][]string{{"mod", "tidy"}, {"build", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				// A go.work above the temp dir would pull in this repo and mask
				// what a user outside it actually gets.
				cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %v in a fresh %s project failed: %v\n%s", args, name, err, out)
				}
			}
		})
	}
}
