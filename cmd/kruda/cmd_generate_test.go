package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if !strings.Contains(content, "func ProductsHandler(c *kruda.C[ProductsRequest]) error") {
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
