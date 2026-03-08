package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantOK    bool
	}{
		{
			name:      "darwin arm64",
			input:     "go version go1.24.0 darwin/arm64",
			wantMajor: 1, wantMinor: 24, wantOK: true,
		},
		{
			name:      "linux amd64",
			input:     "go version go1.23.5 linux/amd64",
			wantMajor: 1, wantMinor: 23, wantOK: true,
		},
		{
			name:      "windows",
			input:     "go version go1.24.3 windows/amd64",
			wantMajor: 1, wantMinor: 24, wantOK: true,
		},
		{
			name:      "future version",
			input:     "go version go1.26.0 linux/amd64",
			wantMajor: 1, wantMinor: 26, wantOK: true,
		},
		{
			name:      "invalid string",
			input:     "invalid",
			wantMajor: 0, wantMinor: 0, wantOK: false,
		},
		{
			name:      "empty string",
			input:     "",
			wantMajor: 0, wantMinor: 0, wantOK: false,
		},
		{
			name:      "partial match",
			input:     "go version go",
			wantMajor: 0, wantMinor: 0, wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, ok := parseGoVersion(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseGoVersion(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if major != tt.wantMajor {
				t.Errorf("parseGoVersion(%q) major = %d, want %d", tt.input, major, tt.wantMajor)
			}
			if minor != tt.wantMinor {
				t.Errorf("parseGoVersion(%q) minor = %d, want %d", tt.input, minor, tt.wantMinor)
			}
		})
	}
}

func TestHandlerNameFromPath_Validate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/users", "Users"},
		{"/products/:id", "Products"},
		{"/api/v1/orders", "Orders"},
		{"", ""},
		{"/", ""},
		{"/:id", ""},
		{"/users/:id/posts", "Posts"},
		{"/api/v1/*path", "V1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := handlerNameFromPath(tt.input)
			if got != tt.want {
				t.Errorf("handlerNameFromPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsNonEmptyDirWithFile(t *testing.T) {
	dir := t.TempDir()

	// Empty dir should return false.
	if isNonEmptyDir(dir) {
		t.Error("expected false for empty dir")
	}

	// Add a file — should return true.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNonEmptyDir(dir) {
		t.Error("expected true for non-empty dir")
	}
}

func TestIsNonEmptyDirNonExistentPath(t *testing.T) {
	if isNonEmptyDir(filepath.Join(t.TempDir(), "nope")) {
		t.Error("expected false for non-existent path")
	}
}

func TestCheckGoModExists(t *testing.T) {
	// Run checkGoMod from a temp dir without go.mod — should fail.
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	result := checkGoMod()
	if result.passed {
		t.Error("expected checkGoMod to fail in dir without go.mod")
	}

	// Create go.mod — should pass.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result = checkGoMod()
	if !result.passed {
		t.Errorf("expected checkGoMod to pass, got: %s", result.message)
	}
}

func TestCheckKrudaDependency(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// go.mod without kruda dependency.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkKrudaDependency()
	if result.passed {
		t.Error("expected checkKrudaDependency to fail without kruda in go.mod")
	}

	// go.mod with kruda dependency.
	content := "module test\n\ngo 1.25\n\nrequire github.com/go-kruda/kruda v0.1.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result = checkKrudaDependency()
	if !result.passed {
		t.Errorf("expected checkKrudaDependency to pass, got: %s", result.message)
	}
}

func TestCountFailed(t *testing.T) {
	results := []validationResult{
		{name: "A", passed: true},
		{name: "B", passed: false},
		{name: "C", passed: false},
		{name: "D", passed: true},
	}

	got := countFailed(results)
	if got != 2 {
		t.Errorf("countFailed = %d, want 2", got)
	}
}
