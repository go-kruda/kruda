package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSkipDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".git", true},
		{"vendor", true},
		{"node_modules", true},
		{".hidden", true},
		{".cache", true},
		{"src", false},
		{"handlers", false},
		{".", false}, // current directory
		{"cmd", false},
	}
	for _, tt := range tests {
		got := skipDir(tt.name)
		if got != tt.want {
			t.Errorf("skipDir(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsWatchedFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"main.go", true},
		{"handler.go", true},
		{"handler_test.go", false},
		{"readme.md", false},
		{"config.yaml", false},
		{"app_pbt_test.go", false},
		{".go", true},       // edge case: just extension
		{"build.sh", false},
	}
	for _, tt := range tests {
		got := isWatchedFile(tt.name)
		if got != tt.want {
			t.Errorf("isWatchedFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAppendUnique(t *testing.T) {
	slice := []string{"a", "b"}

	// Adding duplicate should not increase length
	result := appendUnique(slice, "b")
	if len(result) != 2 {
		t.Errorf("length after dup = %d, want 2", len(result))
	}

	// Adding new element should increase length
	result = appendUnique(result, "c")
	if len(result) != 3 {
		t.Errorf("length after new = %d, want 3", len(result))
	}
	if result[2] != "c" {
		t.Errorf("result[2] = %q, want %q", result[2], "c")
	}

	// Empty slice
	empty := appendUnique(nil, "x")
	if len(empty) != 1 || empty[0] != "x" {
		t.Errorf("append to nil = %v, want [x]", empty)
	}
}

func TestWatcher_ScanFindsGoFiles(t *testing.T) {
	dir := t.TempDir()

	// Create Go files
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "handler.go"), []byte("package main"), 0o644)

	// Create non-Go file
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# readme"), 0o644)

	// Create test file (should be excluded)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main"), 0o644)

	w := newWatcher(dir)
	times, err := w.scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(times) != 2 {
		t.Errorf("found %d files, want 2 (main.go, handler.go)", len(times))
	}

	if _, ok := times[filepath.Join(dir, "main.go")]; !ok {
		t.Error("main.go not found in scan results")
	}
	if _, ok := times[filepath.Join(dir, "handler.go")]; !ok {
		t.Error("handler.go not found in scan results")
	}
	if _, ok := times[filepath.Join(dir, "main_test.go")]; ok {
		t.Error("main_test.go should be excluded")
	}
	if _, ok := times[filepath.Join(dir, "readme.md")]; ok {
		t.Error("readme.md should be excluded")
	}
}

func TestWatcher_SkipsVendorDir(t *testing.T) {
	dir := t.TempDir()

	// Create Go file in vendor dir
	vendorDir := filepath.Join(dir, "vendor", "pkg")
	os.MkdirAll(vendorDir, 0o755)
	os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("package pkg"), 0o644)

	// Create Go file in root
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	w := newWatcher(dir)
	times, err := w.scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(times) != 1 {
		t.Errorf("found %d files, want 1 (only main.go)", len(times))
	}
}

func TestWatcher_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	hiddenDir := filepath.Join(dir, ".hidden")
	os.MkdirAll(hiddenDir, 0o755)
	os.WriteFile(filepath.Join(hiddenDir, "secret.go"), []byte("package secret"), 0o644)

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	w := newWatcher(dir)
	times, err := w.scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(times) != 1 {
		t.Errorf("found %d files, want 1", len(times))
	}
}

func TestWatcher_DetectsModification(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "main.go")
	os.WriteFile(goFile, []byte("package main"), 0o644)

	w := newWatcher(dir)
	if err := w.init(); err != nil {
		t.Fatal(err)
	}

	// No changes yet
	changed := w.detectChanges()
	if len(changed) != 0 {
		t.Errorf("expected no changes, got %v", changed)
	}

	// Modify file (change mod time)
	time.Sleep(10 * time.Millisecond) // ensure different mod time
	os.WriteFile(goFile, []byte("package main\n// modified"), 0o644)

	changed = w.detectChanges()
	if len(changed) != 1 {
		t.Errorf("expected 1 change, got %d", len(changed))
	}
	if len(changed) > 0 && changed[0] != goFile {
		t.Errorf("changed[0] = %q, want %q", changed[0], goFile)
	}
}

func TestWatcher_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)

	w := newWatcher(dir)
	if err := w.init(); err != nil {
		t.Fatal(err)
	}

	// Add new file
	newFile := filepath.Join(dir, "handler.go")
	os.WriteFile(newFile, []byte("package main"), 0o644)

	changed := w.detectChanges()
	if len(changed) != 1 {
		t.Errorf("expected 1 change (new file), got %d", len(changed))
	}
}

func TestWatcher_DetectsDeletedFile(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "main.go")
	os.WriteFile(goFile, []byte("package main"), 0o644)

	w := newWatcher(dir)
	if err := w.init(); err != nil {
		t.Fatal(err)
	}

	// Delete the file
	os.Remove(goFile)

	changed := w.detectChanges()
	if len(changed) != 1 {
		t.Errorf("expected 1 change (deleted file), got %d", len(changed))
	}
}
