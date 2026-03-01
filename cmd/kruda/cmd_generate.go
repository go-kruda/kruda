package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

// generateCmd is the parent command for code generation subcommands.
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate boilerplate code",
	Long: `Generate handler or resource boilerplate code for a Kruda project.

Subcommands:
  handler    Generate a route handler file
  resource   Generate a ResourceService implementation`,
	Aliases: []string{"gen", "g"},
}

// handlerCmd is the subcommand for generating a route handler file.
var handlerCmd = &cobra.Command{
	Use:   "handler",
	Short: "Generate a route handler",
	Long: `Generate a handler Go file with route registration and handler function skeleton.

Examples:
  kruda generate handler --path=/users
  kruda generate handler --path=/users --typed
  kruda generate handler --path=/products/:id --typed --force`,
	RunE: runGenerateHandler,
}

// resourceCmd is the subcommand for generating a ResourceService implementation.
var resourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "Generate a ResourceService implementation",
	Long: `Generate a ResourceService[T, ID] implementation with all CRUD methods.

Examples:
  kruda generate resource --name=User
  kruda generate resource --name=Product --force`,
	RunE: runGenerateResource,
}

func init() {
	handlerCmd.Flags().String("path", "", "route path (e.g. /users, /products/:id)")
	handlerCmd.Flags().Bool("typed", false, "generate typed handler with C[T] pattern")
	handlerCmd.Flags().Bool("force", false, "overwrite existing file without prompting")
	handlerCmd.MarkFlagRequired("path")

	resourceCmd.Flags().String("name", "", "resource name (e.g. User, Product)")
	resourceCmd.Flags().Bool("force", false, "overwrite existing file without prompting")
	resourceCmd.MarkFlagRequired("name")

	generateCmd.AddCommand(handlerCmd)
	generateCmd.AddCommand(resourceCmd)
}

// handlerTemplate is the text/template for generating a basic handler file.
const handlerTemplate = `package handlers

import (
	"github.com/go-kruda/kruda"
)

// {{.FuncName}}Handler handles {{.Method}} {{.Path}} requests.
func {{.FuncName}}Handler(c *kruda.Ctx) error {
	return c.JSON(200, map[string]string{
		"message": "{{.FuncName}} handler",
	})
}
`

// typedHandlerTemplate is the text/template for generating a typed handler file.
const typedHandlerTemplate = `package handlers

import (
	"github.com/go-kruda/kruda"
)

// {{.FuncName}}Request defines the input for {{.Method}} {{.Path}}.
type {{.FuncName}}Request struct {
	// Add your request fields here with struct tags:
	// Name string ` + "`" + `json:"name" validate:"required"` + "`" + `
}

// {{.FuncName}}Response defines the output for {{.Method}} {{.Path}}.
type {{.FuncName}}Response struct {
	Message string ` + "`" + `json:"message"` + "`" + `
}

// {{.FuncName}}Handler handles {{.Method}} {{.Path}} requests with typed input.
func {{.FuncName}}Handler(c *kruda.C[{{.FuncName}}Request]) error {
	req := c.Body()
	_ = req // use req fields

	return c.JSON(200, {{.FuncName}}Response{
		Message: "{{.FuncName}} handler",
	})
}
`

// resourceTemplate is the text/template for generating a ResourceService implementation.
const resourceTemplate = `package services

import (
	"context"
	"fmt"
	"sync"
)

// {{.Name}} represents a {{.LowerName}} entity.
type {{.Name}} struct {
	ID   string ` + "`" + `json:"id"` + "`" + `
	Name string ` + "`" + `json:"name"` + "`" + `
	// Add more fields as needed.
}

// {{.Name}}Service implements kruda.ResourceService[{{.Name}}, string] with
// in-memory storage. Replace with your database implementation.
type {{.Name}}Service struct {
	mu    sync.RWMutex
	store map[string]{{.Name}}
}

// New{{.Name}}Service creates a new {{.Name}}Service.
func New{{.Name}}Service() *{{.Name}}Service {
	return &{{.Name}}Service{
		store: make(map[string]{{.Name}}),
	}
}

// List returns all {{.LowerName}} entities.
func (s *{{.Name}}Service) List(ctx context.Context) ([]{{.Name}}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]{{.Name}}, 0, len(s.store))
	for _, v := range s.store {
		items = append(items, v)
	}
	return items, nil
}

// Get returns a single {{.LowerName}} by ID.
func (s *{{.Name}}Service) Get(ctx context.Context, id string) ({{.Name}}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.store[id]
	if !ok {
		return {{.Name}}{}, fmt.Errorf("{{.LowerName}} %q not found", id)
	}
	return item, nil
}

// Create adds a new {{.LowerName}}.
func (s *{{.Name}}Service) Create(ctx context.Context, item {{.Name}}) ({{.Name}}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.ID == "" {
		return {{.Name}}{}, fmt.Errorf("{{.LowerName}} ID is required")
	}
	if _, exists := s.store[item.ID]; exists {
		return {{.Name}}{}, fmt.Errorf("{{.LowerName}} %q already exists", item.ID)
	}
	s.store[item.ID] = item
	return item, nil
}

// Update modifies an existing {{.LowerName}}.
func (s *{{.Name}}Service) Update(ctx context.Context, id string, item {{.Name}}) ({{.Name}}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.store[id]; !ok {
		return {{.Name}}{}, fmt.Errorf("{{.LowerName}} %q not found", id)
	}
	item.ID = id
	s.store[id] = item
	return item, nil
}

// Delete removes a {{.LowerName}} by ID.
func (s *{{.Name}}Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.store[id]; !ok {
		return fmt.Errorf("{{.LowerName}} %q not found", id)
	}
	delete(s.store, id)
	return nil
}
`

// handlerData holds template data for handler generation.
type handlerData struct {
	FuncName string
	Path     string
	Method   string
}

// resourceData holds template data for resource generation.
type resourceData struct {
	Name      string
	LowerName string
}

// runGenerateHandler generates a handler file from the route path.
func runGenerateHandler(cmd *cobra.Command, args []string) error {
	routePath, _ := cmd.Flags().GetString("path")
	typed, _ := cmd.Flags().GetBool("typed")
	force, _ := cmd.Flags().GetBool("force")

	funcName := handlerNameFromPath(routePath)
	if funcName == "" {
		return fmt.Errorf("cannot derive handler name from path %q", routePath)
	}

	fileName := strings.ToLower(funcName) + ".go"
	outPath := filepath.Join("handlers", fileName)

	if !force {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file %s already exists (use --force to overwrite)", outPath)
		}
	}

	tmplStr := handlerTemplate
	if typed {
		tmplStr = typedHandlerTemplate
	}

	tmpl, err := template.New("handler").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	data := handlerData{
		FuncName: funcName,
		Path:     routePath,
		Method:   "GET",
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	fmt.Printf("  ✅ Generated handler: %s\n", outPath)
	if typed {
		fmt.Printf("     Typed handler with C[%sRequest] pattern\n", funcName)
	}
	fmt.Printf("     Route: %s %s\n", data.Method, routePath)
	return nil
}

// runGenerateResource generates a ResourceService implementation file.
func runGenerateResource(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	force, _ := cmd.Flags().GetBool("force")

	name = strings.ToUpper(name[:1]) + name[1:]

	fileName := strings.ToLower(name) + "_service.go"
	outPath := filepath.Join("services", fileName)

	if !force {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file %s already exists (use --force to overwrite)", outPath)
		}
	}

	tmpl, err := template.New("resource").Parse(resourceTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	data := resourceData{
		Name:      name,
		LowerName: strings.ToLower(name),
	}

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	fmt.Printf("  ✅ Generated resource service: %s\n", outPath)
	fmt.Printf("     Type: %sService implements ResourceService[%s, string]\n", name, name)
	fmt.Printf("     Methods: List, Get, Create, Update, Delete\n")
	return nil
}

// handlerNameFromPath derives a PascalCase function name from a route path.
// Examples: /users → Users, /products/:id → Products, /api/v1/orders → Orders
func handlerNameFromPath(routePath string) string {
	parts := strings.Split(strings.Trim(routePath, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if seg == "" || strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") {
			continue
		}
		return strings.ToUpper(seg[:1]) + seg[1:]
	}
	return ""
}
