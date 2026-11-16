package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateHandlerCLI(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	coreMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if os.IsNotExist(err) || (err == nil && !bytes.HasPrefix(coreMod, []byte("module github.com/go-kruda/kruda\n"))) {
		t.Skip("generator integration requires the parent Kruda core source checkout")
	}
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "kruda")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join(root, "cmd", "kruda")
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	mod := []byte("module handler.test\n\ngo 1.26.11\n\nrequire github.com/go-kruda/kruda v0.0.0\n\nreplace github.com/go-kruda/kruda => " + strconv.Quote(filepath.ToSlash(root)) + "\n")
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"go.mod": mod, "go.sum": sum} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"generate", "handler", "--path=/users"},
		{"generate", "handler", "--path=/products", "--typed"},
	} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate handler %v: %v\n%s", args, err, out)
		}
	}
	src := []byte(`package consumer

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
	"handler.test/handlers"
)

func TestGeneratedHandlers(t *testing.T) {
	app := kruda.New()
	app.Get("/users", handlers.UsersHandler)
	kruda.Get[handlers.ProductsRequest, handlers.ProductsResponse](app, "/products", handlers.ProductsHandler)
	for _, tc := range []struct{ path, message string }{
		{"/users", "Users handler"},
		{"/products", "Products handler"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			contentType, _, err := mime.ParseMediaType(rec.Header().Get("Content-Type"))
			if err != nil || contentType != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json: %v", rec.Header().Get("Content-Type"), err)
			}
			var response struct{ Message string }
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.Message != tc.message {
				t.Fatalf("message = %q, want %q", response.Message, tc.message)
			}
		})
	}
}
`)
	if err := os.WriteFile(filepath.Join(dir, "handlers_test.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	test := exec.Command("go", "test", "-mod=mod", "-tags", "kruda_stdjson", "-v", "./...")
	test.Dir = dir
	test.Env = append(os.Environ(), "GOWORK=off")
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("test generated handlers: %v\n%s", err, out)
	} else {
		t.Logf("generated consumer tests:\n%s", out)
	}
}

func TestHandlerNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/users", "Users"},
		{"/products/:id", "Products"},
		{"/api/v1/orders", "Orders"},
		{"/api", "Api"},
		{"/:id", ""},   // only params → empty
		{"/*path", ""}, // only wildcard → empty
		{"/", ""},      // root only → empty
		{"/users/:id", "Users"},
		{"/api/v2/items/:itemID", "Items"},
	}
	for _, tt := range tests {
		got := handlerNameFromPath(tt.path)
		if got != tt.want {
			t.Errorf("handlerNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestGenerateHandler_BasicTemplate(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := handlerCmd
	cmd.Flags().Set("path", "/users")
	cmd.Flags().Set("typed", "false")
	cmd.Flags().Set("force", "true")

	if err := runGenerateHandler(cmd, nil); err != nil {
		t.Fatalf("runGenerateHandler: %v", err)
	}

	outPath := filepath.Join(dir, "handlers", "users.go")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "package handlers") {
		t.Error("generated file should contain 'package handlers'")
	}
	if !strings.Contains(content, "func UsersHandler(c *kruda.Ctx) error") {
		t.Error("generated file should contain basic handler signature")
	}
	if !strings.Contains(content, `"github.com/go-kruda/kruda"`) {
		t.Error("generated file should import kruda")
	}
}

func TestGenerateHandler_TypedTemplate(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := handlerCmd
	cmd.Flags().Set("path", "/products")
	cmd.Flags().Set("typed", "true")
	cmd.Flags().Set("force", "true")

	if err := runGenerateHandler(cmd, nil); err != nil {
		t.Fatalf("runGenerateHandler: %v", err)
	}

	outPath := filepath.Join(dir, "handlers", "products.go")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "ProductsRequest") {
		t.Error("typed handler should contain request struct")
	}
	if !strings.Contains(content, "ProductsResponse") {
		t.Error("typed handler should contain response struct")
	}
	if !strings.Contains(content, "func ProductsHandler(c *kruda.C[ProductsRequest]) (*ProductsResponse, error)") {
		t.Error("typed handler should use C[T] pattern")
	}
}

func TestGenerateHandler_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Create the file first
	handlersDir := filepath.Join(dir, "handlers")
	os.MkdirAll(handlersDir, 0o755)
	os.WriteFile(filepath.Join(handlersDir, "users.go"), []byte("existing"), 0o644)

	cmd := handlerCmd
	cmd.Flags().Set("path", "/users")
	cmd.Flags().Set("typed", "false")
	cmd.Flags().Set("force", "false")

	err := runGenerateHandler(cmd, nil)
	if err == nil {
		t.Fatal("expected error when file exists and --force is false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}

func TestGenerateHandler_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// Create the file first
	handlersDir := filepath.Join(dir, "handlers")
	os.MkdirAll(handlersDir, 0o755)
	os.WriteFile(filepath.Join(handlersDir, "users.go"), []byte("existing"), 0o644)

	cmd := handlerCmd
	cmd.Flags().Set("path", "/users")
	cmd.Flags().Set("typed", "false")
	cmd.Flags().Set("force", "true")

	if err := runGenerateHandler(cmd, nil); err != nil {
		t.Fatalf("runGenerateHandler with --force: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(handlersDir, "users.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "existing" {
		t.Error("file should be overwritten with --force")
	}
}

func TestGenerateResource_Template(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := resourceCmd
	cmd.Flags().Set("name", "Product")
	cmd.Flags().Set("force", "true")

	if err := runGenerateResource(cmd, nil); err != nil {
		t.Fatalf("runGenerateResource: %v", err)
	}

	outPath := filepath.Join(dir, "services", "product_service.go")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "package services") {
		t.Error("resource should be in services package")
	}
	if !strings.Contains(content, "type Product struct") {
		t.Error("should contain Product struct")
	}
	if !strings.Contains(content, "type ProductService struct") {
		t.Error("should contain ProductService struct")
	}
	if !strings.Contains(content, "NewProductService()") {
		t.Error("should contain constructor")
	}
	for _, method := range []string{"List", "Get", "Create", "Update", "Delete"} {
		if !strings.Contains(content, "func (s *ProductService) "+method) {
			t.Errorf("should contain %s method", method)
		}
	}
}

func TestGenerateResource_NameCapitalization(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := resourceCmd
	cmd.Flags().Set("name", "product") // lowercase
	cmd.Flags().Set("force", "true")

	if err := runGenerateResource(cmd, nil); err != nil {
		t.Fatalf("runGenerateResource: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "services", "product_service.go"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Should be capitalized in type names
	if !strings.Contains(content, "type Product struct") {
		t.Error("'product' should be capitalized to 'Product' in type name")
	}
}
